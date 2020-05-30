package main

import (
	"context"
	"sync"
	"time"

	"github.com/gavin-potgieter/sensense-server/server/proto"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LevelService provides a running level service instance.
// There is only one service instance so critical section management
// is required in the remaining code. All completed levels must be
// removed or there will be a memory leak.
type LevelService struct {
	levelsLock sync.Mutex           // the lock to serialize modification of the active levels
	levels     map[uuid.UUID]*Level // the active levels (for all players in all games)
}

// NewLevelService creates a new level service
func NewLevelService() (*LevelService, error) {
	return &LevelService{
		levels:     make(map[uuid.UUID]*Level),
		levelsLock: sync.Mutex{},
	}, nil
}

// CreateLevel creates a level with a Level ID and stores the game service callback for when the level ends.
func (service *LevelService) CreateLevel(players map[string]*Player, callback EndLevelCallback) (uuid.UUID, error) {
	levelID, err := uuid.NewRandom()
	if err != nil {
		return levelID, status.Errorf(codes.Internal, "level_creation_failed")
	}

	level := &Level{
		EndLevelCallback: callback,
		ID:               levelID,
		Players:          make(map[string]*Player, 0),
	}
	for _, player := range players {
		player.LevelChannel = NewChannel()
		level.Players[player.ID] = player
	}

	service.levelsLock.Lock()
	defer service.levelsLock.Unlock()
	service.levels[levelID] = level

	return level.ID, nil
}

// getLevel a utility function to get the level for the given level id
func (service *LevelService) getLevel(id string) (*Level, error) {
	levelID, err := uuid.Parse(id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid_level_id")
	}
	var level *Level
	var ok bool
	if level, ok = service.levels[levelID]; !ok {
		return nil, status.Errorf(codes.NotFound, "level_not_found")
	}
	return level, nil
}

// notify is a utility function to notify all players of level events.
// It sanitizes uneccessary data from events. It also stores non transient events
// on the level history to enable replaying and therefore resynchronization of players
func (service *LevelService) notify(level *Level, event *proto.LevelEvent) {
	event.PlayerId = ""
	event.LevelId = ""

	for _, player := range level.Players {
		player.LevelChannel.Send(event)
	}
}

// endLevel ends the current level by:
// 1. closing all player channels (causes disconnection of player streams)
// 2. removing the level from the active levels
// 3. Calling the callback on the game service
func (service *LevelService) endLevel(level *Level) {
	service.levelsLock.Lock() // ensure only one player ends the level - pessimistic
	defer service.levelsLock.Unlock()

	// checks that the level has not already been ended
	if _, ok := service.levels[level.ID]; !ok {
		return
	}

	Logger.Printf("INFO LevelService cleaning up level %v", level.ID)

	for _, player := range level.Players {
		player.LevelChannel.Close()
	}

	delete(service.levels, level.ID)
	level.EndLevelCallback()
}

// handleStreamDisconnected is a goroutine to cleanup after dirty disconnects. It
// also notifies all remaining players of the disconnect. It gives the player a
// recovery window before removing them (thereby shielding the other players from noisy network issues).

func (service *LevelService) handleStreamDisconnected(level *Level, player *Player) {
	select {
	case _, ok := <-player.LevelChannel.Recovered:
		if ok {
			Logger.Printf("INFO LevelService player recovered; level:%v player:%v", level.ID, player.ID)
			return
		}
	case <-time.After(LevelRecoveryWindow):
		Logger.Printf("INFO LevelService player timeout; level:%v player:%v", level.ID, player.ID)
	}
	service.endLevel(level)
}

// streamSend sends events from the current player channel to the player client
func (service *LevelService) streamSend(stream proto.LevelService_PlayServer, level *Level, player *Player) error {
	err := player.LevelChannel.Listen()
	if err != nil {
		Logger.Printf("WARN LevelService multiple listeners; level:%v player:%v", level.ID, player.ID)
		return status.Errorf(codes.InvalidArgument, err.Error())
	}
	defer player.LevelChannel.Hangup()

	// next section is to indicate recovery has occurred, and replay failed event
	player.LevelChannel.Recover()
	select {
	case event, ok := <-player.LevelChannel.SkipBack:
		if !ok { // closed by server
			return nil
		}
		levelEvent := event.(*proto.LevelEvent)
		Logger.Printf("DEBUG LevelService recovering level:%v player:%v event:%+v", level.ID, player.ID, event)
		err := stream.Context().Err()
		if err == nil {
			err = stream.Send(levelEvent)
		}
		Logger.Printf("DEBUG LevelService recovered level:%v player:%v event:%+v", level.ID, player.ID, event)
		if err != nil { // failed to send to client... again
			Logger.Printf("WARN LevelService event retry failed; level:%v player:%v", level.ID, player.ID)
			player.LevelChannel.Retry(levelEvent)
			go service.handleStreamDisconnected(level, player)
			return status.Errorf(codes.DataLoss, "listener_aborted")
		}
	default:
		break
	}

	// There are three things that could happen in the next code:
	// 1. An event can be dequeued
	// 2. The channel could be closed by the server
	// 3. The channel could be closed by the client
	events := player.LevelChannel.Events
	for {
		select {
		case event, ok := <-events:
			if !ok { // closed by server
				return nil
			}
			levelEvent := event.(*proto.LevelEvent)

			Logger.Printf("DEBUG LevelService sending level:%v player:%v event:%+v", level.ID, player.ID, event)
			err := stream.Context().Err()
			if err == nil {
				err = stream.Send(levelEvent)
			}
			Logger.Printf("DEBUG LevelService sent level:%v player:%v event:%+v", level.ID, player.ID, event)

			if err != nil { // failed to send to client
				Logger.Printf("WARN LevelService event send failed; level:%v player:%v", level.ID, player.ID)
				player.LevelChannel.Retry(levelEvent)
				go service.handleStreamDisconnected(level, player)
				return status.Errorf(codes.DataLoss, "listener_aborted")
			}
		case <-stream.Context().Done(): // closed by client
			Logger.Printf("WARN LevelService connection disconnected by client; level:%v player:%v", level.ID, player.ID)
			player.LevelChannel.Retry(nil)
			go service.handleStreamDisconnected(level, player)
			return nil
		}
	}
}

// streamReceive is a goroutine to receive and dispatch events from the player client
func (service *LevelService) streamReceive(stream proto.LevelService_PlayServer, level *Level, player *Player) error {
	for {
		event, err := stream.Recv()
		//Logger.Printf("DEBUG LevelService received level:%v player:%v event:%+v", level.ID, player.ID, event)
		if err != nil {
			if err.Error() != "rpc error: code = Canceled desc = context canceled" {
				Logger.Printf("INFO LevelService play receive failed; level:%v player:%v", level.ID, player.ID)
			}
			return err
		}
		service.notify(level, event)
	}
}

// End allows a player to cleanly end a level
func (service *LevelService) End(context context.Context, request *proto.EndLevelRequest) (*empty.Empty, error) {
	Logger.Printf("INFO LevelService end level %v", request.LevelId)
	if request.LevelId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid_level_id")
	}

	level, err := service.getLevel(request.LevelId)
	if err != nil {
		return nil, err
	}

	service.endLevel(level)
	return &empty.Empty{}, nil
}

// Play is for all bi-directional level events between the client and server
func (service *LevelService) Play(stream proto.LevelService_PlayServer) error {
	initialEvent, err := stream.Recv()
	if err != nil {
		return err
	}
	Logger.Printf("INFO LevelService playing; level:%v player:%v", initialEvent.LevelId, initialEvent.PlayerId)
	if initialEvent.PlayerId == "" {
		return status.Errorf(codes.InvalidArgument, "invalid_player_id")
	}

	level, err := service.getLevel(initialEvent.LevelId)
	if err != nil {
		return err
	}
	var player *Player
	var ok bool
	if player, ok = level.Players[initialEvent.PlayerId]; !ok {
		return status.Errorf(codes.InvalidArgument, "invalid_player_id")
	}

	go func() {
		service.streamReceive(stream, level, player)
	}()

	err = service.streamSend(stream, level, player)
	if err != nil {
		return err
	}

	return nil
}

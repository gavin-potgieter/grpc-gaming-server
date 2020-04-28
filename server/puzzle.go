package main

import (
	"sync"
	"time"

	"github.com/gavin-potgieter/sensense-server/server/proto"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PuzzleService provides a running service instance
type PuzzleService struct {
	puzzlesLock sync.Mutex
	puzzles     map[uuid.UUID]*Puzzle
}

// NewPuzzleService creates a new puzzle service
func NewPuzzleService() (*PuzzleService, error) {
	return &PuzzleService{
		puzzles:     make(map[uuid.UUID]*Puzzle),
		puzzlesLock: sync.Mutex{},
	}, nil
}

// rotateRoles rotates the player roles from the last time
func rotateRoles(players map[string]*Player) {
	for _, player := range players {
		player.Role = (player.Role + 1) % 3
	}
}

// CreatePuzzle creates a puzzle given the puzzle name and initial conditions, cycling the player roles automatically
func (service *PuzzleService) CreatePuzzle(name string, initialConditions string, duration int, players map[string]*Player, callback EndPuzzleCallback) (uuid.UUID, error) {
	puzzleID, err := uuid.NewRandom()
	if err != nil {
		return puzzleID, status.Errorf(codes.Internal, "puzzle_creation_failed")
	}

	puzzle := &Puzzle{
		EndPuzzleCallback: callback,
		ID:                puzzleID,
		InitialConditions: initialConditions,
		Name:              name,
		Players:           make(map[string]*Player, 0),
		timeRemaining:     duration,
		ticker:            time.NewTicker(time.Second),
	}
	for _, player := range players {
		puzzle.Players[player.ID] = player
	}

	rotateRoles(puzzle.Players)

	service.puzzlesLock.Lock()
	service.puzzles[puzzleID] = puzzle
	service.puzzlesLock.Unlock()

	go service.puzzleTicker(puzzle)

	return puzzle.ID, nil
}

func (service *PuzzleService) getPuzzle(id string) (*Puzzle, error) {
	puzzleID, err := uuid.Parse(id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid_puzzle_id")
	}
	var puzzle *Puzzle
	var ok bool
	if puzzle, ok = service.puzzles[puzzleID]; !ok {
		return nil, status.Errorf(codes.NotFound, "puzzle_not_found")
	}
	return puzzle, nil
}

// initializeStream sets up a player that is joining a puzzle, it does the following:
// 1. it creates the channel
// 2. it sends events with the name, intial conditions, player role to the player joining
// 3. Once there are enough players it sends the all aboard and remaining time events to all players
func (service *PuzzleService) initializeStream(stream proto.PuzzleService_SolveServer) (*Puzzle, *Player, error) {
	initialEvent, err := stream.Recv()
	if err != nil || initialEvent.Type != proto.PuzzleEvent_DATA_NONE {
		return nil, nil, status.Errorf(codes.Internal, "no_puzzle_initial_event_received")
	}
	Logger.Printf("INFO PuzzleService solving; puzzle:%v player:%v", initialEvent.PuzzleId, initialEvent.PlayerId)
	if initialEvent.PlayerId == "" {
		return nil, nil, status.Errorf(codes.InvalidArgument, "invalid_player_id")
	}
	puzzle, err := service.getPuzzle(initialEvent.PuzzleId)
	if err != nil {
		return nil, nil, err
	}
	var player *Player
	var ok bool
	if player, ok = puzzle.Players[initialEvent.PlayerId]; !ok {
		return nil, nil, status.Errorf(codes.InvalidArgument, "invalid_player_id")
	}
	puzzle.Lock.Lock() // ensures that the game hasn't ended, and only the last player unpauses the game
	defer puzzle.Lock.Unlock()
	if puzzle.timeRemaining <= 0 {
		return nil, nil, status.Errorf(codes.FailedPrecondition, "puzzle_ended")
	}
	player.PuzzleChannelLock.Lock()
	player.PuzzleChannel = make(chan *proto.PuzzleEvent, 10) // 0 channels block if there are no receivers
	player.PuzzleChannelLock.Unlock()

	// these sends don't need to be locked, but we would like the stream order to be consistent
	stream.Send(&proto.PuzzleEvent{
		Type:        proto.PuzzleEvent_DATA_STRING,
		Key:         NameKey,
		ValueString: puzzle.Name,
	})
	stream.Send(&proto.PuzzleEvent{
		Type:        proto.PuzzleEvent_DATA_STRING,
		Key:         InitialConditionsKey,
		ValueString: puzzle.InitialConditions,
	})
	stream.Send(&proto.PuzzleEvent{
		Type:     proto.PuzzleEvent_DATA_STRING,
		Key:      RoleKey,
		ValueInt: int32(player.Role),
	})
	// these DO need to be locked, because the paused function is driven by the number of active player channels
	if !puzzle.Paused() {
		service.notify(puzzle, &proto.PuzzleEvent{
			Type:     proto.PuzzleEvent_DATA_INT,
			Key:      TimeKey,
			ValueInt: int32(puzzle.timeRemaining),
		})
		service.notify(puzzle, &proto.PuzzleEvent{
			Key: AllAboardKey,
		})
	}
	return puzzle, player, nil
}

// endPuzzle ends the current puzzle by:
// 1. stopping the puzzle ticker
// 2. closing all player channels (causes disconnection of player streams)
// 3. removing the puzzle from the active puzzles
func (service *PuzzleService) endPuzzle(puzzle *Puzzle) {
	service.puzzlesLock.Lock() // ensure only one player ends the puzzle - pessimistic
	defer service.puzzlesLock.Unlock()

	// checks that the puzzle has not already been ended
	if _, ok := service.puzzles[puzzle.ID]; !ok {
		return
	}

	Logger.Printf("INFO PuzzleService ending puzzle; puzzle:%v", puzzle.ID)
	puzzle.Lock.Lock() // ensures
	if puzzle.ticker != nil {
		puzzle.ticker.Stop()
		puzzle.ticker = nil
	}
	puzzle.timeRemaining = 0
	puzzle.Lock.Unlock()

	for _, player := range puzzle.Players {
		player.PuzzleChannelLock.Lock()
		if player.PuzzleChannel != nil {
			Logger.Printf("INFO PuzzleService closing channel; puzzle:%v player:%v", puzzle.ID, player.ID)
			close(player.PuzzleChannel)
		}
		player.PuzzleChannel = nil
		player.PuzzleChannelLock.Unlock()
	}

	delete(service.puzzles, puzzle.ID)
	puzzle.EndPuzzleCallback()
}

// puzzleTicker is a goroutine to broadcast events every x seconds with
// the remaining game time; it also ends the game when the time runs out
func (service *PuzzleService) puzzleTicker(puzzle *Puzzle) {
	skippedFirst := false
	for {
		<-puzzle.ticker.C
		if puzzle.timeRemaining <= 0 {
			service.notify(puzzle, &proto.PuzzleEvent{
				Type:        proto.PuzzleEvent_DATA_INT,
				Key:         ResultKey,
				ValueString: ResultLose,
			})
			service.endPuzzle(puzzle)
			return
		} else if skippedFirst && puzzle.timeRemaining%TimerInterval == 0 && !puzzle.Paused() {
			service.notify(puzzle, &proto.PuzzleEvent{
				Type:     proto.PuzzleEvent_DATA_INT,
				Key:      TimeKey,
				ValueInt: int32(puzzle.timeRemaining),
			})
		}
		if !puzzle.Paused() {
			puzzle.timeRemaining--
			skippedFirst = true
		}
	}
}

// notify notifies all players of events
func (service *PuzzleService) notify(puzzle *Puzzle, event *proto.PuzzleEvent) {
	for _, player := range puzzle.Players {
		player.PuzzleChannelLock.RLock()
		playerHasAnOpenChannel := player.PuzzleChannel != nil
		if playerHasAnOpenChannel {
			event.PlayerId = ""
			player.PuzzleChannel <- event
		}
		player.PuzzleChannelLock.RUnlock()
	}
}

func (service *PuzzleService) handleStreamDisconnected(puzzle *Puzzle, player *Player) {
	player.PuzzleChannelLock.Lock()
	if player.PuzzleChannel != nil {
		close(player.PuzzleChannel)
	}
	player.PuzzleChannel = nil
	player.PuzzleChannelLock.Unlock()

	service.notify(puzzle, &proto.PuzzleEvent{
		Type:     proto.PuzzleEvent_DATA_INT,
		Key:      PlayerMissingKey,
		ValueInt: int32(player.Role),
	})

	time.Sleep(PlayerRecoveryTime)
	if player.PuzzleChannel != nil {
		Logger.Printf("INFO PuzzleService recovered player; puzzle:%v player:%v", puzzle.ID, player.ID)
		return
	}
	Logger.Printf("WARN PuzzleService lost player; game:%v player:%v", puzzle.ID, player.ID)
	service.endPuzzle(puzzle)
}

func (service *PuzzleService) streamSend(stream proto.PuzzleService_SolveServer, puzzle *Puzzle, player *Player) error {
	for {
		select {
		case event, ok := <-player.PuzzleChannel:
			if !ok {
				return nil
			}
			err := stream.Send(event)
			if event.Key == ResultKey {
				service.endPuzzle(puzzle)
			}
			if err != nil {
				Logger.Printf("WARN PuzzleService event send failed; puzzle:%v player:%v", puzzle.ID, player.ID)
				go service.handleStreamDisconnected(puzzle, player)
				return status.Errorf(codes.DataLoss, "listener_aborted")
			}
		case <-stream.Context().Done():
			Logger.Printf("WARN PuzzleService connection disconnected by client; puzzle:%v player:%v", puzzle.ID, player.ID)
			go service.handleStreamDisconnected(puzzle, player)
			return nil
		}
	}
}

// streamReceive is a goroutine
func (service *PuzzleService) streamReceive(stream proto.PuzzleService_SolveServer, puzzle *Puzzle, player *Player) error {
	for {
		event, err := stream.Recv()
		if err != nil {
			Logger.Printf("WARN PuzzleService solve receive failed; puzzle:%v player:%v", puzzle.ID, player.ID)
			go service.handleStreamDisconnected(puzzle, player)
			return err
		}
		service.notify(puzzle, event)
	}
}

// Solve is for all bi-directional puzzle events between the client and server
func (service *PuzzleService) Solve(stream proto.PuzzleService_SolveServer) error {
	puzzle, player, err := service.initializeStream(stream)
	if err != nil {
		return err
	}

	go func() {
		service.streamReceive(stream, puzzle, player)
	}()

	err = service.streamSend(stream, puzzle, player)
	if err != nil {
		return err
	}

	return nil
}

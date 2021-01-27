package main

import (
	"context"
	"sync"
	"time"

	"github.com/gavin-potgieter/sensense-server/test_client/proto"
	"google.golang.org/grpc"
)

const (
	Blind = 0
	Deaf  = 1
	Mute  = 2
)

// BEWARE: this client code has a known issue where the player who disconnects
// from the second puzzle and waits 5 seconds after recovery, sometimes delays
// their message receiving. Sometimes the end falls over.
type Player struct {
	PlayerID        string
	Role            int
	Railway         *Railway
	matchConnection *grpc.ClientConn
	matchService    proto.MatchServiceClient
	gameConnection  *grpc.ClientConn
	gameService     proto.GameServiceClient
	eventChannel    chan *proto.GameEvent
	gameID          string
	recovering      bool
}

type MatchEventCallback func(*proto.MatchEvent) error

type GameEventCallback func(*proto.GameEvent) error

// connectMatch is used to create the match; the only reason connections are
// being created twice is to allow for testing
func (player *Player) connectMatch() error {
	if player.gameConnection != nil {
		return nil
	}
	//52.143.158.107:5000
	//localhost:8080
	conn, err := grpc.Dial("localhost:8080", grpc.WithInsecure())
	if err != nil {
		return err
	}

	player.matchConnection = conn
	player.matchService = proto.NewMatchServiceClient(conn)
	return nil
}

// connectGame is used to play the game; the only reason connections are
// being created twice is to allow for testing
func (player *Player) connectGame() error {
	conn, err := grpc.Dial("localhost:8080", grpc.WithInsecure())
	if err != nil {
		return err
	}

	player.gameConnection = conn
	player.gameService = proto.NewGameServiceClient(conn)
	player.eventChannel = make(chan *proto.GameEvent, 10)
	//Logger.Printf("DEBUG create event channel %v", player.PlayerID)

	return nil
}

// DisconnectMatch is used to simulate bad network connectivity
func (player *Player) DisconnectMatch(connection *grpc.ClientConn) error {
	err := connection.Close()
	if err != nil {
		return err
	}
	return nil
}

// DisconnectGame is used to simulate bad network connectivity
func (player *Player) DisconnectGame() error {
	err := player.gameConnection.Close()
	if err != nil {
		return err
	}

	close(player.eventChannel)
	//Logger.Printf("DEBUG delete event channel %v", player.PlayerID)
	player.gameConnection = nil
	player.gameService = nil

	return nil
}

func NewPlayer(playerID string, railway *Railway) (*Player, error) {
	player := &Player{
		PlayerID: playerID,
		Railway:  railway,
		Role:     -1,
	}
	err := player.connectMatch()
	if err != nil {
		return nil, err
	}
	return player, nil
}

func (player *Player) Match(group *sync.WaitGroup, callback MatchEventCallback) error {
	err := player.connectMatch()
	if err != nil {
		return err
	}

	stream, err := player.matchService.Match(context.Background(), &proto.MatchRequest{
		Game:        "Sensense",
		PlayerId:    player.PlayerID,
		PlayerLimit: 3,
		Code:        MatchCode,
	})
	if err != nil {
		return err
	}
	Logger.Printf("INFO %v matching; game_code: %v", player.PlayerID, MatchCode)
	for {
		event, err := stream.Recv()
		if err != nil {
			return err
		}

		Logger.Printf("INFO %v match event; event %+v", player.PlayerID, event)
		if event.GameId == "" {

			MatchCode = event.Code
		} else {
			player.Role = int(event.PlayerIndex)
			player.gameID = event.GameId
			go func() {
				player.Play()
				if player.recovering {
					Logger.Printf("INFO rejoining game %v\n", player.PlayerID)
					time.Sleep(4 * time.Second)
					player.connectGame()
					player.Play()
					player.recovering = false
				}
			}()
		}
		Logger.Printf("INFO %v %+v\n", player.PlayerID, event)
		if callback == nil {
			continue
		}
		err = callback(event)
		if err != nil {
			return err
		}
	}
}

func (player *Player) Play() error {
	err := player.connectGame()
	if err != nil {
		return err
	}

	stream, err := player.gameService.Play(context.Background())
	if err != nil {
		return err
	}
	err = stream.Send(&proto.GameEvent{
		PlayerId: player.PlayerID,
		GameId:   player.gameID,
	})
	if err != nil {
		return err
	}
	go func() {
		for {
			event, ok := <-player.eventChannel
			if !ok {
				Logger.Printf("INFO channel closed %v\n", player.PlayerID)
				return
			}
			stream.Send(event)
		}
	}()

	if !player.recovering {
		player.SendFirstPuzzleEvent()
	}

	Logger.Printf("INFO %v listening to game; game_id: %v", player.PlayerID, player.gameID)
	for {
		event, err := stream.Recv()
		if err != nil {
			stream.CloseSend()
			time.Sleep(1 * time.Second)
			Logger.Printf("ERROR %v %v\n", player.PlayerID, err)
			return err
		}
		Logger.Printf("INFO %v %+v\n", player.PlayerID, event)
		if event.Key != "PLAYER_COUNT" {
			player.ProcessPuzzleEvent(event)
		}
	}
}

func (player *Player) SendEvent(event *proto.GameEvent) {
	player.eventChannel <- event
}

func (player *Player) SendFirstPuzzleEvent() {
	switch PuzzleName {
	case "P1":
		if player.Role == Mute {
			player.SendLevelEvent("wave", 0) //0
		}
	case "P2":
		if player.Role == Deaf {
			player.SendLevelEvent("gulp", 0) //0
		}
	}
}

func (player *Player) ProcessPuzzleEvent(event *proto.GameEvent) {
	switch event.ValueInt {
	case 0:
		if player.Role == Deaf {
			player.SendLevelEvent("shout uselessly", -1)
			player.SendLevelEvent("shout usefully", 1) //1
		}
		if player.Role == Blind {
			player.SendLevelEvent("cricket chirps", -1)
		}
	case 1:
		if player.Role == Blind {
			player.SendLevelEvent("touch_pos", 2) //2
			player.SendLevelEvent("touch_pos", -1)
		}
	case 2:
		if player.Role == Mute {
			player.SendLevelEvent("spin", 3) //3
		}

	case 3:
		if player.Role == Blind {
			player.SendLevelEvent("tapped", -1)
			player.SendLevelEvent("held", 4) //4
		}
	case 4:
		if player.Role == Mute {
			player.SendLevelEvent("twiddled knob", 5)         //5
			player.SendLevelEvent("twiddled another knob", 6) //6
		}
	case 5:
		if player.Role == Mute {
			player.recovering = true
			player.DisconnectGame()
		}
		if player.Role == Deaf {
			player.SendLevelEvent("swiped up", 7)   //7
			player.SendLevelEvent("swiped down", 8) //8
		}
	case 8:
		if player.Role == Blind {
			player.SendLevelEvent("we won!", 9) //9
		}
	case 9:
		if player.Role == Mute {
			player.SendLevelEvent("yay!", 10) //10
		}
	case 10:
		if player.Role == Mute {
			player.Railway.GameEndedSignal.L.Lock()
			player.Railway.GameEndedSignal.Broadcast()
			player.Railway.GameEndedSignal.L.Unlock()
		}
	}
}

func (player *Player) SendLevelEvent(key string, next int) {
	player.SendEvent(&proto.GameEvent{
		Type:     proto.GameEvent_DATA_INT,
		Key:      key,
		ValueInt: int32(next),
	})
}

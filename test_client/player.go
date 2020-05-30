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
	GameID           string
	PlayerID         string
	Role             int
	PuzzleName       string
	Railway          *Railway
	gameConnection   *grpc.ClientConn
	gameService      proto.GameServiceClient
	puzzleConnection *grpc.ClientConn
	puzzleService    proto.LevelServiceClient
	eventChannel     chan *proto.LevelEvent
}

type GameEventCallback func(*proto.GameEvent) error

type LevelEventCallback func(*proto.LevelEvent) error

// connectGame is used to create the game; the only reason connections are
// being created twice is to allow for testing
func (player *Player) connectGame() error {
	if player.gameConnection != nil {
		return nil
	}
	//52.143.158.107:80
	conn, err := grpc.Dial("localhost:8080", grpc.WithInsecure())
	if err != nil {
		return err
	}

	player.gameConnection = conn
	player.gameService = proto.NewGameServiceClient(conn)
	return nil
}

// connectPuzzle is used to create the puzzle; the only reason connections are
// being created twice is to allow for testing
func (player *Player) connectPuzzle() error {
	conn, err := grpc.Dial("localhost:8080", grpc.WithInsecure())
	if err != nil {
		return err
	}

	player.puzzleConnection = conn
	player.puzzleService = proto.NewLevelServiceClient(conn)
	player.eventChannel = make(chan *proto.LevelEvent, 10)
	//Logger.Printf("DEBUG create event channel %v", player.PlayerID)

	return nil
}

// DisconnectGame is used to simulate bad network connectivity
func (player *Player) DisconnectGame() error {
	err := player.gameConnection.Close()
	if err != nil {
		return err
	}
	player.gameConnection = nil
	player.gameService = nil
	return nil
}

// DisconnectPuzzle is used to simulate bad network connectivity
func (player *Player) DisconnectPuzzle() error {
	err := player.puzzleConnection.Close()
	if err != nil {
		return err
	}

	close(player.eventChannel)
	//Logger.Printf("DEBUG delete event channel %v", player.PlayerID)
	player.puzzleConnection = nil
	player.puzzleService = nil

	return nil
}

func NewPlayer(playerID string, railway *Railway) (*Player, error) {
	player := &Player{
		PlayerID: playerID,
		Railway:  railway,
		Role:     -1,
	}
	err := player.connectGame()
	if err != nil {
		return nil, err
	}
	return player, nil
}

func (player *Player) CreateGame() error {
	err := player.connectGame()
	if err != nil {
		return err
	}

	response, err := player.gameService.Create(context.Background(), &proto.CreateGameRequest{
		PlayerId:    player.PlayerID,
		GameName:    "sensense",
		PlayerCount: 3,
	})
	if err != nil {
		return err
	}
	GameCode = response.GameCode
	player.GameID = response.GameId
	player.Role = int(response.Index)
	Logger.Printf("INFO %v created game; code: %v game_id: %v", player.PlayerID, GameCode, player.GameID)
	return nil
}

func (player *Player) JoinGame() error {
	err := player.connectGame()
	if err != nil {
		return err
	}

	response, err := player.gameService.Join(context.Background(), &proto.JoinGameRequest{
		PlayerId: player.PlayerID,
		GameCode: GameCode,
		GameName: "sensense",
	})
	if err != nil {
		return err
	}
	player.GameID = response.GameId
	player.Role = int(response.Index)
	Logger.Printf("INFO %v joined game; code: %v game_id: %v", player.PlayerID, GameCode, player.GameID)
	return nil
}

func (player *Player) RejoinGame() error {
	err := player.connectGame()
	if err != nil {
		return err
	}

	_, err = player.gameService.Rejoin(context.Background(), &proto.RejoinGameRequest{
		PlayerId: player.PlayerID,
		GameId:   player.GameID,
	})
	if err != nil {
		return err
	}
	Logger.Printf("INFO %v rejoined game; code: %v game_id: %v", player.PlayerID, GameCode, player.GameID)
	return nil
}

func (player *Player) ListenGame(group *sync.WaitGroup, callback GameEventCallback) error {
	if group != nil {
		defer group.Done()
	}

	err := player.connectGame()
	if err != nil {
		return err
	}

	stream, err := player.gameService.Listen(context.Background(), &proto.ListenGame{
		GameId:   player.GameID,
		PlayerId: player.PlayerID,
	})
	if err != nil {
		return err
	}
	Logger.Printf("INFO %v listening to game; game_id: %v", player.PlayerID, player.GameID)
	for {
		event, err := stream.Recv()
		if err != nil {
			return err
		}

		switch event.Type {
		case proto.GameEvent_LEVEL_STARTED:
			PuzzleID = event.LevelId
			player.PuzzleName = event.Options["Name"]
			go func() {
				player.ListenPuzzle(0)
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

func (player *Player) Leave() error {
	err := player.connectGame()
	if err != nil {
		return err
	}

	_, err = player.gameService.Leave(context.Background(), &proto.LeaveGameRequest{
		PlayerId: player.PlayerID,
		GameId:   player.GameID,
	})
	if err != nil {
		return err
	}
	Logger.Printf("INFO %v left game; game_id: %v", player.PlayerID, player.GameID)
	return nil
}

func (player *Player) StartPuzzle(name string, time int) error {
	err := player.connectGame()
	if err != nil {
		return err
	}

	_, err = player.gameService.StartLevel(context.Background(), &proto.StartLevelRequest{
		GameId: player.GameID,
		Options: map[string]string{
			"Name":              name,
			"InitialConditions": "some initial conditions",
			"Time":              string(time),
		},
	})
	if err != nil {
		return err
	}
	Logger.Printf("INFO %v started puzzle; game_id: %v", player.PlayerID, player.GameID)
	return nil
}

func (player *Player) ListenPuzzle(sequence int) error {
	err := player.connectPuzzle()
	if err != nil {
		return err
	}

	stream, err := player.puzzleService.Play(context.Background())
	if err != nil {
		return err
	}
	err = stream.Send(&proto.LevelEvent{
		PlayerId: player.PlayerID,
		LevelId:  PuzzleID,
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

	player.SendFirstPuzzleEvent()

	Logger.Printf("INFO %v listening to puzzle; puzzle_id: %v", player.PlayerID, PuzzleID)
	for {
		event, err := stream.Recv()
		if err != nil {
			stream.CloseSend()
			time.Sleep(1 * time.Second)
			Logger.Printf("ERROR %v %v\n", player.PlayerID, err)
			return err
		}
		Logger.Printf("INFO %v %+v\n", player.PlayerID, event)

		player.ProcessPuzzleEvent(event)
	}
}

func (player *Player) SendEvent(event *proto.LevelEvent) {
	player.eventChannel <- event
}

func (player *Player) SendFirstPuzzleEvent() {
	switch player.PuzzleName {
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

func (player *Player) ProcessPuzzleEvent(event *proto.LevelEvent) {
	switch player.PuzzleName {
	case "P1":
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
			if player.Role == Deaf {
				player.EndPuzzle()
				player.Railway.FirstPuzzleEnded.Unlock()
			}
		}
	case "P2":
		switch event.ValueInt {
		case 0:
			if player.Role == Blind {
				player.SendLevelEvent("tapped", -1)
				player.SendLevelEvent("held", 1) //1
			}
		case 1:
			if player.Role == Mute {
				player.SendLevelEvent("twiddled knob", 2)         //2
				player.SendLevelEvent("twiddled another knob", 3) //3
			}
		case 3:
			if player.Role == Mute {
				time.Sleep(4 * time.Second)
				player.DisconnectPuzzle()
				time.Sleep(3 * time.Second)
				go func() {
					err := player.ListenPuzzle(4)
					if err != nil {
						Logger.Printf("ERROR %v %v\n", player.PlayerID, err)
					}
				}()
			}
			if player.Role == Deaf {
				player.SendLevelEvent("swiped up", 4)   //4
				player.SendLevelEvent("swiped down", 5) //5
			}

		case 5:
			if player.Role == Mute {
				time.Sleep(5 * time.Second)
				player.SendLevelEvent("recovered", 6) //6
			}
		case 6:
			if player.Role == Blind {
				player.EndPuzzle()
				player.Railway.SecondPuzzleEnded.Unlock()
			}
		}
	}

}

func (player *Player) EndPuzzle() {
	player.puzzleService.End(context.Background(), &proto.EndLevelRequest{LevelId: PuzzleID})
}

func (player *Player) SendLevelEvent(key string, next int) {
	player.SendEvent(&proto.LevelEvent{
		Type:     proto.LevelEvent_DATA_INT,
		Key:      key,
		ValueInt: int32(next),
	})
}

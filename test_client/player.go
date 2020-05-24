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
	puzzleService    proto.PuzzleServiceClient
	eventChannel     chan *proto.PuzzleEvent
}

type GameEventCallback func(*proto.GameEvent) error

type PuzzleEventCallback func(*proto.PuzzleEvent) error

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
	player.puzzleService = proto.NewPuzzleServiceClient(conn)
	player.eventChannel = make(chan *proto.PuzzleEvent, 10)
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
		PlayerId: player.PlayerID,
	})
	if err != nil {
		return err
	}
	GameCode = response.GameCode
	player.GameID = response.GameId
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
	})
	if err != nil {
		return err
	}
	player.GameID = response.GameId
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
		case proto.GameEvent_PUZZLE_STARTED:
			PuzzleID = event.PuzzleId
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

	_, err = player.gameService.StartPuzzle(context.Background(), &proto.StartPuzzleRequest{
		GameId:            player.GameID,
		Name:              name,
		InitialConditions: "some initial conditions",
		Time:              int32(time),
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

	stream, err := player.puzzleService.Solve(context.Background())
	if err != nil {
		return err
	}
	err = stream.Send(&proto.PuzzleEvent{
		PlayerId: player.PlayerID,
		PuzzleId: PuzzleID,
		Type:     proto.PuzzleEvent_DATA_INT,
		Key:      "SEQN",
		ValueInt: int32(sequence),
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

		switch event.Key {
		case "ROLE":
			player.Role = int(event.ValueInt)
			player.SendFirstPuzzleEvent()
		case "NAME":
			player.PuzzleName = event.ValueString
		case "RSLT":
			if player.PuzzleName == "P1" && player.Role == Deaf {
				player.Railway.FirstPuzzleEnded.Unlock()
			}
			if player.PuzzleName == "P2" && player.Role == Deaf {
				player.Railway.SecondPuzzleEnded.Unlock()
			}
		default:
			player.ProcessPuzzleEvent(event)
		}
	}
}

func (player *Player) SendEvent(event *proto.PuzzleEvent) {
	player.eventChannel <- event
}

func (player *Player) SendFirstPuzzleEvent() {
	switch player.PuzzleName {
	case "P1":
		if player.Role == Mute {
			player.SendStringEvent("wave", true) //0
		}
	case "P2":
		if player.Role == Deaf {
			player.SendStringEvent("gulp", true) //0
		}
	}
}

func (player *Player) ProcessPuzzleEvent(event *proto.PuzzleEvent) {
	if !event.Durable {
		return
	}

	switch player.PuzzleName {
	case "P1":
		switch event.Sequence {
		case 0:
			if player.Role == Deaf {
				player.SendStringEvent("shout uselessly", false)
				player.SendStringEvent("shout usefully", true) //1
			}
			if player.Role == Blind {
				player.SendStringEvent("cricket chirps", false)
			}
		case 1:
			if player.Role == Blind {
				player.SendIntEvent("touch_pos", 5, true) //2
				player.SendIntEvent("touch_pos", 2, false)
				player.SendIntEvent("touch_pos", 3, false)
			}
		case 2:
			if player.Role == Mute {
				player.SendStringEvent("spin", true) //3
			}
		case 3:
			if player.Role == Deaf {
				player.Win()
			}
		}
	case "P2":
		switch event.Sequence {
		case 0:
			if player.Role == Blind {
				player.SendStringEvent("tapped", false)
				player.SendStringEvent("held", true) //1
			}
		case 1:
			if player.Role == Mute {
				player.SendStringEvent("twiddled knob", true)         //2
				player.SendStringEvent("twiddled another knob", true) //3
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
				player.SendStringEvent("swiped up", true)   //4
				player.SendStringEvent("swiped down", true) //5
			}

		case 5:
			if player.Role == Mute {
				time.Sleep(5 * time.Second)
				player.SendStringEvent("recovered", true) //6
			}
		case 6:
			if player.Role == Blind {
				player.Abort()
			}
		}
	}

}

func (player *Player) SendIntEvent(key string, value int, durable bool) {
	player.SendEvent(&proto.PuzzleEvent{
		Type:     proto.PuzzleEvent_DATA_INT,
		Key:      key,
		ValueInt: int32(value),
		Durable:  durable,
	})
}

func (player *Player) SendStringEvent(value string, durable bool) {
	player.SendEvent(&proto.PuzzleEvent{
		Type:        proto.PuzzleEvent_DATA_STRING,
		ValueString: value,
		Durable:     durable,
	})
}

func (player *Player) Win() {
	player.SendEvent(&proto.PuzzleEvent{
		Type:        proto.PuzzleEvent_DATA_STRING,
		Key:         "RSLT",
		ValueString: "WON",
	})
}

func (player *Player) Abort() {
	player.SendEvent(&proto.PuzzleEvent{
		Type:        proto.PuzzleEvent_DATA_STRING,
		Key:         "RSLT",
		ValueString: "ABORT",
	})
}

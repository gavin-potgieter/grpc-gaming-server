package main

import (
	"context"
	"sync"

	"github.com/gavin-potgieter/sensense-server/test_client/proto"
	"google.golang.org/grpc"
)

const (
	Blind = 0
	Deaf  = 1
	Mute  = 2
)

type Player struct {
	GameID           string
	PlayerID         string
	Role             int
	PuzzleName       string
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
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
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
	if player.puzzleConnection != nil {
		return nil
	}
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		return err
	}

	player.puzzleConnection = conn
	player.puzzleService = proto.NewPuzzleServiceClient(conn)
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

func NewPlayer(playerID string) (*Player, error) {
	player := &Player{
		PlayerID:     playerID,
		eventChannel: make(chan *proto.PuzzleEvent, 10),
		Role:         -1,
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
				err := player.ListenPuzzle(0)
				if err != nil {
					Logger.Printf("ERROR %v %v\n", player.PlayerID, err)
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
				return
			}
			stream.Send(event)
		}
	}()

	Logger.Printf("INFO %v listening to puzzle; puzzle_id: %v", player.PlayerID, PuzzleID)
	for {
		event, err := stream.Recv()
		if err != nil {
			return err
		}
		Logger.Printf("INFO %v %+v\n", player.PlayerID, event)

		switch event.Key {
		case "ROLE":
			player.Role = int(event.ValueInt)
			player.SendFirstPuzzleEvent()
		case "NAME":
			player.PuzzleName = event.ValueString
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

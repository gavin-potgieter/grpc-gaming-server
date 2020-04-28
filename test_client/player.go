package main

import (
	"context"
	"sync"

	"github.com/gavin-potgieter/sensense-server/test_client/proto"
	"google.golang.org/grpc"
)

type Player struct {
	GameID           string
	PlayerID         string
	gameConnection   *grpc.ClientConn
	gameService      proto.GameServiceClient
	puzzleConnection *grpc.ClientConn
	puzzleService    proto.PuzzleServiceClient
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
		PlayerID: playerID,
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

func (player *Player) StartPuzzle() error {
	err := player.connectGame()
	if err != nil {
		return err
	}

	_, err = player.gameService.StartPuzzle(context.Background(), &proto.StartPuzzleRequest{
		GameId:            player.GameID,
		Name:              "a puzzle",
		InitialConditions: "some initial conditions",
		Time:              10,
	})
	if err != nil {
		return err
	}
	Logger.Printf("INFO %v started puzzle; game_id: %v", player.PlayerID, player.GameID)
	return nil
}

func (player *Player) ListenPuzzle(group *sync.WaitGroup, callback PuzzleEventCallback) error {
	if group != nil {
		defer group.Done()
	}

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
	})
	if err != nil {
		return err
	}
	Logger.Printf("INFO %v listening to puzzle; puzzle_id: %v", player.PlayerID, PuzzleID)
	for {
		event, err := stream.Recv()
		if err != nil {
			return err
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

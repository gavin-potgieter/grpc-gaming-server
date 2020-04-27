package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/gavin-potgieter/sensense-server/server/proto"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	_, filename = path.Split(os.Args[0])
	// Logger is the default logger
	Logger = log.New(os.Stdout, filename+" ", log.LstdFlags)
)

// GameService provides a running service instance
type GameService struct {
	gameCodes     map[int]*Game
	games         map[uuid.UUID]*Game
	puzzleService *PuzzleService
}

// NewGameService creates a new GameService
func NewGameService(ps *PuzzleService) (*GameService, error) {
	return &GameService{
		gameCodes:     make(map[int]*Game, 0),
		games:         make(map[uuid.UUID]*Game, 0),
		puzzleService: ps,
	}, nil
}

func createGameCode() int {
	code, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return int(code.Int64())
}

func isDuplicate(service *GameService, gameCode int) bool {
	_, ok := service.gameCodes[gameCode]
	return ok
}

// Create creates a new game
func (service GameService) Create(context context.Context, request *proto.CreateGameRequest) (*proto.CreateGameResponse, error) {
	Logger.Printf("INFO GameService creating; player:%v", request.PlayerId)
	if request.PlayerId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid_player_id")
	}
	gameID, err := uuid.NewRandom()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "game_creation_failed")
	}

	code := 0
	for code = createGameCode(); isDuplicate(&service, code); code = createGameCode() {
		// pass
	}

	game := &Game{
		Code: code,
		ID:   gameID,
		Players: map[string]*Player{
			request.PlayerId: &Player{ID: request.PlayerId},
		},
	}
	service.games[gameID] = game
	service.gameCodes[code] = game
	return &proto.CreateGameResponse{
		GameCode: fmt.Sprintf("%06.f", float64(code)),
		GameId:   gameID.String(),
	}, nil
}

func findMissingRole(players map[string]*Player) (Role, error) {
	for role := Role(0); role < PlayerLimit; role++ {
		found := false
		for _, player := range players {
			if player.Role == role {
				found = true
				break
			}
		}
		if !found {
			return role, nil
		}
	}
	return Role(0), status.Errorf(codes.ResourceExhausted, "game_full")
}

func join(service *GameService, game *Game, playerID string) error {
	if _, ok := game.Players[playerID]; ok {
		return nil
	}
	if len(game.Players) >= PlayerLimit {
		return status.Errorf(codes.ResourceExhausted, "game_full")
	}
	role, err := findMissingRole(game.Players)
	if err != nil {
		return err
	}
	game.Players[playerID] = &Player{
		ID:   playerID,
		Role: role,
	}
	service.notify(game, &proto.GameEvent{
		Type:  proto.GameEvent_PLAYER_COUNT_CHANGED,
		Count: int32(len(game.Players)),
	})
	return nil
}

// Join allows a player to join the game
func (service GameService) Join(context context.Context, request *proto.JoinGameRequest) (*proto.JoinGameResponse, error) {
	Logger.Printf("INFO GameService joining; code:%v player:%v", request.GameCode, request.PlayerId)
	if request.PlayerId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid_player_id")
	}
	gameCode, err := strconv.Atoi(request.GameCode)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid_game_code")
	}
	var game *Game
	var ok bool
	if game, ok = service.gameCodes[gameCode]; !ok {
		return nil, status.Errorf(codes.NotFound, "game_not_found")
	}
	err = join(&service, game, request.PlayerId)
	if err != nil {
		return nil, err
	}
	return &proto.JoinGameResponse{
		GameId: game.ID.String(),
	}, nil
}

// Rejoin allows a player to rejoin the game
func (service GameService) Rejoin(context context.Context, request *proto.RejoinGameRequest) (*empty.Empty, error) {
	Logger.Printf("INFO GameService rejoining player:%v game:%v\n", request.PlayerId, request.GameId)
	if request.PlayerId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid_player_id")
	}
	game, err := service.getGame(request.GameId)
	if err != nil {
		return nil, err
	}

	err = join(&service, game, request.PlayerId)
	if err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (service GameService) deleteGame(gameID uuid.UUID) {
	game := service.games[gameID]
	delete(service.gameCodes, game.Code)
	delete(service.games, game.ID)
}

// Leave allows players to cleanly leave a game
func (service GameService) Leave(context context.Context, request *proto.LeaveGameRequest) (*empty.Empty, error) {
	Logger.Printf("INFO GameService leaving; game:%v player:%v", request.GameId, request.PlayerId)
	if request.PlayerId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid_player_id")
	}
	game, err := service.getGame(request.GameId)
	if err != nil {
		return nil, err
	}

	var player *Player
	var ok bool
	if player, ok = game.Players[request.PlayerId]; !ok {
		return &empty.Empty{}, nil
	}

	service.leave(game, player)
	return &empty.Empty{}, nil
}

func (service GameService) notify(game *Game, event *proto.GameEvent) {
	for _, player := range game.Players {
		if player.GameChannel != nil {
			player.GameChannel <- event
		}
	}
}

func (service GameService) leave(game *Game, player *Player) {
	if player.GameChannel != nil {
		close(player.GameChannel)
		player.GameChannel = nil
	}
	delete(game.Players, player.ID)
	if len(game.Players) == 0 {
		service.deleteGame(game.ID)
	} else {
		service.notify(game, &proto.GameEvent{
			Type:  proto.GameEvent_PLAYER_COUNT_CHANGED,
			Count: int32(len(game.Players)),
		})
	}
}

func handleStreamDisconnected(service *GameService, game *Game, player *Player) {
	close(player.GameChannel)
	player.GameChannel = nil
	time.Sleep(PlayerRecoveryTime)
	if player.GameChannel != nil {
		Logger.Printf("INFO GameService recovered player; game:%v player:%v", game.ID, player.ID)
		return
	}
	Logger.Printf("WARN GameService lost player; game:%v player:%v", game.ID, player.ID)
	service.leave(game, player)
}

func (service GameService) getGame(id string) (*Game, error) {
	gameID, err := uuid.Parse(id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid_game_id")
	}
	var game *Game
	var ok bool
	if game, ok = service.games[gameID]; !ok {
		return nil, status.Errorf(codes.NotFound, "game_not_found")
	}
	return game, nil
}

// StartPuzzle starts a puzzle
func (service GameService) StartPuzzle(context context.Context, request *proto.StartPuzzleRequest) (*empty.Empty, error) {
	Logger.Printf("INFO GameService starting puzzle; game:%v", request.GameId)
	if request.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid_name")
	}
	game, err := service.getGame(request.GameId)
	if err != nil {
		return nil, err
	}
	if len(game.Players) != PlayerLimit {
		return nil, status.Errorf(codes.FailedPrecondition, "insufficient_players")
	}

	puzzleID, err := service.puzzleService.CreatePuzzle(request.Name, request.InitialConditions, game.Players)
	if err != nil {
		return nil, err
	}
	service.notify(game, &proto.GameEvent{
		Type:     proto.GameEvent_PUZZLE_STARTED,
		PuzzleId: puzzleID.String(),
	})

	return &empty.Empty{}, nil
}

// Listen allows a client to listen for game notifications
func (service GameService) Listen(request *proto.ListenGame, stream proto.GameService_ListenServer) error {
	Logger.Printf("INFO GameService listening; game:%v player:%v", request.GameId, request.PlayerId)
	if request.PlayerId == "" {
		return status.Errorf(codes.InvalidArgument, "invalid_player_id")
	}
	game, err := service.getGame(request.GameId)
	if err != nil {
		return err
	}
	var player *Player
	var ok bool
	if player, ok = game.Players[request.PlayerId]; !ok {
		return status.Errorf(codes.NotFound, "player_not_found")
	}
	// on listen, send the current player count to the client
	stream.Send(&proto.GameEvent{
		Type:  proto.GameEvent_PLAYER_COUNT_CHANGED,
		Count: int32(len(game.Players)),
	})
	// on listen (if a puzzle is running), send the puzzle identifier (recovery logic)
	if game.CurrentPuzzle != nil {
		stream.Send(&proto.GameEvent{
			Type:     proto.GameEvent_PUZZLE_STARTED,
			PuzzleId: game.CurrentPuzzle.ID.String(),
		})
	}

	player.GameChannel = make(chan *proto.GameEvent, 0)
	for {
		select {
		case event, ok := <-player.GameChannel:
			if !ok {
				return nil
			}
			err := stream.Send(event)
			if err != nil {
				Logger.Printf("WARN GameService event send failed; game:%v player:%v", game.ID, player.ID)
				go handleStreamDisconnected(&service, game, player)
				return status.Errorf(codes.DataLoss, "listener_aborted")
			}
		case <-stream.Context().Done():
			Logger.Printf("WARN GameService connection disconnected by client; game:%v player:%v", game.ID, player.ID)
			go handleStreamDisconnected(&service, game, player)
			return nil
		}
	}
}

package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"

	"github.com/gavin-potgieter/sensense-server/server/proto"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// PlayerLimit is the total number of players
	PlayerLimit = 3
)

// Player is a game player
type Player struct {
	ID string
}

// Game is a running instance of a sensense game
type Game struct {
	Code    int
	ID      uuid.UUID
	Players map[string]*Player
}

// GameService provides a running service instance
type GameService struct {
	gameCodes map[int]*Game
	games     map[uuid.UUID]*Game
}

// NewGameService creates a new GameService
func NewGameService() (*GameService, error) {
	return &GameService{
		gameCodes: make(map[int]*Game, 0),
		games:     make(map[uuid.UUID]*Game, 0),
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

func join(service *GameService, game *Game, playerID string) error {
	if _, ok := game.Players[playerID]; ok {
		return nil
	}
	if len(game.Players) >= PlayerLimit {
		return status.Errorf(codes.ResourceExhausted, "game_full")
	}
	game.Players[playerID] = &Player{ID: playerID}
	return nil
}

// Join allows a player to join the game
func (service GameService) Join(context context.Context, request *proto.JoinGameRequest) (*proto.JoinGameResponse, error) {
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
func (service GameService) Rejoin(context context.Context, request *proto.RejoinGameRequest) (*proto.RejoinGameResponse, error) {
	if request.PlayerId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid_player_id")
	}
	gameID, err := uuid.Parse(request.GameId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid_game_id")
	}
	var game *Game
	var ok bool
	if game, ok = service.games[gameID]; !ok {
		return nil, status.Errorf(codes.NotFound, "game_not_found")
	}
	err = join(&service, game, request.PlayerId)
	if err != nil {
		return nil, err
	}
	return &proto.RejoinGameResponse{}, nil
}

func deleteGame(service *GameService, gameID uuid.UUID) {
	game := service.games[gameID]
	delete(service.gameCodes, game.Code)
	delete(service.games, game.ID)
}

// Leave allows players to cleanly leave a game
func (service GameService) Leave(context context.Context, request *proto.LeaveGameRequest) (*proto.LeaveGameResponse, error) {
	if request.PlayerId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid_player_id")
	}
	gameID, err := uuid.Parse(request.GameId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid_game_id")
	}

	var game *Game
	var ok bool
	if game, ok = service.games[gameID]; !ok {
		return &proto.LeaveGameResponse{}, nil
	}

	delete(game.Players, request.PlayerId)
	if len(game.Players) == 0 {
		deleteGame(&service, game.ID)
	}
	return &proto.LeaveGameResponse{}, nil
}

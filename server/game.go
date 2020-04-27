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

const (
	// PlayerLimit is the total number of players
	PlayerLimit = 3
	// PlayerRecoveryTime is the time the game waits if a player is disconnected before gracefully cleaning up
	PlayerRecoveryTime = 10 * time.Second
)

// Player is a game player
type Player struct {
	ID      string
	Channel chan *proto.GameNotification
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

func join(service *GameService, game *Game, playerID string) error {
	if _, ok := game.Players[playerID]; ok {
		return nil
	}
	if len(game.Players) >= PlayerLimit {
		return status.Errorf(codes.ResourceExhausted, "game_full")
	}
	game.Players[playerID] = &Player{ID: playerID}
	notify(game, service, &proto.GameNotification{
		Type:  proto.GameNotification_PLAYER_COUNT_CHANGED,
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
	return &empty.Empty{}, nil
}

func deleteGame(service *GameService, gameID uuid.UUID) {
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
	gameID, err := uuid.Parse(request.GameId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid_game_id")
	}

	var game *Game
	var ok bool
	if game, ok = service.games[gameID]; !ok {
		return &empty.Empty{}, nil
	}

	var player *Player
	if player, ok = game.Players[request.PlayerId]; !ok {
		return &empty.Empty{}, nil
	}

	leave(&service, game, player)
	return &empty.Empty{}, nil
}

func notify(game *Game, service *GameService, notification *proto.GameNotification) {
	for _, player := range game.Players {
		if player.Channel != nil {
			player.Channel <- notification
		}
	}
}

func leave(service *GameService, game *Game, player *Player) {
	if player.Channel != nil {
		close(player.Channel)
		player.Channel = nil
	}
	delete(game.Players, player.ID)
	if len(game.Players) == 0 {
		deleteGame(service, game.ID)
	} else {
		notify(game, service, &proto.GameNotification{
			Type:  proto.GameNotification_PLAYER_COUNT_CHANGED,
			Count: int32(len(game.Players)),
		})
	}
}

func handleStreamDisconnected(service *GameService, game *Game, player *Player) {
	close(player.Channel)
	player.Channel = nil
	time.Sleep(PlayerRecoveryTime)
	if player.Channel != nil {
		Logger.Printf("INFO GameService recovered player; game:%v player:%v", game.ID, player.ID)
		return
	}
	Logger.Printf("WARN GameService lost player; game:%v player:%v", game.ID, player.ID)
	leave(service, game, player)
}

func (service GameService) StartPuzzle(context.Context, *proto.StartPuzzleRequest) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StartPuzzle not implemented")
}

// Listen allows a client to listen for game notifications
func (service GameService) Listen(request *proto.ListenGameRequest, stream proto.GameService_ListenServer) error {
	Logger.Printf("INFO GameService listening; game:%v player:%v", request.GameId, request.PlayerId)
	if request.PlayerId == "" {
		return status.Errorf(codes.InvalidArgument, "invalid_player_id")
	}
	gameID, err := uuid.Parse(request.GameId)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid_game_id")
	}
	var game *Game
	var ok bool
	if game, ok = service.games[gameID]; !ok {
		return status.Errorf(codes.NotFound, "game_not_found")
	}
	var player *Player
	if player, ok = game.Players[request.PlayerId]; !ok {
		return status.Errorf(codes.NotFound, "player_not_found")
	}
	player.Channel = make(chan *proto.GameNotification, 0)
	for {
		select {
		case notification, ok := <-player.Channel:
			if !ok {
				return nil
			}
			err := stream.Send(notification)
			if err != nil {
				Logger.Printf("WARN GameService notification send failed; game:%v player:%v", game.ID, player.ID)
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

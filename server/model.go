package main

import (
	"sync"
	"time"

	"github.com/gavin-potgieter/sensense-server/server/proto"
	"github.com/google/uuid"
)

const (
	// PlayerLimit is the total number of players
	PlayerLimit = 3
	// PlayerRecoveryTime is the time the game waits if a player is disconnected before gracefully cleaning up
	PlayerRecoveryTime = 10 * time.Second
	// TimerInterval is the interval that timer events are sent in seconds
	TimerInterval = 2 // 10

	// Blind is the player who can't see
	Blind = 0
	// Deaf is the player who can't hear
	Deaf = 1
	// Mute is the player who can't talk
	Mute = 2

	// InitialConditionsKey is the key for the initial conditions event
	InitialConditionsKey = "INCO"
	// RoleKey is the key for the role event
	RoleKey = "ROLE"
	// ResultKey is the key for the result of the puzzle event
	ResultKey = "RSLT"
	// NameKey is the key for the name of the puzzle event
	NameKey = "NAME"
	// TimeKey is the key for the seconds remaining event
	TimeKey = "TIME"
	// PlayerMissingKey is the key for when a player is missing (probably due to connection issues)
	PlayerMissingKey = "PMIA"
	// AllAboardKey is the key for when all players are connected to the puzzle
	AllAboardKey = "ABRD"

	// ResultAbort is the cancelled result value
	ResultAbort = "ABORT"
	// ResultLose is the failed result value
	ResultLose = "LOSE"
	// ResultWin is the success result value
	ResultWin = "WIN"
)

// Role is a sensense player role for a puzzle
type Role int

// Player is a game player
type Player struct {
	ID                string
	Role              Role
	GameChannel       chan *proto.GameEvent
	GameChannelLock   sync.RWMutex
	PuzzleChannel     chan *proto.PuzzleEvent
	PuzzleChannelLock sync.RWMutex
}

// NewPlayer creates a new role
func NewPlayer(id string, role Role) *Player {
	return &Player{
		ID:                id,
		GameChannelLock:   sync.RWMutex{},
		PuzzleChannelLock: sync.RWMutex{},
		Role:              role,
	}
}

// Game is a running instance of a sensense game
type Game struct {
	Code     int
	PuzzleID *uuid.UUID
	ID       uuid.UUID
	Lock     sync.Mutex
	Players  map[string]*Player
}

// EndPuzzleCallback is raised when a puzzle ends
type EndPuzzleCallback func()

// Puzzle is a running instance of a sensense game level
type Puzzle struct {
	EndPuzzleCallback EndPuzzleCallback
	ID                uuid.UUID
	InitialConditions string
	Name              string
	Lock              sync.Mutex
	Players           map[string]*Player
	ticker            *time.Ticker
	timeRemaining     int
}

// Paused determines if the puzzle is paused due to an MIA player
func (puzzle *Puzzle) Paused() bool {
	connectedPlayers := 0
	for _, player := range puzzle.Players {
		player.PuzzleChannelLock.RLock()
		if player.PuzzleChannel != nil {
			connectedPlayers++
		}
		player.PuzzleChannelLock.RUnlock()
	}
	return connectedPlayers != PlayerLimit
}

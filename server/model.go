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
	ID                string                  // player identifier (generated by client)
	Role              Role                    // player current role
	GameChannel       chan *proto.GameEvent   // the queue of game events for the player
	GameChannelLock   sync.RWMutex            // the lock to serialize the allocation of the game event queue
	PuzzleChannel     chan *proto.PuzzleEvent // the queue of puzzle events for the player
	PuzzleChannelLock sync.RWMutex            // the lock to serialize the allocation of the puzzle event queue
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

// Game is a running instance of a SenSense game
type Game struct {
	Code     int                // player friendly code generated by the server for the player to join the game
	PuzzleID *uuid.UUID         // the active puzzle identifier (used to ensure that only one puzzle is active)
	ID       uuid.UUID          // the game identifier generated by the server
	Lock     sync.Mutex         // the lock to serialize the game from modification
	Players  map[string]*Player // the players of the game
}

// EndPuzzleCallback is raised when a puzzle ends
type EndPuzzleCallback func()

// Puzzle is a running instance of a SenSense game level
type Puzzle struct {
	EndPuzzleCallback EndPuzzleCallback  // a callback that is raised when the game ends (used to deactivate the game puzzle)
	ID                uuid.UUID          // the puzzle identifier generated by the server
	InitialConditions string             // an unstructured initial puzzle state
	Name              string             // the name of the puzzle so players know what puzzle they're playing
	Lock              sync.Mutex         // the lock to serialize the puzzle from modification
	Players           map[string]*Player // the players of the puzzle
	ticker            *time.Ticker       // the game timer / ticker
	timeRemaining     int                // the time remaining for the puzzle (counts down automatically)
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

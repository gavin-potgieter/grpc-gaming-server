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
)

// Role is a sensense player role for a puzzle
type Role int

// Player is a game player
type Player struct {
	ID            string
	Role          Role
	GameChannel   chan *proto.GameEvent
	PuzzleChannel chan *proto.PuzzleEvent
}

// Game is a running instance of a sensense game
type Game struct {
	Code     int
	PuzzleID *uuid.UUID
	ID       uuid.UUID
	Lock     sync.Mutex
	Players  map[string]*Player
}

// Puzzle is a running instance of a game level
type Puzzle struct {
	ID                uuid.UUID
	InitialConditions string
	Name              string
	Lock              sync.Mutex
	Players           map[string]*Player
}

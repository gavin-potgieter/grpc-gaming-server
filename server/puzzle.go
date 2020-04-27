package main

import (
	"github.com/gavin-potgieter/sensense-server/server/proto"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PuzzleService provides a running service instance
type PuzzleService struct {
	puzzles map[uuid.UUID]*Puzzle
}

// NewPuzzleService creates a new puzzle service
func NewPuzzleService() (*PuzzleService, error) {
	return &PuzzleService{
		puzzles: make(map[uuid.UUID]*Puzzle),
	}, nil
}

func cycleRoles(players map[string]*Player) {
	for _, player := range players {
		player.Role = (player.Role + 1) % 3
	}
}

// CreatePuzzle creates a puzzle given the puzzle name and initial conditions, cycling the player roles automatically
func (service PuzzleService) CreatePuzzle(name string, initialConditions string, players map[string]*Player) (uuid.UUID, error) {
	puzzleID, err := uuid.NewRandom()
	if err != nil {
		return puzzleID, status.Errorf(codes.Internal, "puzzle_creation_failed")
	}

	puzzle := &Puzzle{
		ID:                puzzleID,
		InitialConditions: initialConditions,
		Name:              name,
		Players:           make(map[string]*Player, 0),
	}
	for _, player := range players {
		puzzle.Players[player.ID] = player
	}

	cycleRoles(puzzle.Players)

	service.puzzles[puzzleID] = puzzle
	return puzzle.ID, nil
}

func (service PuzzleService) Solve(proto.PuzzleService_SolveServer) error {
	return status.Errorf(codes.Unimplemented, "method Solve not implemented")
}

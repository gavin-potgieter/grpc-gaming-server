package main

import (
	"sync"
)

type Railway struct {
	GameCreatedSignal sync.Cond
	FirstPuzzleEnded  sync.Mutex
	SecondPuzzleEnded sync.Mutex
	GameEndedSignal   sync.Cond
}

func NewRailway() *Railway {
	railway := &Railway{
		GameCreatedSignal: sync.Cond{L: &sync.Mutex{}},
		FirstPuzzleEnded:  sync.Mutex{},
		SecondPuzzleEnded: sync.Mutex{},
		GameEndedSignal:   sync.Cond{L: &sync.Mutex{}},
	}
	railway.FirstPuzzleEnded.Lock()
	railway.SecondPuzzleEnded.Lock()
	return railway
}

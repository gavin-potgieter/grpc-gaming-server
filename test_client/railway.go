package main

import (
	"sync"
)

type Railway struct {
	MatchCreatedSignal sync.Cond
	GameEndedSignal    sync.Cond
}

func NewRailway() *Railway {
	railway := &Railway{
		MatchCreatedSignal: sync.Cond{L: &sync.Mutex{}},
		GameEndedSignal:    sync.Cond{L: &sync.Mutex{}},
	}
	return railway
}

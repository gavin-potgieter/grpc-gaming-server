package main

import (
	"sync"
)

type Railway struct {
	GameCreatedSignal sync.Cond
	GameEndedSignal   sync.Cond
}

func NewRailway() *Railway {
	railway := &Railway{
		GameCreatedSignal: sync.Cond{L: &sync.Mutex{}},
		GameEndedSignal:   sync.Cond{L: &sync.Mutex{}},
	}
	//railway.GameCreatedSignal.L.Lock()
	//railway.GameEndedSignal.L.Lock()
	return railway
}

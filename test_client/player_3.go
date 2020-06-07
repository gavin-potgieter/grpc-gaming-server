package main

import (
	"sync"
	"time"
)

type Player3 struct {
	Player *Player
}

func NewPlayer3(railway *Railway) (*Player3, error) {
	player, err := NewPlayer("player_3", railway)
	if err != nil {
		return nil, err
	}
	return &Player3{
		Player: player,
	}, nil
}

func (player3 *Player3) Interact(group *sync.WaitGroup) error {
	player3.Player.Railway.MatchCreatedSignal.L.Lock()
	player3.Player.Railway.MatchCreatedSignal.Wait()
	player3.Player.Railway.MatchCreatedSignal.L.Unlock()

	time.Sleep(300 * time.Millisecond)

	go func() {
		err := player3.Player.Match(group, nil)
		if err != nil {
			Logger.Printf("ERROR %v %v\n", player3.Player.PlayerID, err)
		}
	}()

	player3.Player.Railway.GameEndedSignal.L.Lock()
	player3.Player.Railway.GameEndedSignal.Wait()
	player3.Player.Railway.GameEndedSignal.L.Unlock()

	group.Done()

	return nil
}

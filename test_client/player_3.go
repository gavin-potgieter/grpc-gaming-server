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
	player3.Player.Railway.GameCreatedSignal.L.Lock()
	player3.Player.Railway.GameCreatedSignal.Wait()
	player3.Player.Railway.GameCreatedSignal.L.Unlock()

	err := player3.Player.JoinGame()
	if err != nil {
		return err
	}

	go func() {
		err := player3.Player.ListenGame(group, nil)
		if err != nil {
			Logger.Printf("ERROR %v %v\n", player3.Player.PlayerID, err)
		}
	}()

	player3.Player.Railway.FirstPuzzleEnded.Lock()

	time.Sleep(1 * time.Second)

	err = player3.Player.StartPuzzle("P2", 10)
	if err != nil {
		return err
	}

	player3.Player.Railway.SecondPuzzleEnded.Lock()

	player3.Player.Railway.GameEndedSignal.L.Lock()
	player3.Player.Railway.GameEndedSignal.Broadcast()
	player3.Player.Railway.GameEndedSignal.L.Unlock()

	err = player3.Player.Leave()
	if err != nil {
		return err
	}

	return nil
}

package main

import (
	"sync"
)

type Player3 struct {
	Player  *Player
	Railway *Railway
}

func NewPlayer3(railway *Railway) (*Player3, error) {
	player, err := NewPlayer("player_3")
	if err != nil {
		return nil, err
	}
	return &Player3{
		Player:  player,
		Railway: railway,
	}, nil
}

func (player3 *Player3) Interact(group *sync.WaitGroup) error {
	player3.Railway.GameCreatedSignal.L.Lock()
	player3.Railway.GameCreatedSignal.Wait()
	player3.Railway.GameCreatedSignal.L.Unlock()

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

	player3.Railway.GameEndedSignal.L.Lock()
	player3.Railway.GameEndedSignal.Wait()
	player3.Railway.GameEndedSignal.L.Unlock()

	return nil
}

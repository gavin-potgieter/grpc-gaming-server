package main

import (
	"sync"
)

type Player1 struct {
	Player  *Player
	Railway *Railway
}

func NewPlayer1(railway *Railway) (*Player1, error) {
	player, err := NewPlayer("player_1")
	if err != nil {
		return nil, err
	}
	return &Player1{
		Player:  player,
		Railway: railway,
	}, nil
}

func (player1 *Player1) Interact(group *sync.WaitGroup) error {
	err := player1.Player.CreateGame()
	if err != nil {
		return err
	}

	go func() {
		err := player1.Player.ListenGame(group, nil)
		if err != nil {
			Logger.Printf("ERROR %v %v\n", player1.Player.PlayerID, err)
		}
	}()

	player1.Railway.GameCreatedSignal.L.Lock()
	player1.Railway.GameCreatedSignal.Broadcast()
	player1.Railway.GameCreatedSignal.L.Unlock()

	player1.Railway.GameEndedSignal.L.Lock()
	player1.Railway.GameEndedSignal.Wait()
	player1.Railway.GameEndedSignal.L.Unlock()

	err = player1.Player.Leave()
	if err != nil {
		return err
	}
	return nil
}

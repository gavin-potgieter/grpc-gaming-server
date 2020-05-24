package main

import (
	"sync"
	"time"
)

type Player2 struct {
	Player *Player
}

func NewPlayer2(railway *Railway) (*Player2, error) {
	player, err := NewPlayer("player_2", railway)
	if err != nil {
		return nil, err
	}
	return &Player2{
		Player: player,
	}, nil
}

func (player2 *Player2) Interact(group *sync.WaitGroup) error {
	player2.Player.Railway.GameCreatedSignal.L.Lock()
	player2.Player.Railway.GameCreatedSignal.Wait()
	player2.Player.Railway.GameCreatedSignal.L.Unlock()

	err := player2.Player.JoinGame()
	if err != nil {
		return err
	}

	go func() {
		err := player2.Player.ListenGame(group, nil)
		if err != nil {
			Logger.Printf("ERROR %v %v\n", player2.Player.PlayerID, err)
		}
	}()

	err = player2.Player.DisconnectGame()
	if err != nil {
		Logger.Printf("ERROR %v %v\n", player2.Player.PlayerID, err)
	}

	time.Sleep(1 * time.Second)

	go func() {
		err := player2.Player.ListenGame(group, nil)
		if err != nil {
			Logger.Printf("ERROR %v %v\n", player2.Player.PlayerID, err)
		}
	}()

	player2.Player.Railway.GameEndedSignal.L.Lock()
	player2.Player.Railway.GameEndedSignal.Wait()
	player2.Player.Railway.GameEndedSignal.L.Unlock()

	err = player2.Player.Leave()
	if err != nil {
		return err
	}

	return nil
}

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
	player2.Player.Railway.MatchCreatedSignal.L.Lock()
	player2.Player.Railway.MatchCreatedSignal.Wait()
	player2.Player.Railway.MatchCreatedSignal.L.Unlock()

	go func() {
		err := player2.Player.Match(group, nil)
		if err != nil {
			Logger.Printf("ERROR %v %v\n", player2.Player.PlayerID, err)
		}
		time.Sleep(200 * time.Millisecond)
		Logger.Printf("INFO rejoining %v\n", player2.Player.PlayerID)
		player2.Player.connectMatch()
		err = player2.Player.Match(group, nil)
		if err != nil {
			Logger.Printf("ERROR %v %v\n", player2.Player.PlayerID, err)
		}
	}()

	time.Sleep(200 * time.Millisecond)

	Logger.Printf("INFO disconnecting %v\n", player2.Player.PlayerID)
	err := player2.Player.DisconnectMatch()
	if err != nil {
		Logger.Printf("ERROR %v %v\n", player2.Player.PlayerID, err)
	}

	player2.Player.Railway.GameEndedSignal.L.Lock()
	player2.Player.Railway.GameEndedSignal.Wait()
	player2.Player.Railway.GameEndedSignal.L.Unlock()

	group.Done()

	return nil
}

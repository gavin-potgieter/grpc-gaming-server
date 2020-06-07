package main

import (
	"sync"

	"github.com/gavin-potgieter/sensense-server/test_client/proto"
)

type Player1 struct {
	Player *Player
}

func NewPlayer1(railway *Railway) (*Player1, error) {
	player, err := NewPlayer("player_1", railway)
	if err != nil {
		return nil, err
	}
	return &Player1{
		Player: player,
	}, nil
}

func (player1 *Player1) Interact(group *sync.WaitGroup) error {
	flag := false
	go func() {
		err := player1.Player.Match(group, func(e *proto.MatchEvent) error {
			if !flag {
				player1.Player.Railway.MatchCreatedSignal.L.Lock()
				player1.Player.Railway.MatchCreatedSignal.Broadcast()
				player1.Player.Railway.MatchCreatedSignal.L.Unlock()
				flag = true
			}
			return nil
		})
		if err != nil {
			Logger.Printf("ERROR %v %v\n", player1.Player.PlayerID, err)
		}
	}()

	player1.Player.Railway.GameEndedSignal.L.Lock()
	player1.Player.Railway.GameEndedSignal.Wait()
	player1.Player.Railway.GameEndedSignal.L.Unlock()

	return nil
}

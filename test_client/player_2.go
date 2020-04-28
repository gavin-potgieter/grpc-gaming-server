package main

import (
	"sync"
	"time"

	"github.com/gavin-potgieter/sensense-server/test_client/proto"
)

type Player2 struct {
	Player  *Player
	Railway *Railway
}

func NewPlayer2(railway *Railway) (*Player2, error) {
	player, err := NewPlayer("player_2")
	if err != nil {
		return nil, err
	}
	return &Player2{
		Player:  player,
		Railway: railway,
	}, nil
}

func (player2 *Player2) callback(event *proto.GameEvent) error {
	switch event.Type {
	case proto.GameEvent_PLAYER_COUNT_CHANGED:
		if event.Count == 3 {
			err := player2.Player.StartPuzzle()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (player2 *Player2) Interact(group *sync.WaitGroup) error {
	player2.Railway.GameCreatedSignal.L.Lock()
	player2.Railway.GameCreatedSignal.Wait()
	player2.Railway.GameCreatedSignal.L.Unlock()

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

	time.Sleep(10 * time.Millisecond)

	err = player2.Player.DisconnectGame()
	if err != nil {
		Logger.Printf("ERROR %v %v\n", player2.Player.PlayerID, err)
	}

	err = player2.Player.RejoinGame()
	if err != nil {
		return err
	}

	go func() {
		err := player2.Player.ListenGame(group, player2.callback)
		if err != nil {
			Logger.Printf("ERROR %v %v\n", player2.Player.PlayerID, err)
		}
	}()

	player2.Railway.GameEndedSignal.L.Lock()
	player2.Railway.GameEndedSignal.Broadcast()
	player2.Railway.GameEndedSignal.L.Unlock()

	return nil
}

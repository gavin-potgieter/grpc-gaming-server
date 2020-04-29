package main

import (
	"sync"
	"time"

	"github.com/gavin-potgieter/sensense-server/test_client/proto"
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

func (player2 *Player2) gameCallback(event *proto.GameEvent) error {
	switch event.Type {
	case proto.GameEvent_PLAYER_COUNT_CHANGED:
		if event.Count == 3 {
			err := player2.Player.StartPuzzle("P1", 15)
			if err != nil {
				return err
			}
		}
	}
	return nil
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
		err := player2.Player.ListenGame(group, player2.gameCallback)
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

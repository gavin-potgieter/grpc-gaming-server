package main

import (
	"log"
	"os"
	"path"
	"sync"
	"time"
)

var (
	_, filename = path.Split(os.Args[0])
	// Logger is the default logger
	Logger = log.New(os.Stdout, filename+" ", log.LstdFlags)
	// GameCode the game code
	GameCode string
	PuzzleID string
)

func main() {
	var group sync.WaitGroup
	group.Add(3)

	railway := NewRailway()

	player1, err := NewPlayer1(railway)
	if err != nil {
		Logger.Printf("ERROR %v %v", player1.Player.PlayerID, err)
	}
	player2, err := NewPlayer2(railway)
	if err != nil {
		Logger.Printf("ERROR %v %v", player1.Player.PlayerID, err)
	}
	player3, err := NewPlayer3(railway)
	if err != nil {
		Logger.Printf("ERROR %v %v", player1.Player.PlayerID, err)
	}

	go func() {
		err := player1.Interact(&group)
		if err != nil {
			Logger.Printf("ERROR %v %v", player1.Player.PlayerID, err)
		}
	}()
	go func() {
		err := player2.Interact(&group)
		if err != nil {
			Logger.Printf("ERROR %v %v", player2.Player.PlayerID, err)
		}
	}()
	go func() {
		err := player3.Interact(&group)
		if err != nil {
			Logger.Printf("ERROR %v %v", player3.Player.PlayerID, err)
		}
	}()

	group.Wait()

	time.Sleep(500 * time.Millisecond)
}

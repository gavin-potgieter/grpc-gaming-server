package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gavin-potgieter/sensense-server/test_client/proto"
	"google.golang.org/grpc"
)

func listen(gameService proto.GameServiceClient, gameID string, playerID string) {
	stream, err := gameService.Listen(context.Background(), &proto.ListenGameRequest{
		GameId:   gameID,
		PlayerId: playerID,
	})
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	for {
		notification, err := stream.Recv()
		if err != nil {
			fmt.Printf("%v\n", err)
			return
		}
		fmt.Printf("%+v\n", notification)
	}
}

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(-1)
	}
	defer conn.Close()

	gameService := proto.NewGameServiceClient(conn)
	response1, err := gameService.Create(context.Background(), &proto.CreateGameRequest{
		PlayerId: "player_1",
	})
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(-1)
	}
	fmt.Printf("Created %+v\n", response1)

	go listen(gameService, response1.GameId, "player_1")

	time.Sleep(1 * time.Second)

	response2, err := gameService.Join(context.Background(), &proto.JoinGameRequest{
		PlayerId: "player_2",
		GameCode: response1.GameCode,
	})
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(-1)
	}
	fmt.Printf("Joined %+v\n", response2)

	go listen(gameService, response1.GameId, "player_2")

	time.Sleep(1 * time.Second)

	response3, err := gameService.Rejoin(context.Background(), &proto.RejoinGameRequest{
		PlayerId: "player_2",
		GameId:   response2.GameId,
	})
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(-1)
	}
	fmt.Printf("Rejoined %+v\n", response3)

	response4, err := gameService.Leave(context.Background(), &proto.LeaveGameRequest{
		PlayerId: "player_2",
		GameId:   response2.GameId,
	})
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(-1)
	}
	fmt.Printf("Left %+v\n", response4)

	time.Sleep(5 * time.Second)
}

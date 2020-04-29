package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path"

	"github.com/gavin-potgieter/sensense-server/server/proto"
	"google.golang.org/grpc"
)

var (
	_, filename = path.Split(os.Args[0])
	// Logger is the default logger
	Logger = log.New(os.Stdout, filename+" ", log.LstdFlags)
)

func serve() {
	addr := fmt.Sprintf(":%d", 50051)
	conn, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Cannot listen to address %s", addr)
	}
	puzzleService, err := NewPuzzleService()
	if err != nil {
		log.Fatalf("%v", err)
	}
	gameService, err := NewGameService(puzzleService)
	if err != nil {
		log.Fatalf("%v", err)
	}
	server := grpc.NewServer()
	proto.RegisterGameServiceServer(server, gameService)
	proto.RegisterPuzzleServiceServer(server, puzzleService)
	log.Printf("Starting server\n")
	if err := server.Serve(conn); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func main() {
	serve()
}

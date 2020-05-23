package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/akamensky/argparse"
	"github.com/gavin-potgieter/sensense-server/server/proto"
	"google.golang.org/grpc"
)

var (
	// Logger is the default logger
	Logger = log.New(os.Stdout, "", log.LstdFlags)
)

func serve() {
	parser := argparse.NewParser("print", "SenSense Server (c) 2020")
	port := parser.String("p", "port", &argparse.Options{Required: false, Help: "the port to run on", Default: "8080"})
	err := parser.Parse(os.Args)
	if err != nil {
		log.Fatal(parser.Usage(err))
		return
	}
	addr := fmt.Sprintf(":%v", *port)

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
	log.Printf("Starting server %v\n", addr)
	if err := server.Serve(conn); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func main() {
	serve()
}

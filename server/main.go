package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"server/game"
	"server/match"
	"server/proto"

	"github.com/akamensky/argparse"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

var (
	// Logger is the default logger
	Logger = log.New(os.Stdout, "", log.LstdFlags)
)

func serve() {
	parser := argparse.NewParser("print", "SenSense Server (c) 2020")
	port := parser.String("p", "port", &argparse.Options{Required: false, Help: "the port to run on", Default: "8080"})
	gameServerURL := parser.String("g", "game-server-url", &argparse.Options{Required: false, Help: "the URL of the game server", Default: "localhost:8080"})
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
	gameService, err := game.NewService()
	if err != nil {
		log.Fatalf("%v", err)
	}
	matchService, err := match.NewService(*gameServerURL)
	if err != nil {
		log.Fatalf("%v", err)
	}

	policy := keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second,
		PermitWithoutStream: false,
	}

	keepalive := keepalive.ServerParameters{
		MaxConnectionIdle: 5 * time.Minute,
		Time:              1 * time.Second,
		Timeout:           2 * time.Second,
	}

	server := grpc.NewServer(grpc.KeepaliveEnforcementPolicy(policy), grpc.KeepaliveParams(keepalive))
	proto.RegisterMatchServiceServer(server, matchService)
	proto.RegisterGameServiceServer(server, gameService)
	log.Printf("Starting server %v\n", addr)
	if err := server.Serve(conn); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func main() {
	serve()
}

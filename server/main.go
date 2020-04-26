package main

import (
	"fmt"
	"log"
	"net"

	"github.com/gavin-potgieter/sensense-server/server/proto"
	"google.golang.org/grpc"
)

func serve() {
	addr := fmt.Sprintf(":%d", 50051)
	conn, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Cannot listen to address %s", addr)
	}

	service, err := NewGameService()
	if err != nil {
		log.Fatalf("%v", err)
	}
	server := grpc.NewServer()
	proto.RegisterGameServiceServer(server, service)
	log.Printf("Starting server\n")
	if err := server.Serve(conn); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func main() {
	serve()
}

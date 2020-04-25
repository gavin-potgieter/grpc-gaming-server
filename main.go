package main

import (
	context "context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gavin-potgieter/sensense-server/proto"
	"google.golang.org/grpc"
)

type MyGameServiceServer struct {
}

func (server MyGameServiceServer) StartGame(context.Context, *proto.StartGameRequest) (*proto.StartGameResponse, error) {
	time.Sleep(100 * time.Second)
	return nil, fmt.Errorf("special_error_code_string")
	// return &StartGameResponse{
	// 	GameCode: "123456",
	// }, nil
}

func (server MyGameServiceServer) JoinGame(context.Context, *proto.JoinGameRequest) (*proto.JoinGameResponse, error) {
	return nil, nil
}

func (server MyGameServiceServer) LeaveGame(context.Context, *proto.LeaveGameRequest) (*proto.LeaveGameResponse, error) {
	return nil, nil
}

func Serve() {
	addr := fmt.Sprintf(":%d", 50051)
	conn, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Cannot listen to address %s", addr)
	}

	service := MyGameServiceServer{}
	server := grpc.NewServer()
	proto.RegisterGameServiceServer(server, service)
	log.Printf("Starting server\n")
	if err := server.Serve(conn); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func main() {
	Serve()
}

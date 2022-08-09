# grpc-gaming-server
A gaming server implemented in gRPC. Contains two services:
* Match - for matching players using a code
* Game - for broadcasting messages between players

This server was build to replace UNET in [Sensense](https://apps.apple.com/us/app/sensense/id1448612001) and to resolve network stability issues without kicking players out the game. There is no game logic in the server. 

## Generating server stub

```bash
protoc --go_out=server/proto grpc-gaming-server.proto
protoc --go-grpc_out=server/proto grpc-gaming-server.proto
```

## Generating test client stub

```bash
protoc --go_out=test_client/proto grpc-gaming-server.proto
protoc --go-grpc_out=test_client/proto grpc-gaming-server.proto
```
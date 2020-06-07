# grpc-gaming-server
A gaming server implemented in gRPC. Contains two services:
* Match - for matching players using a code
* Game - for broadcasting messages between players

## Generating server stub

```bash
protoc grpc-gaming-server.proto --go_out=plugins=grpc:server/proto
```

## Generating test client stub

```bash
protoc grpc-gaming-server.proto --go_out=plugins=grpc:test_client/proto
```

## Generating client stub for C# clients

```bash
tools_2_26_0\protoc.exe -I client sensense.proto --csharp_out=client --grpc_out=client --plugin=protoc-gen-grpc=tools_2_26_0\grpc_csharp_plugin.exe --proto_path=.
```

## Building the server

```bash
cd server
./build
```

## Building the test client

```bash
cd test_client
go build
./test_client
```
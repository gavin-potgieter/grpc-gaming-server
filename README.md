# sensense-server
The server for the Sensense game

## Generating server stub

```bash
protoc sensense.proto --go_out=plugins=grpc:server/proto
```

## Generating test client stub

```bash
protoc sensense.proto --go_out=plugins=grpc:test_client/proto
```

## Building the Server

```bash
cd server
./build
```

## Testing

```bash
grpcurl -proto sensense.proto -plaintext -d '{"phone_id": "12345"}' localhost:50051 GameService/StartGame
```
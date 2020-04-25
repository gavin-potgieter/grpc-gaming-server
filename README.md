# sensense-server
The server for the Sensense game

## Building

```bash
protoc sensense.proto --go_out=plugins=grpc:proto
go build
```

## Testing

```bash
grpcurl -proto sensense.proto -plaintext -d '{"phone_id": "12345"}' localhost:50051 GameService/StartGame
```
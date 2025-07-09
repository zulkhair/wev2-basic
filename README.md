# Basic Game Example

This is a port of the starter game template to cardinal V2.

## Prerequisites

- Go 1.24 or later
- NATS server with JetStream enabled
- Docker and Kubernetes (for running with Tilt)

## Running the Game

1. Start the development environment using Tilt:

```bash
tilt up
```

2. Run the game server:

```bash
go run main.go
```

## Testing the Game

The example includes a test client that can send various game commands. The client automatically creates necessary JetStream streams and uses the correct message format.

### Available Commands

1. Create a new player:
```bash
go run cmd/client/main.go create-player <nickname>

# Example:
go run cmd/client/main.go create-player player1
```

2. Attack a player:
```bash
go run cmd/client/main.go attack-player <target> <damage>

# Example:
go run cmd/client/main.go attack-player player1 20
```

3. View game state (debug log):
```bash
go run cmd/client/main.go debug-log
```
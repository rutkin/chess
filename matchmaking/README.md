# Matchmaking Service

A Go microservice for chess matchmaking using Redis sorted sets and Lua to pair players by rating, time control, and region. Exposes REST APIs compatible with the spec.

## Features
- POST /v1/matchmaking/enqueue
- DELETE /v1/matchmaking/enqueue/{ticket_id}
- GET /v1/matchmaking/status/{ticket_id}
- Background matcher pairs players and calls Coordinator stub
- Prometheus metrics and basic healthz/readyz

## Config
Environment variables (12-factor):
- MM_HTTP_ADDR=:8080
- MM_REDIS_ADDR=localhost:6379
- MM_REDIS_DB=0
- MM_REDIS_PASSWORD=
- MM_MATCH_INTERVAL_MS=50
- MM_PAIR_RATING_DELTA=150 (initial), expands over wait time
- MM_REGION=strict|any (strict by default)
- MM_COORDINATOR_URL=http://localhost:8090

## Run
- Redis required locally.

```bash
# macOS: start redis with docker
docker run --rm -p 6379:6379 redis:7

# build and run
go mod tidy
go run ./cmd/matchmaking
```

## Notes
- This is a minimal but production-leaning starter. Coordinator call is mocked to return game_id and resume_token.
- Replace coordinator client with your real endpoint and auth.

run:
	MM_HTTP_ADDR=:8080 MM_REDIS_ADDR=localhost:6379 go run ./cmd/matchmaking

build:
	GOOS=darwin GOARCH=arm64 go build -o bin/matchmaking ./cmd/matchmaking

tidy:
	go mod tidy

# Build stage
FROM golang:1.22 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /matchmaking ./cmd/matchmaking

# Runtime stage
FROM scratch
COPY --from=builder /matchmaking /matchmaking
ENV MM_HTTP_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/matchmaking"]

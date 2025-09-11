package coordinator

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/example/chess/matchmaking/internal/config"
)

type Client struct {
	baseURL string
	c       *http.Client
}

type CreateGameRequest struct {
	Players     []string
	Mode        string
	TimeControl string
	RatingType  string
	Region      string
}

func NewClient(cfg config.Config) *Client {
	return &Client{baseURL: cfg.CoordinatorURL, c: &http.Client{Timeout: 800 * time.Millisecond}}
}

func (cl *Client) CreateGame(ctx context.Context, req CreateGameRequest) (string, error) {
	// TODO: replace with real HTTP call. For now, simulate success quickly.
	_ = ctx
	if len(req.Players) != 2 {
		return "", fmt.Errorf("need 2 players")
	}
	return "g_" + uuid.NewString(), nil
}

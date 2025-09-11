package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/example/chess/matchmaking/internal/config"
	"github.com/example/chess/matchmaking/internal/metrics"
)

type Store struct {
	rdb *redis.Client
	log *zap.Logger
	cfg config.Config
}

type Ticket struct {
	TicketID    string    `json:"ticket_id"`
	UserID      string    `json:"user_id"`
	Region      string    `json:"region"`
	Mode        string    `json:"mode"`
	TimeControl string    `json:"time_control"`
	RatingType  string    `json:"rating_type"`
	Rating      int       `json:"rating"`
	EnqueuedAt  time.Time `json:"enqueued_at"`
}

type TicketStatus struct {
	Status string  `json:"status"`
	GameID *string `json:"game_id,omitempty"`
}

// Keys layout
// mm:queue:{region}:{rating_type}:{time_control} -> ZSET(score=rating, member=ticketID)
// mm:ticket:{ticketID} -> HASH(user_id, region, ...)
// mm:status:{ticketID} -> STRING queued|matched|cancelled
// mm:match:{ticketID} -> STRING game_id

func MustRedis(cfg config.Config) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		panic(err)
	}
	return rdb
}

func NewStore(rdb *redis.Client, log *zap.Logger) *Store {
	return &Store{rdb: rdb, log: log}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.rdb.Ping(ctx).Err()
}

func (s *Store) Enqueue(ctx context.Context, t Ticket) (string, error) {
	if t.TicketID == "" {
		t.TicketID = "t_" + uuid.NewString()
	}
	t.EnqueuedAt = time.Now().UTC()

	keyQ := s.queueKey(t.Region, t.RatingType, t.TimeControl)
	keyT := s.ticketKey(t.TicketID)
	keyS := s.statusKey(t.TicketID)

	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, keyT, map[string]interface{}{
		"user_id":      t.UserID,
		"region":       t.Region,
		"mode":         t.Mode,
		"time_control": t.TimeControl,
		"rating_type":  t.RatingType,
		"rating":       t.Rating,
		"enqueued_at":  t.EnqueuedAt.Unix(),
	})
	pipe.Set(ctx, keyS, "queued", 0)
	pipe.ZAdd(ctx, keyQ, redis.Z{Score: float64(t.Rating), Member: t.TicketID})
	_, err := pipe.Exec(ctx)
	if err == nil {
		// update depth
		size, _ := s.rdb.ZCard(ctx, keyQ).Result()
		metrics.QueueDepthGauge.WithLabelValues(t.Region, t.RatingType, t.TimeControl).Set(float64(size))
	}
	return t.TicketID, err
}

func (s *Store) Cancel(ctx context.Context, ticketID string) error {
	// We need to read ticket to know keys
	h, err := s.rdb.HGetAll(ctx, s.ticketKey(ticketID)).Result()
	if err != nil {
		return err
	}
	if len(h) == 0 {
		return redis.Nil
	}
	keyQ := s.queueKey(h["region"], h["rating_type"], h["time_control"])
	pipe := s.rdb.TxPipeline()
	pipe.ZRem(ctx, keyQ, ticketID)
	pipe.Set(ctx, s.statusKey(ticketID), "cancelled", 0)
	pipe.Del(ctx, s.ticketKey(ticketID))
	_, err = pipe.Exec(ctx)
	if err == nil {
		size, _ := s.rdb.ZCard(ctx, keyQ).Result()
		metrics.QueueDepthGauge.WithLabelValues(h["region"], h["rating_type"], h["time_control"]).Set(float64(size))
	}
	return err
}

func (s *Store) GetStatus(ctx context.Context, ticketID string) (TicketStatus, error) {
	st, err := s.rdb.Get(ctx, s.statusKey(ticketID)).Result()
	if err != nil {
		return TicketStatus{}, err
	}
	var gameID *string
	if st == "matched" {
		gid, err2 := s.rdb.Get(ctx, s.matchKey(ticketID)).Result()
		if err2 == nil {
			gameID = &gid
		}
	}
	return TicketStatus{Status: st, GameID: gameID}, nil
}

// MatchCandidates finds two closest by rating in the same bucket.
// A simplified approach: take top N waiting, pair greedy by nearest rating.
func (s *Store) MatchCandidates(ctx context.Context, region, ratingType, timeControl string, max int64) ([]Ticket, error) {
	keyQ := s.queueKey(region, ratingType, timeControl)
	ids, err := s.rdb.ZRange(ctx, keyQ, 0, max-1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]Ticket, 0, len(ids))
	for _, id := range ids {
		m, err := s.rdb.HGetAll(ctx, s.ticketKey(id)).Result()
		if err != nil {
			return nil, err
		}
		if len(m) == 0 {
			continue
		}
		rating := 0
		fmt.Sscanf(m["rating"], "%d", &rating)
		out = append(out, Ticket{
			TicketID:    id,
			UserID:      m["user_id"],
			Region:      m["region"],
			Mode:        m["mode"],
			TimeControl: m["time_control"],
			RatingType:  m["rating_type"],
			Rating:      rating,
		})
	}
	return out, nil
}

func (s *Store) MarkMatched(ctx context.Context, a, b Ticket, gameID string) error {
	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, s.statusKey(a.TicketID), "matched", 0)
	pipe.Set(ctx, s.statusKey(b.TicketID), "matched", 0)
	pipe.Set(ctx, s.matchKey(a.TicketID), gameID, 0)
	pipe.Set(ctx, s.matchKey(b.TicketID), gameID, 0)
	pipe.ZRem(ctx, s.queueKey(a.Region, a.RatingType, a.TimeControl), a.TicketID, b.TicketID)
	_, err := pipe.Exec(ctx)
	if err == nil {
		keyQ := s.queueKey(a.Region, a.RatingType, a.TimeControl)
		size, _ := s.rdb.ZCard(ctx, keyQ).Result()
		metrics.QueueDepthGauge.WithLabelValues(a.Region, a.RatingType, a.TimeControl).Set(float64(size))
	}
	return err
}

type Bucket struct{ Region, RatingType, TimeControl string }

// ListBuckets scans Redis for mm:queue:* keys and returns distinct buckets.
func (s *Store) ListBuckets(ctx context.Context) ([]Bucket, error) {
	var (
		cursor uint64
		out    = make([]Bucket, 0)
		seen   = make(map[string]struct{})
	)
	for {
		keys, cur, err := s.rdb.Scan(ctx, cursor, "mm:queue:*", 200).Result()
		if err != nil {
			return nil, err
		}
		cursor = cur
		for _, k := range keys {
			// k format: mm:queue:{region}:{rating_type}:{time_control}
			parts := strings.SplitN(k, ":", 5)
			if len(parts) == 5 {
				region, rt, tc := parts[2], parts[3], parts[4]
				key := region + "|" + rt + "|" + tc
				if _, ok := seen[key]; !ok {
					seen[key] = struct{}{}
					out = append(out, Bucket{Region: region, RatingType: rt, TimeControl: tc})
				}
			}
		}
		if cursor == 0 {
			break
		}
	}
	return out, nil
}

func (s *Store) queueKey(region, ratingType, tc string) string {
	return fmt.Sprintf("mm:queue:%s:%s:%s", region, ratingType, tc)
}
func (s *Store) ticketKey(ticketID string) string { return "mm:ticket:" + ticketID }
func (s *Store) statusKey(ticketID string) string { return "mm:status:" + ticketID }
func (s *Store) matchKey(ticketID string) string  { return "mm:match:" + ticketID }

var ErrNoPair = errors.New("no pair")

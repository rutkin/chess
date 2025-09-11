package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr         string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	MatchInterval    time.Duration
	PairRatingDelta  int
	RegionModeStrict bool
	CoordinatorURL   string
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() Config {
	addr := getenv("MM_HTTP_ADDR", ":8080")
	redisAddr := getenv("MM_REDIS_ADDR", "localhost:6379")
	redisPass := getenv("MM_REDIS_PASSWORD", "")
	dbStr := getenv("MM_REDIS_DB", "0")
	db, _ := strconv.Atoi(dbStr)
	intervalMs, _ := strconv.Atoi(getenv("MM_MATCH_INTERVAL_MS", "50"))
	delta, _ := strconv.Atoi(getenv("MM_PAIR_RATING_DELTA", "150"))
	strict := getenv("MM_REGION", "strict") != "any"
	coordURL := getenv("MM_COORDINATOR_URL", "http://localhost:8090")

	return Config{
		HTTPAddr:         addr,
		RedisAddr:        redisAddr,
		RedisPassword:    redisPass,
		RedisDB:          db,
		MatchInterval:    time.Duration(intervalMs) * time.Millisecond,
		PairRatingDelta:  delta,
		RegionModeStrict: strict,
		CoordinatorURL:   coordURL,
	}
}

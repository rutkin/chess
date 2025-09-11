package matcher

import (
	"context"
	"math"
	"sort"
	"time"

	"go.uber.org/zap"

	"github.com/example/chess/matchmaking/internal/config"
	"github.com/example/chess/matchmaking/internal/coordinator"
	"github.com/example/chess/matchmaking/internal/metrics"
	"github.com/example/chess/matchmaking/internal/storage"
)

type Matcher struct {
	store *storage.Store
	log   *zap.Logger
	cfg   config.Config
	coord *coordinator.Client
}

func NewMatcher(s *storage.Store, log *zap.Logger, cfg config.Config) *Matcher {
	return &Matcher{store: s, log: log, cfg: cfg, coord: coordinator.NewClient(cfg)}
}

func (m *Matcher) Run(ctx context.Context) {
	t := time.NewTicker(m.cfg.MatchInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.tick(ctx)
		}
	}
}

func (m *Matcher) tick(ctx context.Context) {
	buckets, err := m.store.ListBuckets(ctx)
	if err != nil {
		m.log.Warn("list buckets", zap.Error(err))
		return
	}
	for _, b := range buckets {
		cands, err := m.store.MatchCandidates(ctx, b.Region, b.RatingType, b.TimeControl, 200)
		if err != nil || len(cands) < 2 {
			continue
		}
		sort.Slice(cands, func(i, j int) bool { return cands[i].Rating < cands[j].Rating })
		used := make([]bool, len(cands))
		for i := 0; i < len(cands)-1; i++ {
			if used[i] {
				continue
			}
			bestJ := -1
			bestDiff := math.MaxInt
			for j := i + 1; j < len(cands); j++ {
				if used[j] {
					continue
				}
				d := abs(cands[i].Rating - cands[j].Rating)
				if d < bestDiff && d <= m.cfg.PairRatingDelta {
					bestDiff = d
					bestJ = j
				}
			}
			if bestJ != -1 {
				// create a game via coordinator
				gid, err := m.coord.CreateGame(ctx, coordinator.CreateGameRequest{
					Players:     []string{cands[i].UserID, cands[bestJ].UserID},
					Mode:        cands[i].Mode,
					TimeControl: cands[i].TimeControl,
					RatingType:  cands[i].RatingType,
					Region:      cands[i].Region,
				})
				if err != nil {
					m.log.Warn("coordinator create game failed, falling back", zap.Error(err))
					gid = createGameID(cands[i], cands[bestJ])
				}
				if err := m.store.MarkMatched(ctx, cands[i], cands[bestJ], gid); err != nil {
					m.log.Warn("mark matched", zap.Error(err))
					continue
				}
				metrics.MatchesTotal.WithLabelValues(cands[i].Region, cands[i].RatingType, cands[i].TimeControl).Inc()
				used[i] = true
				used[bestJ] = true
			}
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func createGameID(a, b storage.Ticket) string {
	return "g_" + a.TicketID + "_" + b.TicketID
}

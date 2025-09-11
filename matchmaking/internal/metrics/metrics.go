package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "matchmaking_http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"route", "method", "status"})

	MatchesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "matchmaking_matches_total",
		Help: "Total matches created",
	}, []string{"region", "rating_type", "time_control"})

	QueueDepthGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "matchmaking_queue_depth",
		Help: "Current queue depth per bucket",
	}, []string{"region", "rating_type", "time_control"})
)

package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/example/chess/matchmaking/internal/config"
	"github.com/example/chess/matchmaking/internal/storage"
)

type enqueueReq struct {
	Mode        string `json:"mode"`
	TimeControl string `json:"time_control"`
	RatingType  string `json:"rating_type"`
	Region      string `json:"region"`
}

type enqueueResp struct {
	TicketID         string `json:"ticket_id"`
	EstimatedWaitSec int    `json:"estimated_wait_sec"`
}

type statusResp struct {
	Status string  `json:"status"`
	GameID *string `json:"game_id,omitempty"`
}

func RegisterRoutes(r chi.Router, store *storage.Store, log *zap.Logger, cfg config.Config) {
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		if err := store.Ping(req.Context()); err != nil {
			http.Error(w, "redis down", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(200)
	})

	r.Route("/v1/matchmaking", func(r chi.Router) {
		r.Post("/enqueue", func(w http.ResponseWriter, req *http.Request) {
			var in enqueueReq
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			// TODO: auth, userID from token. For now, use header X-User-ID and X-Rating
			userID := req.Header.Get("X-User-ID")
			if userID == "" {
				http.Error(w, "missing user", http.StatusUnauthorized)
				return
			}
			rating, _ := strconv.Atoi(req.Header.Get("X-Rating"))
			t := storage.Ticket{UserID: userID, Region: in.Region, Mode: in.Mode, TimeControl: in.TimeControl, RatingType: in.RatingType, Rating: rating}
			id, err := store.Enqueue(req.Context(), t)
			if err != nil {
				log.Error("enqueue", zap.Error(err))
				http.Error(w, "enqueue failed", 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(enqueueResp{TicketID: id, EstimatedWaitSec: 8})
		})

		r.Delete("/enqueue/{ticket_id}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "ticket_id")
			if err := store.Cancel(req.Context(), id); err != nil {
				w.WriteHeader(204) // idempotent
				return
			}
			w.WriteHeader(204)
		})

		r.Get("/status/{ticket_id}", func(w http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "ticket_id")
			st, err := store.GetStatus(req.Context(), id)
			if err != nil {
				http.Error(w, "not found", 404)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(statusResp{Status: st.Status, GameID: st.GameID})
		})
	})
}

package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"mygrep/internal/domain"
	"mygrep/internal/usecase"
)

type Server struct {
	worker        *usecase.Worker
	id            string
	processBudget time.Duration
}

func NewServer(id string, w *usecase.Worker, processBudget time.Duration) *Server {
	if processBudget <= 0 {
		processBudget = 30 * time.Second
	}
	return &Server{worker: w, id: id, processBudget: processBudget}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/process", s.handleProcess)
	return mux
}

func (s *Server) handleProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, "", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.processBudget)
	defer cancel()

	chunk := domain.Chunk{
		ID: req.ChunkID, Source: req.Source,
		StartLine: req.StartLine, Lines: req.Lines,
	}
	matches, err := s.worker.Process(ctx, chunk, req.Pattern)
	if err != nil {
		writeErr(w, req.ChunkID, err)
		return
	}

	resp := ProcessResponse{ChunkID: req.ChunkID}
	for _, m := range matches {
		resp.Matches = append(resp.Matches, toDTO(m))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func writeErr(w http.ResponseWriter, id string, err error) {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		w.WriteHeader(http.StatusRequestTimeout)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ProcessResponse{ChunkID: id, Error: err.Error()})
}

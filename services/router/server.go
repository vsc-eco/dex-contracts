package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

// Server provides HTTP API for DEX routing
type Server struct {
	router *Service
	http   *http.Server
}

// NewServer creates a new HTTP server for the router service
func NewServer(svc *Service, port string) *Server {
	s := &Server{
		router: svc,
	}

	r := mux.NewRouter()

	// Route computation endpoint
	r.HandleFunc("/api/v1/route", s.handleComputeRoute).Methods("POST")

	// Instruction-based swap endpoint
	r.HandleFunc("/api/v1/instruction", s.handleExecuteInstruction).Methods("POST")

	// Health check
	r.HandleFunc("/health", s.handleHealth).Methods("GET")

	s.http = &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.http.ListenAndServe()
}

// Stop stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

// handleComputeRoute handles route computation requests
func (s *Server) handleComputeRoute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FromAsset   string `json:"fromAsset"`
		ToAsset     string `json:"toAsset"`
		Amount      int64  `json:"amount"`
		MinOut      int64  `json:"minOut,omitempty"`
		SlippageBps uint64 `json:"slippageBps,omitempty"`
		Sender      string `json:"sender,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.SlippageBps == 0 {
		req.SlippageBps = 50 // 0.5% default slippage
	}

	params := SwapParams{
		AssetIn:      req.FromAsset,
		AssetOut:     req.ToAsset,
		AmountIn:     req.Amount,
		MinAmountOut: req.MinOut,
		MaxSlippage:  req.SlippageBps,
		Sender:       req.Sender,
	}

	result, err := s.router.ComputeRoute(r.Context(), params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleExecuteInstruction handles instruction-based swap requests (instruction-schema.md)
// Accepts instruction as JSON object: {"instruction": {"type":"swap",...}, "amountIn": 1000000}
func (s *Server) handleExecuteInstruction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Instruction json.RawMessage `json:"instruction"`
		AmountIn    int64           `json:"amountIn"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Empty, null, or whitespace-only instruction
	trimmed := bytes.TrimSpace(req.Instruction)
	if len(trimmed) == 0 || string(trimmed) == `""` || string(trimmed) == `null` {
		http.Error(w, "instruction is required", http.StatusBadRequest)
		return
	}

	if req.AmountIn <= 0 {
		http.Error(w, "amountIn must be greater than 0", http.StatusBadRequest)
		return
	}

	// Parse and convert instruction to SwapParams (instruction is raw JSON bytes)
	params, err := ParseAndConvertInstruction([]byte(req.Instruction), req.AmountIn)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to process instruction: %v", err), http.StatusBadRequest)
		return
	}

	// Execute the swap
	result, err := s.router.ComputeRoute(r.Context(), *params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleHealth provides health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"service": "dex-router",
	})
}

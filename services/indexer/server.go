package indexer

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// Server provides HTTP API for indexer read models
type Server struct {
	indexer *Service
	http    *http.Server
}

// NewServer creates a new HTTP server for the indexer
func NewServer(svc *Service, port string) *Server {
	s := &Server{
		indexer: svc,
	}

	r := mux.NewRouter()

	// Pool endpoints
	r.HandleFunc("/api/v1/pools", s.handleGetPools).Methods("GET")
	r.HandleFunc("/api/v1/pools/{id}", s.handleGetPool).Methods("GET")
	r.HandleFunc("/api/v1/pools/{id}/accounts", s.handleGetPoolAccounts).Methods("GET")
	r.HandleFunc("/api/v1/pools/{id}/richlist", s.handleGetPoolRichList).Methods("GET")

	// Transaction endpoints
	r.HandleFunc("/api/v1/transactions", s.handleGetTransactions).Methods("GET")
	r.HandleFunc("/api/v1/transactions/{id}", s.handleGetTransaction).Methods("GET")

	// Schema endpoint
	r.HandleFunc("/api/v1/schema", s.handleGetSchema).Methods("GET")

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

// handleGetPools returns all pools
func (s *Server) handleGetPools(w http.ResponseWriter, r *http.Request) {
	pools, err := s.indexer.QueryPools()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pools)
}

// handleGetPool returns a specific pool
func (s *Server) handleGetPool(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	poolID := vars["id"]

	// Get the first read model that supports pool queries
	for _, reader := range s.indexer.readers {
		if dexReader, ok := reader.(*DexReadModel); ok {
			if pool, exists := dexReader.GetPool(poolID); exists {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(pool)
				return
			}
		}
	}

	http.Error(w, "Pool not found", http.StatusNotFound)
}

// handleGetPoolAccounts returns liquidity accounts for a specific pool
func (s *Server) handleGetPoolAccounts(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	poolID := vars["id"]

	// Get the first read model that supports pool queries
	for _, reader := range s.indexer.readers {
		if dexReader, ok := reader.(*DexReadModel); ok {
			// Return 404 if pool doesn't exist (consistent with handleGetPool)
			if _, exists := dexReader.GetPool(poolID); !exists {
				http.Error(w, "Pool not found", http.StatusNotFound)
				return
			}
			accounts, err := dexReader.QueryLiquidityPositions(poolID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"pool_id":  poolID,
				"accounts": accounts,
			})
			return
		}
	}

	http.Error(w, "Pool not found", http.StatusNotFound)
}

// handleGetPoolRichList returns paginated rich list for a specific pool
func (s *Server) handleGetPoolRichList(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	poolID := vars["id"]

	// Parse pagination parameters
	offsetStr := r.URL.Query().Get("offset")
	limitStr := r.URL.Query().Get("limit")

	offset := 0
	limit := 50 // Default limit

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Get the first read model that supports pool queries
	for _, reader := range s.indexer.readers {
		if dexReader, ok := reader.(*DexReadModel); ok {
			// Return 404 if pool doesn't exist (consistent with handleGetPool)
			if _, exists := dexReader.GetPool(poolID); !exists {
				http.Error(w, "Pool not found", http.StatusNotFound)
				return
			}
			richList, err := dexReader.QueryRichList(poolID, offset, limit)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"pool_id": poolID,
				"offset":  offset,
				"limit":   limit,
				"holders": richList,
			})
			return
		}
	}

	http.Error(w, "Pool not found", http.StatusNotFound)
}

// handleGetTransactions returns transaction history with optional filtering
func (s *Server) handleGetTransactions(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	poolID := r.URL.Query().Get("pool_id")
	txType := r.URL.Query().Get("type")
	limitStr := r.URL.Query().Get("limit")

	limit := 100 // Default limit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	// Get the first read model that supports transaction queries
	for _, reader := range s.indexer.readers {
		if dexReader, ok := reader.(*DexReadModel); ok {
			transactions, err := dexReader.QueryTransactions(poolID, txType, limit)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"transactions": transactions,
				"count":        len(transactions),
			})
			return
		}
	}

	http.Error(w, "No transaction data available", http.StatusInternalServerError)
}

// handleGetTransaction returns a specific transaction by ID
func (s *Server) handleGetTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txID := vars["id"]

	// Get the first read model that supports transaction queries
	for _, reader := range s.indexer.readers {
		if dexReader, ok := reader.(*DexReadModel); ok {
			transaction, found := dexReader.GetTransaction(txID)
			if !found {
				http.Error(w, "Transaction not found", http.StatusNotFound)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(transaction)
			return
		}
	}

	http.Error(w, "Transaction not found", http.StatusNotFound)
}

// handleGetSchema returns the current instruction schema based on registered pools
func (s *Server) handleGetSchema(w http.ResponseWriter, r *http.Request) {
	// Get the first read model that supports schema queries
	for _, reader := range s.indexer.readers {
		if dexReader, ok := reader.(*DexReadModel); ok {
			schema := dexReader.GetSchema()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(schema)
			return
		}
	}

	// Fallback: return basic schema if no read model available
	http.Error(w, "Schema data not available", http.StatusInternalServerError)
}

// handleHealth provides health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "dex-indexer",
	})
}

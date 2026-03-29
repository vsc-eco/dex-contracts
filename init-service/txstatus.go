package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// FetchTransactionStatus queries the VSC node for a transaction's current status.
func FetchTransactionStatus(ctx context.Context, gqlURL string, txId string) (string, error) {
	gqlQuery := `query FindTransaction($filterOptions: TransactionFilter) {
		findTransaction(filterOptions: $filterOptions) {
			status
		}
	}`

	reqBody, err := json.Marshal(map[string]any{
		"query": gqlQuery,
		"variables": map[string]any{
			"filterOptions": map[string]any{
				"byId": txId,
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("marshalling graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gqlURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("querying transaction status: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			FindTransaction []struct {
				Status string `json:"status"`
			} `json:"findTransaction"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}
	if len(result.Errors) > 0 {
		return "", fmt.Errorf("graphql error: %s", result.Errors[0].Message)
	}
	if len(result.Data.FindTransaction) == 0 {
		return "", nil // not found yet
	}

	return result.Data.FindTransaction[0].Status, nil
}

// WaitForConfirmation polls the GraphQL endpoint until the transaction reaches
// CONFIRMED status or the context deadline is exceeded.
func WaitForConfirmation(ctx context.Context, gqlURL string, txId string, pollInterval time.Duration) error {
	log.Printf("waiting for tx %s to be confirmed...", txId)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for tx %s: %w", txId, ctx.Err())
		case <-ticker.C:
			status, err := FetchTransactionStatus(ctx, gqlURL, txId)
			if err != nil {
				log.Printf("  poll error for %s (will retry): %v", txId, err)
				continue
			}
			if status == "" {
				log.Printf("  tx %s not found yet, retrying...", txId)
				continue
			}

			log.Printf("  tx %s status: %s", txId, status)

			switch status {
			case "CONFIRMED":
				return nil
			case "FAILED":
				return fmt.Errorf("tx %s failed", txId)
			}
			// INCLUDED or other statuses: keep polling
		}
	}
}

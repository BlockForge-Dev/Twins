package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"twins/internal/core"
)

func TestMilestone2HTTPFlow(t *testing.T) {
	handler := NewServer(core.NewMemoryStore())

	businessBody := postJSON(t, handler, "/v1/businesses", "", "", map[string]string{
		"name": "Acme Labs",
	}, http.StatusCreated)
	apiKey, ok := businessBody["api_key"].(string)
	if !ok || apiKey == "" {
		t.Fatalf("api_key missing from response: %#v", businessBody)
	}

	walletBody := postJSON(t, handler, "/v1/wallets", apiKey, "", map[string]string{
		"label":   "Main Solana wallet",
		"chain":   "solana",
		"address": "7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDH",
	}, http.StatusCreated)
	wallet := walletBody["wallet"].(map[string]any)
	walletID := wallet["id"].(string)

	paymentRequestInput := map[string]any{
		"wallet_id":   walletID,
		"customer_id": "cust_123",
		"invoice_id":  "INV-1001",
		"amount":      "500.00",
		"token":       "USDC",
		"chain":       "solana",
		"expires_at":  time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
	}
	requestBody := postJSON(t, handler, "/v1/payment-requests", apiKey, "idem_001", paymentRequestInput, http.StatusCreated)
	request := requestBody["payment_request"].(map[string]any)
	if request["status"] != core.PaymentStatusAwaitingPayment {
		t.Fatalf("status = %q, want %q", request["status"], core.PaymentStatusAwaitingPayment)
	}

	replayedBody := postJSON(t, handler, "/v1/payment-requests", apiKey, "idem_001", paymentRequestInput, http.StatusOK)
	if replayedBody["idempotent_replayed"] != true {
		t.Fatalf("idempotent_replayed = %#v, want true", replayedBody["idempotent_replayed"])
	}

	listBody := getJSON(t, handler, "/v1/payment-requests", apiKey, http.StatusOK)
	requests := listBody["payment_requests"].([]any)
	if len(requests) != 1 {
		t.Fatalf("len(payment_requests) = %d, want 1", len(requests))
	}

	transactionBody := postJSON(t, handler, "/v1/stablecoin-transactions", apiKey, "", map[string]any{
		"chain":               "solana",
		"signature":           "5LqMEXAMPLE111111111111111111111111111111111111111111111111111",
		"slot":                123456,
		"confirmation_status": "finalized",
		"source_address":      "8xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDJ",
		"source_owner":        "9xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDK",
		"destination_address": "6xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDG",
		"destination_owner":   "7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDH",
		"token":               "USDC",
		"mint":                core.SolanaUSDCMint,
		"amount":              "500.00",
		"amount_atomic":       "500000000",
		"decimals":            6,
	}, http.StatusCreated)
	transaction := transactionBody["stablecoin_transaction"].(map[string]any)
	if transaction["status"] != core.TransactionStatusMatchedToRequest {
		t.Fatalf("transaction status = %q, want %q", transaction["status"], core.TransactionStatusMatchedToRequest)
	}

	transactionListBody := getJSON(t, handler, "/v1/stablecoin-transactions", apiKey, http.StatusOK)
	transactions := transactionListBody["stablecoin_transactions"].([]any)
	if len(transactions) != 1 {
		t.Fatalf("len(stablecoin_transactions) = %d, want 1", len(transactions))
	}

	updatedListBody := getJSON(t, handler, "/v1/payment-requests", apiKey, http.StatusOK)
	updatedRequests := updatedListBody["payment_requests"].([]any)
	updatedRequest := updatedRequests[0].(map[string]any)
	if updatedRequest["status"] != core.PaymentStatusConfirmed {
		t.Fatalf("payment request status = %q, want %q", updatedRequest["status"], core.PaymentStatusConfirmed)
	}

	matchListBody := getJSON(t, handler, "/v1/transaction-matches", apiKey, http.StatusOK)
	matches := matchListBody["transaction_matches"].([]any)
	if len(matches) != 1 {
		t.Fatalf("len(transaction_matches) = %d, want 1", len(matches))
	}

	exceptionListBody := getJSON(t, handler, "/v1/exceptions", apiKey, http.StatusOK)
	exceptions := exceptionListBody["exceptions"].([]any)
	if len(exceptions) != 0 {
		t.Fatalf("len(exceptions) = %d, want 0", len(exceptions))
	}

	auditBody := getJSON(t, handler, "/v1/audit-logs", apiKey, http.StatusOK)
	auditLogs := auditBody["audit_logs"].([]any)
	if len(auditLogs) < 5 {
		t.Fatalf("len(audit_logs) = %d, want at least 5", len(auditLogs))
	}
}

func postJSON(t *testing.T, handler http.Handler, path, apiKey, idempotencyKey string, body any, wantStatus int) map[string]any {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("POST %s status = %d, want %d, body = %s", path, rec.Code, wantStatus, rec.Body.String())
	}
	return decodeBody(t, rec)
}

func getJSON(t *testing.T, handler http.Handler, path, apiKey string, wantStatus int) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("GET %s status = %d, want %d, body = %s", path, rec.Code, wantStatus, rec.Body.String())
	}
	return decodeBody(t, rec)
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

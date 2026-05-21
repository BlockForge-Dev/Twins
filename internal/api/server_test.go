package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"twins/internal/core"
)

func TestMilestone2HTTPFlow(t *testing.T) {
	handler := NewServer(core.NewMemoryStore())
	var webhookHits int32
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&webhookHits, 1)
		if r.Header.Get("Twins-Event-Type") != core.ReceiptEventPaymentConfirmed {
			t.Errorf("Twins-Event-Type = %q, want %q", r.Header.Get("Twins-Event-Type"), core.ReceiptEventPaymentConfirmed)
		}
		if r.Header.Get("Twins-Signature") == "" {
			t.Error("Twins-Signature header is required")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer webhookServer.Close()

	businessBody := postJSON(t, handler, "/v1/businesses", "", "", map[string]string{
		"name": "Acme Labs",
	}, http.StatusCreated)
	apiKey, ok := businessBody["api_key"].(string)
	if !ok || apiKey == "" {
		t.Fatalf("api_key missing from response: %#v", businessBody)
	}

	otherBusinessBody := postJSON(t, handler, "/v1/businesses", "", "", map[string]string{
		"name": "Other Labs",
	}, http.StatusCreated)
	otherAPIKey := otherBusinessBody["api_key"].(string)

	policyBody := getJSON(t, handler, "/v1/security-policy", apiKey, http.StatusOK)
	policy := policyBody["security_policy"].(map[string]any)
	if policy["require_scoped_api_keys"] != true {
		t.Fatalf("require_scoped_api_keys = %#v, want true", policy["require_scoped_api_keys"])
	}

	userBody := postJSON(t, handler, "/v1/users", apiKey, "", map[string]string{
		"email": "ops@example.com",
		"name":  "Ops Lead",
		"role":  core.UserRoleOperator,
	}, http.StatusCreated)
	user := userBody["user"].(map[string]any)
	if user["role"] != core.UserRoleOperator {
		t.Fatalf("user role = %q, want %q", user["role"], core.UserRoleOperator)
	}
	userListBody := getJSON(t, handler, "/v1/users", apiKey, http.StatusOK)
	users := userListBody["users"].([]any)
	if len(users) != 2 {
		t.Fatalf("len(users) = %d, want 2", len(users))
	}

	limitedKeyBody := postJSON(t, handler, "/v1/api-keys", apiKey, "", map[string]any{
		"name":   "Payment request reader",
		"scopes": []string{core.ScopePaymentRequestsRead},
	}, http.StatusCreated)
	limitedAPIKey := limitedKeyBody["api_key"].(map[string]any)
	limitedSecret := limitedKeyBody["secret"].(string)
	if limitedSecret == "" {
		t.Fatal("limited API key secret is required")
	}
	if _, ok := limitedAPIKey["secret"]; ok {
		t.Fatal("api_key metadata must not include raw secret")
	}
	getJSON(t, handler, "/v1/payment-requests", limitedSecret, http.StatusOK)
	forbiddenBody := postJSON(t, handler, "/v1/wallets", limitedSecret, "", map[string]string{
		"label":   "Forbidden wallet",
		"chain":   "solana",
		"address": "8xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDH",
	}, http.StatusForbidden)
	forbiddenError := forbiddenBody["error"].(map[string]any)
	if forbiddenError["code"] != string(core.CodeForbidden) {
		t.Fatalf("forbidden error code = %q, want %q", forbiddenError["code"], core.CodeForbidden)
	}

	apiKeyListBody := getJSON(t, handler, "/v1/api-keys", apiKey, http.StatusOK)
	apiKeys := apiKeyListBody["api_keys"].([]any)
	if len(apiKeys) != 2 {
		t.Fatalf("len(api_keys) = %d, want 2", len(apiKeys))
	}

	revokedBody := postJSON(t, handler, "/v1/api-keys/"+limitedAPIKey["id"].(string)+"/revoke", apiKey, "", map[string]string{}, http.StatusOK)
	revoked := revokedBody["api_key"].(map[string]any)
	if revoked["revoked_at"] == nil {
		t.Fatal("revoked_at is required after API key revoke")
	}
	getJSON(t, handler, "/v1/payment-requests", limitedSecret, http.StatusUnauthorized)

	updatedPolicyBody := patchJSON(t, handler, "/v1/security-policy", apiKey, map[string]any{
		"rate_limit_per_minute":     120,
		"data_retention_days":       400,
		"access_log_retention_days": 120,
		"webhook_retention_days":    45,
		"incident_retention_days":   400,
		"require_scoped_api_keys":   true,
	}, http.StatusOK)
	updatedPolicy := updatedPolicyBody["security_policy"].(map[string]any)
	if updatedPolicy["rate_limit_per_minute"] != float64(120) {
		t.Fatalf("rate_limit_per_minute = %#v, want 120", updatedPolicy["rate_limit_per_minute"])
	}

	incidentBody := postJSON(t, handler, "/v1/incidents", apiKey, "", map[string]string{
		"title":       "Webhook endpoint unavailable",
		"severity":    "medium",
		"description": "Design partner endpoint refused delivery during local verification.",
	}, http.StatusCreated)
	incident := incidentBody["incident"].(map[string]any)
	if incident["status"] != core.IncidentStatusOpen {
		t.Fatalf("incident status = %q, want %q", incident["status"], core.IncidentStatusOpen)
	}
	resolvedIncidentBody := postJSON(t, handler, "/v1/incidents/"+incident["id"].(string)+"/resolve", apiKey, "", map[string]string{
		"summary": "Endpoint owner acknowledged the local test failure.",
	}, http.StatusOK)
	resolvedIncident := resolvedIncidentBody["incident"].(map[string]any)
	if resolvedIncident["status"] != core.IncidentStatusResolved {
		t.Fatalf("resolved incident status = %q, want %q", resolvedIncident["status"], core.IncidentStatusResolved)
	}

	walletBody := postJSON(t, handler, "/v1/wallets", apiKey, "", map[string]string{
		"label":   "Main Solana wallet",
		"chain":   "solana",
		"address": "7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDH",
	}, http.StatusCreated)
	wallet := walletBody["wallet"].(map[string]any)
	walletID := wallet["id"].(string)

	otherWalletsBody := getJSON(t, handler, "/v1/wallets", otherAPIKey, http.StatusOK)
	otherWallets := otherWalletsBody["wallets"].([]any)
	if len(otherWallets) != 0 {
		t.Fatalf("other tenant wallet count = %d, want 0", len(otherWallets))
	}

	webhookBody := postJSON(t, handler, "/v1/webhook-subscriptions", apiKey, "", map[string]any{
		"url":         webhookServer.URL,
		"secret":      "whsec_test",
		"event_types": []string{core.ReceiptEventPaymentConfirmed},
	}, http.StatusCreated)
	subscription := webhookBody["webhook_subscription"].(map[string]any)
	if subscription["url"] != webhookServer.URL {
		t.Fatalf("webhook url = %q, want %q", subscription["url"], webhookServer.URL)
	}

	subscriptionListBody := getJSON(t, handler, "/v1/webhook-subscriptions", apiKey, http.StatusOK)
	subscriptions := subscriptionListBody["webhook_subscriptions"].([]any)
	if len(subscriptions) != 1 {
		t.Fatalf("len(webhook_subscriptions) = %d, want 1", len(subscriptions))
	}

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
	paymentRequestID := updatedRequest["id"].(string)

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

	receiptEventBody := getJSON(t, handler, "/v1/receipt-events", apiKey, http.StatusOK)
	receiptEvents := receiptEventBody["receipt_events"].([]any)
	if len(receiptEvents) != 5 {
		t.Fatalf("len(receipt_events) = %d, want 5", len(receiptEvents))
	}
	firstReceiptEvent := receiptEvents[0].(map[string]any)
	lastReceiptEvent := receiptEvents[len(receiptEvents)-1].(map[string]any)
	if firstReceiptEvent["type"] != core.ReceiptEventPaymentRequestCreated || lastReceiptEvent["type"] != core.ReceiptEventPaymentConfirmed {
		t.Fatalf("receipt event order = first %q last %q, want created -> confirmed", firstReceiptEvent["type"], lastReceiptEvent["type"])
	}

	privateReceiptBody := getJSON(t, handler, "/v1/payment-requests/"+paymentRequestID+"/receipt", apiKey, http.StatusOK)
	privateReceipt := privateReceiptBody["receipt"].(map[string]any)
	privateEvents := privateReceipt["events"].([]any)
	if len(privateEvents) != 5 {
		t.Fatalf("len(private receipt events) = %d, want 5", len(privateEvents))
	}

	publicReceiptBody := getJSON(t, handler, "/receipts/"+paymentRequestID, "", http.StatusOK)
	publicReceipt := publicReceiptBody["receipt"].(map[string]any)
	publicEvents := publicReceipt["events"].([]any)
	if len(publicEvents) != 5 {
		t.Fatalf("len(public receipt events) = %d, want 5", len(publicEvents))
	}

	deliveryListBody := getJSON(t, handler, "/v1/webhook-deliveries", apiKey, http.StatusOK)
	deliveries := deliveryListBody["webhook_deliveries"].([]any)
	if len(deliveries) != 1 {
		t.Fatalf("len(webhook_deliveries) = %d, want 1", len(deliveries))
	}
	delivery := deliveries[0].(map[string]any)
	if delivery["status"] != core.WebhookStatusDelivered {
		t.Fatalf("webhook delivery status = %q, want %q", delivery["status"], core.WebhookStatusDelivered)
	}
	replayBody := postJSON(t, handler, "/v1/webhook-deliveries/"+delivery["id"].(string)+"/replay", apiKey, "", map[string]string{}, http.StatusOK)
	replayedDelivery := replayBody["webhook_delivery"].(map[string]any)
	if replayedDelivery["attempts"] != float64(2) {
		t.Fatalf("replayed attempts = %#v, want 2", replayedDelivery["attempts"])
	}
	if atomic.LoadInt32(&webhookHits) != 2 {
		t.Fatalf("webhook hits = %d, want 2", atomic.LoadInt32(&webhookHits))
	}

	reconciliationBody := postJSON(t, handler, "/v1/reconciliation-runs", apiKey, "", map[string]any{
		"period_start": time.Now().UTC().Add(-time.Hour).Format(time.RFC3339),
		"period_end":   time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
	}, http.StatusCreated)
	report := reconciliationBody["reconciliation_report"].(map[string]any)
	run := report["reconciliation_run"].(map[string]any)
	if run["status"] != core.ReconciliationStatusCompleted {
		t.Fatalf("reconciliation status = %q, want %q", run["status"], core.ReconciliationStatusCompleted)
	}
	if run["matched_transactions"] != float64(1) {
		t.Fatalf("matched_transactions = %#v, want 1", run["matched_transactions"])
	}
	rows := report["rows"].([]any)
	if len(rows) != 1 {
		t.Fatalf("len(settlement rows) = %d, want 1", len(rows))
	}

	reconciliationListBody := getJSON(t, handler, "/v1/reconciliation-runs", apiKey, http.StatusOK)
	runs := reconciliationListBody["reconciliation_runs"].([]any)
	if len(runs) != 1 {
		t.Fatalf("len(reconciliation_runs) = %d, want 1", len(runs))
	}
	reconciliationReportBody := getJSON(t, handler, "/v1/reconciliation-runs/"+run["id"].(string), apiKey, http.StatusOK)
	storedReport := reconciliationReportBody["reconciliation_report"].(map[string]any)
	storedRows := storedReport["rows"].([]any)
	if len(storedRows) != 1 {
		t.Fatalf("len(stored settlement rows) = %d, want 1", len(storedRows))
	}

	exportBody := postJSON(t, handler, "/v1/exports", apiKey, "", map[string]any{
		"reconciliation_run_id": run["id"].(string),
		"format":                "csv",
	}, http.StatusCreated)
	export := exportBody["export"].(map[string]any)
	if export["status"] != core.ExportStatusReady {
		t.Fatalf("export status = %q, want %q", export["status"], core.ExportStatusReady)
	}
	if export["row_count"] != float64(1) {
		t.Fatalf("export row_count = %#v, want 1", export["row_count"])
	}
	exportListBody := getJSON(t, handler, "/v1/exports", apiKey, http.StatusOK)
	exports := exportListBody["exports"].([]any)
	if len(exports) != 1 {
		t.Fatalf("len(exports) = %d, want 1", len(exports))
	}

	partnerBody := postJSON(t, handler, "/v1/design-partners", apiKey, "", map[string]any{
		"company_name":            "Beta AI Labs",
		"segment":                 "AI API company",
		"contact_name":            "Finance Lead",
		"contact_email":           "finance@beta.example",
		"use_case":                "USDC invoice matching",
		"status":                  core.DesignPartnerStatusOnboarding,
		"agreed_to_test":          true,
		"pricing_commitment":      true,
		"expected_monthly_volume": 250,
	}, http.StatusCreated)
	partner := partnerBody["design_partner"].(map[string]any)
	partnerStatus := core.DesignPartnerStatusActive
	updatedPartnerBody := patchJSON(t, handler, "/v1/design-partners/"+partner["id"].(string), apiKey, map[string]any{
		"status": partnerStatus,
	}, http.StatusOK)
	updatedPartner := updatedPartnerBody["design_partner"].(map[string]any)
	if updatedPartner["status"] != core.DesignPartnerStatusActive {
		t.Fatalf("design partner status = %q, want active", updatedPartner["status"])
	}

	postJSON(t, handler, "/v1/beta-evidence", apiKey, "", map[string]any{
		"design_partner_id":         partner["id"].(string),
		"type":                      core.BetaEvidenceTypeRealTransaction,
		"title":                     "First real USDC payment processed",
		"payment_request_id":        paymentRequestID,
		"stablecoin_transaction_id": transaction["id"].(string),
	}, http.StatusCreated)
	postJSON(t, handler, "/v1/beta-evidence", apiKey, "", map[string]any{
		"design_partner_id": partner["id"].(string),
		"type":              core.BetaEvidenceTypeTestimonial,
		"title":             "Finance workflow confidence",
		"quote":             "This makes USDC payment operations easier to prove.",
	}, http.StatusCreated)

	usageBody := getJSON(t, handler, "/v1/usage-metrics", apiKey, http.StatusOK)
	usage := usageBody["usage_metrics"].(map[string]any)
	if usage["payment_requests_created"] != float64(1) || usage["transactions_matched"] != float64(1) {
		t.Fatalf("usage metrics = %#v, want one matched payment", usage)
	}
	if usage["design_partners"] != float64(1) || usage["beta_evidence_items"] != float64(2) {
		t.Fatalf("beta usage metrics = %#v, want one partner and two evidence items", usage)
	}

	partnerListBody := getJSON(t, handler, "/v1/design-partners", apiKey, http.StatusOK)
	partners := partnerListBody["design_partners"].([]any)
	if len(partners) != 1 {
		t.Fatalf("len(design_partners) = %d, want 1", len(partners))
	}
	evidenceListBody := getJSON(t, handler, "/v1/beta-evidence", apiKey, http.StatusOK)
	evidence := evidenceListBody["beta_evidence"].([]any)
	if len(evidence) != 2 {
		t.Fatalf("len(beta_evidence) = %d, want 2", len(evidence))
	}
	betaReportBody := getJSON(t, handler, "/v1/private-beta-report", apiKey, http.StatusOK)
	betaReport := betaReportBody["private_beta_report"].(map[string]any)
	if betaReport["design_partners_onboarded"] != float64(1) || betaReport["partners_with_real_transactions"] != float64(1) {
		t.Fatalf("private beta report = %#v, want one onboarded transaction partner", betaReport)
	}

	incidentListBody := getJSON(t, handler, "/v1/incidents", apiKey, http.StatusOK)
	incidents := incidentListBody["incidents"].([]any)
	if len(incidents) != 1 {
		t.Fatalf("len(incidents) = %d, want 1", len(incidents))
	}

	accessLogBody := getJSON(t, handler, "/v1/access-logs", apiKey, http.StatusOK)
	accessLogs := accessLogBody["access_logs"].([]any)
	if len(accessLogs) == 0 {
		t.Fatal("expected access logs")
	}

	auditBody := getJSON(t, handler, "/v1/audit-logs", apiKey, http.StatusOK)
	auditLogs := auditBody["audit_logs"].([]any)
	if len(auditLogs) < 5 {
		t.Fatalf("len(audit_logs) = %d, want at least 5", len(auditLogs))
	}
}

func TestRateLimitUsesSecurityPolicy(t *testing.T) {
	handler := NewServer(core.NewMemoryStore())
	businessBody := postJSON(t, handler, "/v1/businesses", "", "", map[string]string{
		"name": "Rate Limited Labs",
	}, http.StatusCreated)
	apiKey := businessBody["api_key"].(string)

	patchJSON(t, handler, "/v1/security-policy", apiKey, map[string]any{
		"rate_limit_per_minute": 10,
	}, http.StatusOK)

	for i := 0; i < 9; i++ {
		getJSON(t, handler, "/v1/payment-requests", apiKey, http.StatusOK)
	}
	getJSON(t, handler, "/v1/payment-requests", apiKey, http.StatusTooManyRequests)
}

func TestBusinessCreationSetupToken(t *testing.T) {
	handler := NewServerWithOptions(core.NewMemoryStore(), ServerOptions{BusinessCreationToken: "setup-secret"})

	postJSON(t, handler, "/v1/businesses", "", "", map[string]string{
		"name": "Blocked Labs",
	}, http.StatusUnauthorized)

	body := postJSONWithHeaders(t, handler, "/v1/businesses", map[string]string{
		"X-Twins-Setup-Token": "setup-secret",
	}, map[string]string{
		"name": "Allowed Labs",
	}, http.StatusCreated)
	if body["api_key"] == "" {
		t.Fatalf("api_key missing from setup-token creation response: %#v", body)
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

func postJSONWithHeaders(t *testing.T, handler http.Handler, path string, headers map[string]string, body any, wantStatus int) map[string]any {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("POST %s status = %d, want %d, body = %s", path, rec.Code, wantStatus, rec.Body.String())
	}
	return decodeBody(t, rec)
}

func patchJSON(t *testing.T, handler http.Handler, path, apiKey string, body any, wantStatus int) map[string]any {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("PATCH %s status = %d, want %d, body = %s", path, rec.Code, wantStatus, rec.Body.String())
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

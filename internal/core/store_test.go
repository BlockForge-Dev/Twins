package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const testSolanaAddress = "7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDH"

func TestMilestone2PaymentRequestFlow(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	businessResult, err := store.CreateBusiness(ctx, CreateBusinessInput{Name: "Acme Labs"})
	if err != nil {
		t.Fatalf("CreateBusiness() error = %v", err)
	}

	business, apiKey, err := store.AuthenticateAPIKey(ctx, businessResult.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAPIKey() error = %v", err)
	}
	if business.ID != businessResult.Business.ID {
		t.Fatalf("authenticated business ID = %q, want %q", business.ID, businessResult.Business.ID)
	}

	wallet, err := store.RegisterWallet(ctx, business.ID, apiKey.ID, RegisterWalletInput{
		Label:   "Main Solana wallet",
		Chain:   "solana",
		Address: testSolanaAddress,
	})
	if err != nil {
		t.Fatalf("RegisterWallet() error = %v", err)
	}

	input := CreatePaymentRequestInput{
		WalletID:   wallet.ID,
		CustomerID: "cust_123",
		InvoiceID:  "INV-1001",
		Amount:     "500.00",
		Token:      "USDC",
		Chain:      "solana",
		ExpiresAt:  time.Now().UTC().Add(24 * time.Hour),
		Metadata:   map[string]string{"source": "test"},
	}

	result, err := store.CreatePaymentRequest(ctx, business.ID, apiKey.ID, "idem_001", input)
	if err != nil {
		t.Fatalf("CreatePaymentRequest() error = %v", err)
	}
	if result.PaymentRequest.Status != PaymentStatusAwaitingPayment {
		t.Fatalf("status = %q, want %q", result.PaymentRequest.Status, PaymentStatusAwaitingPayment)
	}
	if result.PaymentRequest.DestinationAddress != testSolanaAddress {
		t.Fatalf("destination address = %q, want %q", result.PaymentRequest.DestinationAddress, testSolanaAddress)
	}

	replayed, err := store.CreatePaymentRequest(ctx, business.ID, apiKey.ID, "idem_001", input)
	if err != nil {
		t.Fatalf("CreatePaymentRequest() replay error = %v", err)
	}
	if !replayed.IdempotentReplayed {
		t.Fatal("expected idempotent replay")
	}
	if replayed.PaymentRequest.ID != result.PaymentRequest.ID {
		t.Fatalf("replayed ID = %q, want %q", replayed.PaymentRequest.ID, result.PaymentRequest.ID)
	}
}

func TestPaymentRequestIdempotencyConflict(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	businessResult, err := store.CreateBusiness(ctx, CreateBusinessInput{Name: "Acme Labs"})
	if err != nil {
		t.Fatalf("CreateBusiness() error = %v", err)
	}
	business, apiKey, err := store.AuthenticateAPIKey(ctx, businessResult.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAPIKey() error = %v", err)
	}
	wallet, err := store.RegisterWallet(ctx, business.ID, apiKey.ID, RegisterWalletInput{
		Label:   "Main Solana wallet",
		Chain:   "solana",
		Address: testSolanaAddress,
	})
	if err != nil {
		t.Fatalf("RegisterWallet() error = %v", err)
	}

	input := CreatePaymentRequestInput{
		WalletID:   wallet.ID,
		CustomerID: "cust_123",
		InvoiceID:  "INV-1001",
		Amount:     "500.00",
		Token:      "USDC",
		Chain:      "solana",
		ExpiresAt:  time.Now().UTC().Add(24 * time.Hour),
	}
	if _, err := store.CreatePaymentRequest(ctx, business.ID, apiKey.ID, "idem_001", input); err != nil {
		t.Fatalf("CreatePaymentRequest() error = %v", err)
	}

	input.Amount = "501.00"
	_, err = store.CreatePaymentRequest(ctx, business.ID, apiKey.ID, "idem_001", input)
	appErr, ok := err.(*AppError)
	if !ok {
		t.Fatalf("error type = %T, want *AppError", err)
	}
	if appErr.Code != CodeConflict {
		t.Fatalf("error code = %q, want %q", appErr.Code, CodeConflict)
	}
}

func TestIngestStablecoinTransaction(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	businessResult, err := store.CreateBusiness(ctx, CreateBusinessInput{Name: "Acme Labs"})
	if err != nil {
		t.Fatalf("CreateBusiness() error = %v", err)
	}
	business, apiKey, err := store.AuthenticateAPIKey(ctx, businessResult.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAPIKey() error = %v", err)
	}
	wallet, err := store.RegisterWallet(ctx, business.ID, apiKey.ID, RegisterWalletInput{
		Label:   "Main Solana wallet",
		Chain:   "solana",
		Address: testSolanaAddress,
	})
	if err != nil {
		t.Fatalf("RegisterWallet() error = %v", err)
	}

	blockTime := int64(1_779_293_456)
	input := IngestStablecoinTransactionInput{
		Chain:              "solana",
		Signature:          "5LqMEXAMPLE111111111111111111111111111111111111111111111111111",
		Slot:               123456,
		BlockTime:          &blockTime,
		ConfirmationStatus: "finalized",
		SourceAddress:      "8xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDJ",
		SourceOwner:        "9xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDK",
		DestinationAddress: "6xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDG",
		DestinationOwner:   wallet.Address,
		Token:              "USDC",
		Mint:               SolanaUSDCMint,
		Amount:             "500.00",
		AmountAtomic:       "500000000",
		Decimals:           6,
	}

	result, err := store.IngestStablecoinTransaction(ctx, business.ID, apiKey.ID, input)
	if err != nil {
		t.Fatalf("IngestStablecoinTransaction() error = %v", err)
	}
	if result.StablecoinTransaction.WalletID != wallet.ID {
		t.Fatalf("wallet ID = %q, want %q", result.StablecoinTransaction.WalletID, wallet.ID)
	}
	if result.StablecoinTransaction.Status != TransactionStatusOrphan {
		t.Fatalf("status = %q, want %q", result.StablecoinTransaction.Status, TransactionStatusOrphan)
	}
	if result.Exception == nil || result.Exception.Type != ExceptionTypeOrphan {
		t.Fatalf("exception = %#v, want orphan exception", result.Exception)
	}

	replayed, err := store.IngestStablecoinTransaction(ctx, business.ID, apiKey.ID, input)
	if err != nil {
		t.Fatalf("IngestStablecoinTransaction() replay error = %v", err)
	}
	if !replayed.DuplicateReplayed {
		t.Fatal("expected duplicate replay")
	}
}

func TestMatchingEngineConfirmsExactPayment(t *testing.T) {
	ctx, store, business, apiKey, wallet := setupBusinessWallet(t)

	_, err := store.CreatePaymentRequest(ctx, business.ID, apiKey.ID, "idem_exact", CreatePaymentRequestInput{
		WalletID:   wallet.ID,
		CustomerID: "cust_123",
		InvoiceID:  "INV-1001",
		Amount:     "500.00",
		Token:      "USDC",
		Chain:      "solana",
		ExpiresAt:  time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("CreatePaymentRequest() error = %v", err)
	}

	result, err := store.IngestStablecoinTransaction(ctx, business.ID, apiKey.ID, stablecoinTxInput(wallet.Address, "sig_exact", "500.00", "500000000"))
	if err != nil {
		t.Fatalf("IngestStablecoinTransaction() error = %v", err)
	}
	if result.Exception != nil {
		t.Fatalf("exception = %#v, want nil", result.Exception)
	}
	if result.TransactionMatch == nil || result.TransactionMatch.Status != MatchStatusConfirmed {
		t.Fatalf("match = %#v, want confirmed match", result.TransactionMatch)
	}
	if result.StablecoinTransaction.Status != TransactionStatusMatchedToRequest {
		t.Fatalf("transaction status = %q, want %q", result.StablecoinTransaction.Status, TransactionStatusMatchedToRequest)
	}

	requests, err := store.ListPaymentRequests(ctx, business.ID)
	if err != nil {
		t.Fatalf("ListPaymentRequests() error = %v", err)
	}
	if requests[0].Status != PaymentStatusConfirmed {
		t.Fatalf("payment request status = %q, want %q", requests[0].Status, PaymentStatusConfirmed)
	}

	receipt, err := store.GetReceipt(ctx, business.ID, requests[0].ID)
	if err != nil {
		t.Fatalf("GetReceipt() error = %v", err)
	}
	if len(receipt.Events) != 5 {
		t.Fatalf("len(receipt.Events) = %d, want 5", len(receipt.Events))
	}
	if receipt.Events[0].Type != ReceiptEventPaymentRequestCreated {
		t.Fatalf("first receipt event type = %q, want %q", receipt.Events[0].Type, ReceiptEventPaymentRequestCreated)
	}
	if receipt.Events[len(receipt.Events)-1].Type != ReceiptEventPaymentConfirmed {
		t.Fatalf("last receipt event type = %q, want %q", receipt.Events[len(receipt.Events)-1].Type, ReceiptEventPaymentConfirmed)
	}

	events, err := store.ListReceiptEvents(ctx, business.ID)
	if err != nil {
		t.Fatalf("ListReceiptEvents() error = %v", err)
	}
	if events[0].Type != ReceiptEventPaymentRequestCreated || events[len(events)-1].Type != ReceiptEventPaymentConfirmed {
		t.Fatalf("receipt event order = first %q last %q, want created -> confirmed", events[0].Type, events[len(events)-1].Type)
	}
}

func TestMatchingEngineCreatesAmountExceptions(t *testing.T) {
	cases := []struct {
		name              string
		transactionAmount string
		atomicAmount      string
		wantRequestStatus string
		wantMatchStatus   string
		wantExceptionType string
	}{
		{
			name:              "underpaid",
			transactionAmount: "400.00",
			atomicAmount:      "400000000",
			wantRequestStatus: PaymentStatusUnderpaid,
			wantMatchStatus:   MatchStatusUnderpaid,
			wantExceptionType: ExceptionTypeUnderpaid,
		},
		{
			name:              "overpaid",
			transactionAmount: "600.00",
			atomicAmount:      "600000000",
			wantRequestStatus: PaymentStatusOverpaid,
			wantMatchStatus:   MatchStatusOverpaid,
			wantExceptionType: ExceptionTypeOverpaid,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, store, business, apiKey, wallet := setupBusinessWallet(t)

			_, err := store.CreatePaymentRequest(ctx, business.ID, apiKey.ID, "idem_"+tc.name, CreatePaymentRequestInput{
				WalletID:   wallet.ID,
				CustomerID: "cust_123",
				InvoiceID:  "INV-1001",
				Amount:     "500.00",
				Token:      "USDC",
				Chain:      "solana",
				ExpiresAt:  time.Now().UTC().Add(24 * time.Hour),
			})
			if err != nil {
				t.Fatalf("CreatePaymentRequest() error = %v", err)
			}

			result, err := store.IngestStablecoinTransaction(ctx, business.ID, apiKey.ID, stablecoinTxInput(wallet.Address, "sig_"+tc.name, tc.transactionAmount, tc.atomicAmount))
			if err != nil {
				t.Fatalf("IngestStablecoinTransaction() error = %v", err)
			}
			if result.TransactionMatch == nil || result.TransactionMatch.Status != tc.wantMatchStatus {
				t.Fatalf("match = %#v, want %q", result.TransactionMatch, tc.wantMatchStatus)
			}
			if result.Exception == nil || result.Exception.Type != tc.wantExceptionType {
				t.Fatalf("exception = %#v, want %q", result.Exception, tc.wantExceptionType)
			}

			requests, err := store.ListPaymentRequests(ctx, business.ID)
			if err != nil {
				t.Fatalf("ListPaymentRequests() error = %v", err)
			}
			if requests[0].Status != tc.wantRequestStatus {
				t.Fatalf("payment request status = %q, want %q", requests[0].Status, tc.wantRequestStatus)
			}
		})
	}
}

func TestResolveExceptionMarksRequestManuallyResolved(t *testing.T) {
	ctx, store, business, apiKey, wallet := setupBusinessWallet(t)

	_, err := store.CreatePaymentRequest(ctx, business.ID, apiKey.ID, "idem_underpaid", CreatePaymentRequestInput{
		WalletID:   wallet.ID,
		CustomerID: "cust_123",
		InvoiceID:  "INV-1001",
		Amount:     "500.00",
		Token:      "USDC",
		Chain:      "solana",
		ExpiresAt:  time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("CreatePaymentRequest() error = %v", err)
	}

	result, err := store.IngestStablecoinTransaction(ctx, business.ID, apiKey.ID, stablecoinTxInput(wallet.Address, "sig_resolve", "400.00", "400000000"))
	if err != nil {
		t.Fatalf("IngestStablecoinTransaction() error = %v", err)
	}
	if result.Exception == nil {
		t.Fatal("expected exception")
	}

	resolved, err := store.ResolveException(ctx, business.ID, apiKey.ID, result.Exception.ID, ResolveExceptionInput{Reason: "customer sent remaining balance separately"})
	if err != nil {
		t.Fatalf("ResolveException() error = %v", err)
	}
	if resolved.Status != ExceptionStatusResolved {
		t.Fatalf("exception status = %q, want %q", resolved.Status, ExceptionStatusResolved)
	}

	requests, err := store.ListPaymentRequests(ctx, business.ID)
	if err != nil {
		t.Fatalf("ListPaymentRequests() error = %v", err)
	}
	if requests[0].Status != PaymentStatusManuallyResolved {
		t.Fatalf("payment request status = %q, want %q", requests[0].Status, PaymentStatusManuallyResolved)
	}

	receipt, err := store.GetReceipt(ctx, business.ID, requests[0].ID)
	if err != nil {
		t.Fatalf("GetReceipt() error = %v", err)
	}
	if receipt.Events[len(receipt.Events)-1].Type != ReceiptEventExceptionResolved {
		t.Fatalf("last receipt event type = %q, want %q", receipt.Events[len(receipt.Events)-1].Type, ReceiptEventExceptionResolved)
	}
}

func TestWebhookDeliveryAndReplay(t *testing.T) {
	ctx, store, business, apiKey, wallet := setupBusinessWallet(t)

	var mu sync.Mutex
	received := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		received = append(received, r.Header.Get("Twins-Signature"))
		mu.Unlock()
		if r.Header.Get("Twins-Event-Type") != ReceiptEventPaymentConfirmed {
			t.Errorf("Twins-Event-Type = %q, want %q", r.Header.Get("Twins-Event-Type"), ReceiptEventPaymentConfirmed)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	subscriptionResult, err := store.CreateWebhookSubscription(ctx, business.ID, apiKey.ID, CreateWebhookSubscriptionInput{
		URL:        server.URL,
		Secret:     "whsec_test",
		EventTypes: []string{ReceiptEventPaymentConfirmed},
	})
	if err != nil {
		t.Fatalf("CreateWebhookSubscription() error = %v", err)
	}
	if subscriptionResult.SigningSecret != "" {
		t.Fatalf("SigningSecret = %q, want empty for supplied secret", subscriptionResult.SigningSecret)
	}

	requestResult, err := store.CreatePaymentRequest(ctx, business.ID, apiKey.ID, "idem_webhook", CreatePaymentRequestInput{
		WalletID:   wallet.ID,
		CustomerID: "cust_123",
		InvoiceID:  "INV-1001",
		Amount:     "500.00",
		Token:      "USDC",
		Chain:      "solana",
		ExpiresAt:  time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("CreatePaymentRequest() error = %v", err)
	}

	if _, err := store.IngestStablecoinTransaction(ctx, business.ID, apiKey.ID, stablecoinTxInput(wallet.Address, "sig_webhook", "500.00", "500000000")); err != nil {
		t.Fatalf("IngestStablecoinTransaction() error = %v", err)
	}

	deliveries, err := store.ListWebhookDeliveries(ctx, business.ID)
	if err != nil {
		t.Fatalf("ListWebhookDeliveries() error = %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("len(deliveries) = %d, want 1", len(deliveries))
	}
	if deliveries[0].Status != WebhookStatusDelivered {
		t.Fatalf("delivery status = %q, want %q", deliveries[0].Status, WebhookStatusDelivered)
	}
	if deliveries[0].Attempts != 1 {
		t.Fatalf("delivery attempts = %d, want 1", deliveries[0].Attempts)
	}
	if !strings.HasPrefix(deliveries[0].Signature, "sha256=") {
		t.Fatalf("delivery signature = %q, want sha256 prefix", deliveries[0].Signature)
	}
	mu.Lock()
	if len(received) != 1 || received[0] != deliveries[0].Signature {
		mu.Unlock()
		t.Fatalf("received signatures = %#v, want delivery signature", received)
	}
	mu.Unlock()

	replayed, err := store.ReplayWebhookDelivery(ctx, business.ID, apiKey.ID, deliveries[0].ID)
	if err != nil {
		t.Fatalf("ReplayWebhookDelivery() error = %v", err)
	}
	if replayed.Attempts != 2 {
		t.Fatalf("replayed attempts = %d, want 2", replayed.Attempts)
	}
	if replayed.Status != WebhookStatusDelivered {
		t.Fatalf("replayed status = %q, want %q", replayed.Status, WebhookStatusDelivered)
	}
	mu.Lock()
	receivedCount := len(received)
	mu.Unlock()
	if receivedCount != 2 {
		t.Fatalf("received count = %d, want 2", receivedCount)
	}

	publicReceipt, err := store.GetPublicReceipt(ctx, requestResult.PaymentRequest.ID)
	if err != nil {
		t.Fatalf("GetPublicReceipt() error = %v", err)
	}
	if len(publicReceipt.Events) != 5 {
		t.Fatalf("len(publicReceipt.Events) = %d, want 5", len(publicReceipt.Events))
	}
}

func TestReconciliationRunAndExport(t *testing.T) {
	ctx, store, business, apiKey, wallet := setupBusinessWallet(t)

	_, err := store.CreatePaymentRequest(ctx, business.ID, apiKey.ID, "idem_reconciliation", CreatePaymentRequestInput{
		WalletID:   wallet.ID,
		CustomerID: "cust_123",
		InvoiceID:  "INV-1001",
		Amount:     "500.00",
		Token:      "USDC",
		Chain:      "solana",
		ExpiresAt:  time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("CreatePaymentRequest() error = %v", err)
	}
	if _, err := store.IngestStablecoinTransaction(ctx, business.ID, apiKey.ID, stablecoinTxInput(wallet.Address, "sig_reconcile", "500.00", "500000000")); err != nil {
		t.Fatalf("IngestStablecoinTransaction() error = %v", err)
	}

	report, err := store.CreateReconciliationRun(ctx, business.ID, apiKey.ID, CreateReconciliationRunInput{
		PeriodStart: time.Now().UTC().Add(-time.Hour),
		PeriodEnd:   time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateReconciliationRun() error = %v", err)
	}
	run := report.ReconciliationRun
	if run.Status != ReconciliationStatusCompleted {
		t.Fatalf("reconciliation status = %q, want %q", run.Status, ReconciliationStatusCompleted)
	}
	if run.TotalPaymentRequests != 1 || run.TotalTransactions != 1 || run.MatchedTransactions != 1 || run.UnmatchedTransactions != 0 {
		t.Fatalf("run counts = %#v, want one matched payment and transaction", run)
	}
	if run.TotalReceivedUSDC != "500" {
		t.Fatalf("total received = %q, want 500", run.TotalReceivedUSDC)
	}
	if len(report.WalletSnapshots) != 1 {
		t.Fatalf("len(wallet snapshots) = %d, want 1", len(report.WalletSnapshots))
	}
	if report.WalletSnapshots[0].ObservedInboundAmount != "500" {
		t.Fatalf("snapshot inbound amount = %q, want 500", report.WalletSnapshots[0].ObservedInboundAmount)
	}
	if len(report.Rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(report.Rows))
	}
	if report.Rows[0].ReconciliationStatus != "reconciled" {
		t.Fatalf("row reconciliation status = %q, want reconciled", report.Rows[0].ReconciliationStatus)
	}

	export, err := store.CreateExport(ctx, business.ID, apiKey.ID, CreateExportInput{
		ReconciliationRunID: run.ID,
		Format:              ExportFormatCSV,
	})
	if err != nil {
		t.Fatalf("CreateExport() error = %v", err)
	}
	if export.Status != ExportStatusReady {
		t.Fatalf("export status = %q, want %q", export.Status, ExportStatusReady)
	}
	if export.RowCount != 1 {
		t.Fatalf("export row count = %d, want 1", export.RowCount)
	}
	if !strings.Contains(export.Content, "INV-1001") {
		t.Fatalf("export content = %q, want invoice ID", export.Content)
	}
}

func TestSecurityControlsAndTenantRecords(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	businessResult, err := store.CreateBusiness(ctx, CreateBusinessInput{
		Name:       "Acme Labs",
		OwnerEmail: "owner@acme.example",
	})
	if err != nil {
		t.Fatalf("CreateBusiness() error = %v", err)
	}
	if businessResult.Owner.Role != UserRoleOwner || businessResult.Owner.Status != UserStatusActive {
		t.Fatalf("owner = %#v, want active owner", businessResult.Owner)
	}

	business, apiKey, err := store.AuthenticateAPIKey(ctx, businessResult.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAPIKey() error = %v", err)
	}

	operator, err := store.CreateUser(ctx, business.ID, apiKey.ID, CreateUserInput{
		Email: "ops@acme.example",
		Name:  "Ops Lead",
		Role:  UserRoleOperator,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if operator.Role != UserRoleOperator {
		t.Fatalf("operator role = %q, want %q", operator.Role, UserRoleOperator)
	}
	users, err := store.ListUsers(ctx, business.ID)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("len(users) = %d, want 2", len(users))
	}

	if _, err := store.CreateAPIKey(ctx, business.ID, apiKey.ID, CreateAPIKeyInput{
		Name:   "Invalid scoped key",
		Scopes: []string{"payments:delete"},
	}); err == nil {
		t.Fatal("CreateAPIKey() with invalid scope succeeded, want error")
	}

	limitedResult, err := store.CreateAPIKey(ctx, business.ID, apiKey.ID, CreateAPIKeyInput{
		Name:   "Payment reader",
		Scopes: []string{ScopePaymentRequestsRead},
	})
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if limitedResult.Secret == "" {
		t.Fatal("limited API key secret is required")
	}
	if len(limitedResult.APIKey.Scopes) != 1 || limitedResult.APIKey.Scopes[0] != ScopePaymentRequestsRead {
		t.Fatalf("limited scopes = %#v, want payment_requests:read", limitedResult.APIKey.Scopes)
	}

	_, limitedKey, err := store.AuthenticateAPIKey(ctx, limitedResult.Secret)
	if err != nil {
		t.Fatalf("AuthenticateAPIKey(limited) error = %v", err)
	}
	if limitedKey.ID != limitedResult.APIKey.ID {
		t.Fatalf("limited key ID = %q, want %q", limitedKey.ID, limitedResult.APIKey.ID)
	}

	revoked, err := store.RevokeAPIKey(ctx, business.ID, apiKey.ID, limitedKey.ID)
	if err != nil {
		t.Fatalf("RevokeAPIKey() error = %v", err)
	}
	if revoked.RevokedAt == nil {
		t.Fatal("revoked API key must have RevokedAt")
	}
	if _, _, err := store.AuthenticateAPIKey(ctx, limitedResult.Secret); err == nil {
		t.Fatal("revoked API key authenticated, want error")
	}

	store.RecordAccessLog(ctx, RecordAccessLogInput{
		BusinessID: business.ID,
		APIKeyID:   apiKey.ID,
		Method:     http.MethodGet,
		Path:       "/v1/payment-requests",
		StatusCode: http.StatusOK,
		RemoteAddr: "192.0.2.1:50000",
	})
	accessLogs, err := store.ListAccessLogs(ctx, business.ID)
	if err != nil {
		t.Fatalf("ListAccessLogs() error = %v", err)
	}
	if len(accessLogs) != 1 || accessLogs[0].Path != "/v1/payment-requests" {
		t.Fatalf("access logs = %#v, want recorded request", accessLogs)
	}

	incident, err := store.CreateIncident(ctx, business.ID, apiKey.ID, CreateIncidentInput{
		Title:       "Webhook endpoint refused local test",
		Severity:    "high",
		Description: "The endpoint was unavailable during verification.",
	})
	if err != nil {
		t.Fatalf("CreateIncident() error = %v", err)
	}
	if incident.Status != IncidentStatusOpen {
		t.Fatalf("incident status = %q, want %q", incident.Status, IncidentStatusOpen)
	}
	resolved, err := store.ResolveIncident(ctx, business.ID, apiKey.ID, incident.ID, ResolveIncidentInput{
		Summary: "Endpoint health restored.",
	})
	if err != nil {
		t.Fatalf("ResolveIncident() error = %v", err)
	}
	if resolved.Status != IncidentStatusResolved || resolved.ResolutionSummary == "" {
		t.Fatalf("resolved incident = %#v, want resolved with summary", resolved)
	}

	policy, err := store.GetSecurityPolicy(ctx, business.ID)
	if err != nil {
		t.Fatalf("GetSecurityPolicy() error = %v", err)
	}
	if !policy.RequireScopedAPIKeys {
		t.Fatal("default policy should require scoped API keys")
	}
	retentionDays := 400
	rateLimit := 120
	updatedPolicy, err := store.UpdateSecurityPolicy(ctx, business.ID, apiKey.ID, UpdateSecurityPolicyInput{
		RateLimitPerMinute: &rateLimit,
		DataRetentionDays:  &retentionDays,
	})
	if err != nil {
		t.Fatalf("UpdateSecurityPolicy() error = %v", err)
	}
	if updatedPolicy.RateLimitPerMinute != rateLimit || updatedPolicy.DataRetentionDays != retentionDays {
		t.Fatalf("updated policy = %#v, want custom rate and retention", updatedPolicy)
	}

	otherBusinessResult, err := store.CreateBusiness(ctx, CreateBusinessInput{Name: "Other Labs"})
	if err != nil {
		t.Fatalf("CreateBusiness(other) error = %v", err)
	}
	otherUsers, err := store.ListUsers(ctx, otherBusinessResult.Business.ID)
	if err != nil {
		t.Fatalf("ListUsers(other) error = %v", err)
	}
	if len(otherUsers) != 1 {
		t.Fatalf("len(otherUsers) = %d, want only owner", len(otherUsers))
	}
	otherAccessLogs, err := store.ListAccessLogs(ctx, otherBusinessResult.Business.ID)
	if err != nil {
		t.Fatalf("ListAccessLogs(other) error = %v", err)
	}
	if len(otherAccessLogs) != 0 {
		t.Fatalf("len(otherAccessLogs) = %d, want 0", len(otherAccessLogs))
	}

	auditLogs, err := store.ListAuditLogs(ctx, business.ID)
	if err != nil {
		t.Fatalf("ListAuditLogs() error = %v", err)
	}
	actions := make(map[string]bool)
	for _, log := range auditLogs {
		actions[log.Action] = true
	}
	for _, action := range []string{"user.created", "api_key.created", "api_key.revoked", "incident.created", "incident.resolved", "security_policy.updated"} {
		if !actions[action] {
			t.Fatalf("audit logs missing %q in %#v", action, actions)
		}
	}
}

func TestPrivateBetaReadinessAndUsageMetrics(t *testing.T) {
	ctx, store, business, apiKey, wallet := setupBusinessWallet(t)

	requestResult, err := store.CreatePaymentRequest(ctx, business.ID, apiKey.ID, "idem_beta", CreatePaymentRequestInput{
		WalletID:   wallet.ID,
		CustomerID: "cust_beta",
		InvoiceID:  "INV-BETA-001",
		Amount:     "500.00",
		Token:      "USDC",
		Chain:      "solana",
		ExpiresAt:  time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("CreatePaymentRequest() error = %v", err)
	}
	transactionResult, err := store.IngestStablecoinTransaction(ctx, business.ID, apiKey.ID, stablecoinTxInput(wallet.Address, "sig_beta", "500.00", "500000000"))
	if err != nil {
		t.Fatalf("IngestStablecoinTransaction() error = %v", err)
	}
	report, err := store.CreateReconciliationRun(ctx, business.ID, apiKey.ID, CreateReconciliationRunInput{
		PeriodStart: time.Now().UTC().Add(-time.Hour),
		PeriodEnd:   time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateReconciliationRun() error = %v", err)
	}
	if len(report.Rows) != 1 {
		t.Fatalf("len(report.Rows) = %d, want 1", len(report.Rows))
	}

	partner, err := store.CreateDesignPartner(ctx, business.ID, apiKey.ID, CreateDesignPartnerInput{
		CompanyName:           "Beta AI Labs",
		Segment:               "AI API company",
		ContactName:           "Finance Lead",
		ContactEmail:          "finance@beta.example",
		UseCase:               "USDC invoice matching",
		Status:                DesignPartnerStatusOnboarding,
		AgreedToTest:          true,
		PricingCommitment:     true,
		ExpectedMonthlyVolume: 250,
	})
	if err != nil {
		t.Fatalf("CreateDesignPartner() error = %v", err)
	}
	if partner.Status != DesignPartnerStatusOnboarding {
		t.Fatalf("partner status = %q, want onboarding", partner.Status)
	}

	status := DesignPartnerStatusActive
	updatedPartner, err := store.UpdateDesignPartner(ctx, business.ID, apiKey.ID, partner.ID, UpdateDesignPartnerInput{Status: &status})
	if err != nil {
		t.Fatalf("UpdateDesignPartner() error = %v", err)
	}
	if updatedPartner.Status != DesignPartnerStatusActive {
		t.Fatalf("updated status = %q, want active", updatedPartner.Status)
	}

	if _, err := store.CreateBetaEvidence(ctx, business.ID, apiKey.ID, CreateBetaEvidenceInput{
		DesignPartnerID:         partner.ID,
		Type:                    BetaEvidenceTypeRealTransaction,
		Title:                   "First real USDC payment processed",
		PaymentRequestID:        requestResult.PaymentRequest.ID,
		StablecoinTransactionID: transactionResult.StablecoinTransaction.ID,
	}); err != nil {
		t.Fatalf("CreateBetaEvidence(real transaction) error = %v", err)
	}
	if _, err := store.CreateBetaEvidence(ctx, business.ID, apiKey.ID, CreateBetaEvidenceInput{
		DesignPartnerID: partner.ID,
		Type:            BetaEvidenceTypeTestimonial,
		Title:           "Finance workflow confidence",
		Quote:           "This makes USDC payment operations easier to prove.",
	}); err != nil {
		t.Fatalf("CreateBetaEvidence(testimonial) error = %v", err)
	}

	metrics, err := store.GetUsageMetrics(ctx, business.ID)
	if err != nil {
		t.Fatalf("GetUsageMetrics() error = %v", err)
	}
	if metrics.PaymentRequestsCreated != 1 || metrics.TransactionsDetected != 1 || metrics.TransactionsMatched != 1 {
		t.Fatalf("usage payment metrics = %#v, want one matched payment", metrics)
	}
	if metrics.ReconciledBusinessRecords != 1 {
		t.Fatalf("reconciled business records = %d, want 1", metrics.ReconciledBusinessRecords)
	}
	if metrics.DesignPartners != 1 || metrics.ActiveDesignPartners != 1 || metrics.PricingCommitments != 1 {
		t.Fatalf("usage beta metrics = %#v, want one active pricing partner", metrics)
	}
	if metrics.BetaEvidenceItems != 2 || metrics.Testimonials != 1 || metrics.PrivateBetaTransactionsProcessed != 1 {
		t.Fatalf("usage evidence metrics = %#v, want transaction and testimonial evidence", metrics)
	}

	betaReport, err := store.GetPrivateBetaReport(ctx, business.ID)
	if err != nil {
		t.Fatalf("GetPrivateBetaReport() error = %v", err)
	}
	if betaReport.DesignPartnersOnboarded != 1 || betaReport.PartnersWithRealTransactions != 1 || betaReport.PricingCommitments != 1 {
		t.Fatalf("private beta report = %#v, want one onboarded transaction partner with pricing", betaReport)
	}
	if betaReport.ReadyForPrivateBetaEvidence {
		t.Fatal("private beta report should not be ready with only one design partner")
	}

	auditLogs, err := store.ListAuditLogs(ctx, business.ID)
	if err != nil {
		t.Fatalf("ListAuditLogs() error = %v", err)
	}
	actions := make(map[string]bool)
	for _, log := range auditLogs {
		actions[log.Action] = true
	}
	for _, action := range []string{"design_partner.created", "design_partner.updated", "beta_evidence.created"} {
		if !actions[action] {
			t.Fatalf("audit logs missing %q in %#v", action, actions)
		}
	}
}

func TestPersistentStoreSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "twins-store.json")

	store, err := NewPersistentStore(path)
	if err != nil {
		t.Fatalf("NewPersistentStore() error = %v", err)
	}
	businessResult, err := store.CreateBusiness(ctx, CreateBusinessInput{
		Name:       "Durable Labs",
		OwnerEmail: "owner@durable.example",
	})
	if err != nil {
		t.Fatalf("CreateBusiness() error = %v", err)
	}
	business, apiKey, err := store.AuthenticateAPIKey(ctx, businessResult.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAPIKey() error = %v", err)
	}
	wallet, err := store.RegisterWallet(ctx, business.ID, apiKey.ID, RegisterWalletInput{
		Label:   "Durable Solana wallet",
		Chain:   ChainSolana,
		Address: testSolanaAddress,
	})
	if err != nil {
		t.Fatalf("RegisterWallet() error = %v", err)
	}
	_, err = store.CreatePaymentRequest(ctx, business.ID, apiKey.ID, "idem_durable", CreatePaymentRequestInput{
		WalletID:   wallet.ID,
		CustomerID: "cust_durable",
		InvoiceID:  "INV-DURABLE",
		Amount:     "500.00",
		Token:      TokenUSDC,
		Chain:      ChainSolana,
		ExpiresAt:  time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("CreatePaymentRequest() error = %v", err)
	}

	reloaded, err := NewPersistentStore(path)
	if err != nil {
		t.Fatalf("NewPersistentStore(reloaded) error = %v", err)
	}
	reloadedBusiness, reloadedKey, err := reloaded.AuthenticateAPIKey(ctx, businessResult.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAPIKey(reloaded) error = %v", err)
	}
	if reloadedBusiness.ID != business.ID || reloadedKey.ID != apiKey.ID {
		t.Fatalf("reloaded auth = business %q key %q, want %q %q", reloadedBusiness.ID, reloadedKey.ID, business.ID, apiKey.ID)
	}
	requests, err := reloaded.ListPaymentRequests(ctx, business.ID)
	if err != nil {
		t.Fatalf("ListPaymentRequests(reloaded) error = %v", err)
	}
	if len(requests) != 1 || requests[0].InvoiceID != "INV-DURABLE" {
		t.Fatalf("reloaded requests = %#v, want durable request", requests)
	}
	persistent, storagePath, err := reloaded.PersistenceStatus()
	if err != nil {
		t.Fatalf("PersistenceStatus() error = %v", err)
	}
	if !persistent || storagePath != path {
		t.Fatalf("persistence = %v path %q, want true %q", persistent, storagePath, path)
	}
}

func setupBusinessWallet(t *testing.T) (context.Context, *MemoryStore, Business, APIKey, Wallet) {
	t.Helper()

	ctx := context.Background()
	store := NewMemoryStore()

	businessResult, err := store.CreateBusiness(ctx, CreateBusinessInput{Name: "Acme Labs"})
	if err != nil {
		t.Fatalf("CreateBusiness() error = %v", err)
	}
	business, apiKey, err := store.AuthenticateAPIKey(ctx, businessResult.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAPIKey() error = %v", err)
	}
	wallet, err := store.RegisterWallet(ctx, business.ID, apiKey.ID, RegisterWalletInput{
		Label:   "Main Solana wallet",
		Chain:   "solana",
		Address: testSolanaAddress,
	})
	if err != nil {
		t.Fatalf("RegisterWallet() error = %v", err)
	}

	return ctx, store, business, apiKey, wallet
}

func stablecoinTxInput(destinationOwner, signature, amount, atomicAmount string) IngestStablecoinTransactionInput {
	return IngestStablecoinTransactionInput{
		Chain:              "solana",
		Signature:          signature,
		Slot:               123456,
		ConfirmationStatus: "finalized",
		SourceAddress:      "8xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDJ",
		SourceOwner:        "9xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDK",
		DestinationAddress: "6xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgEDG",
		DestinationOwner:   destinationOwner,
		Token:              "USDC",
		Mint:               SolanaUSDCMint,
		Amount:             amount,
		AmountAtomic:       atomicAmount,
		Decimals:           6,
	}
}

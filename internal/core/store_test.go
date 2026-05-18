package core

import (
	"context"
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

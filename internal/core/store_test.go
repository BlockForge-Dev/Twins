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
	if result.StablecoinTransaction.Status != TransactionStatusConfirmedOnchain {
		t.Fatalf("status = %q, want %q", result.StablecoinTransaction.Status, TransactionStatusConfirmedOnchain)
	}
	if result.StablecoinTransaction.WalletID != wallet.ID {
		t.Fatalf("wallet ID = %q, want %q", result.StablecoinTransaction.WalletID, wallet.ID)
	}

	replayed, err := store.IngestStablecoinTransaction(ctx, business.ID, apiKey.ID, input)
	if err != nil {
		t.Fatalf("IngestStablecoinTransaction() replay error = %v", err)
	}
	if !replayed.DuplicateReplayed {
		t.Fatal("expected duplicate replay")
	}
}

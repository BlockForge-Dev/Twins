package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu sync.RWMutex

	businesses map[string]Business
	apiKeys    map[string]APIKey
	apiKeyHash map[string]string

	wallets map[string]Wallet

	paymentRequests map[string]PaymentRequest
	idempotency     map[string]idempotencyRecord

	auditLogs map[string]AuditLog
}

type idempotencyRecord struct {
	BodyHash         string
	PaymentRequestID string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		businesses:      make(map[string]Business),
		apiKeys:         make(map[string]APIKey),
		apiKeyHash:      make(map[string]string),
		wallets:         make(map[string]Wallet),
		paymentRequests: make(map[string]PaymentRequest),
		idempotency:     make(map[string]idempotencyRecord),
		auditLogs:       make(map[string]AuditLog),
	}
}

func (s *MemoryStore) CreateBusiness(_ context.Context, input CreateBusinessInput) (CreateBusinessResult, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return CreateBusinessResult{}, InvalidArgument("business name is required")
	}

	now := time.Now().UTC()
	rawKey, err := newAPIKey()
	if err != nil {
		return CreateBusinessResult{}, err
	}
	keyHash := hashSecret(rawKey)

	business := Business{
		ID:        newID("biz"),
		Name:      name,
		CreatedAt: now,
	}
	apiKey := APIKey{
		ID:         newID("key"),
		BusinessID: business.ID,
		Name:       "Default API key",
		Prefix:     rawKey[:min(18, len(rawKey))],
		SecretHash: keyHash,
		Scopes:     []string{"wallets:write", "payment_requests:write", "payment_requests:read", "audit_logs:read"},
		CreatedAt:  now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.businesses[business.ID] = business
	s.apiKeys[apiKey.ID] = apiKey
	s.apiKeyHash[keyHash] = apiKey.ID
	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   business.ID,
		ActorType:    "system",
		ActorID:      "system",
		Action:       "business.created",
		ResourceType: "business",
		ResourceID:   business.ID,
		CreatedAt:    now,
	})
	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   business.ID,
		ActorType:    "system",
		ActorID:      "system",
		Action:       "api_key.created",
		ResourceType: "api_key",
		ResourceID:   apiKey.ID,
		Metadata:     map[string]string{"prefix": apiKey.Prefix},
		CreatedAt:    now,
	})

	return CreateBusinessResult{Business: business, APIKey: rawKey}, nil
}

func (s *MemoryStore) AuthenticateAPIKey(_ context.Context, rawKey string) (Business, APIKey, error) {
	rawKey = strings.TrimSpace(rawKey)
	if rawKey == "" {
		return Business{}, APIKey{}, Unauthorized("missing API key")
	}

	keyHash := hashSecret(rawKey)

	s.mu.RLock()
	defer s.mu.RUnlock()

	keyID, ok := s.apiKeyHash[keyHash]
	if !ok {
		return Business{}, APIKey{}, Unauthorized("invalid API key")
	}
	apiKey := s.apiKeys[keyID]
	if apiKey.RevokedAt != nil {
		return Business{}, APIKey{}, Unauthorized("API key has been revoked")
	}
	business, ok := s.businesses[apiKey.BusinessID]
	if !ok {
		return Business{}, APIKey{}, Unauthorized("API key business does not exist")
	}

	return business, apiKey, nil
}

func (s *MemoryStore) RegisterWallet(_ context.Context, businessID, actorID string, input RegisterWalletInput) (Wallet, error) {
	label := strings.TrimSpace(input.Label)
	chain := strings.ToLower(strings.TrimSpace(input.Chain))
	address := strings.TrimSpace(input.Address)

	if label == "" {
		return Wallet{}, InvalidArgument("wallet label is required")
	}
	if chain != ChainSolana {
		return Wallet{}, InvalidArgument("only solana wallets are supported in v1")
	}
	if err := validateAddress(address); err != nil {
		return Wallet{}, err
	}

	now := time.Now().UTC()
	wallet := Wallet{
		ID:         newID("wal"),
		BusinessID: businessID,
		Label:      label,
		Chain:      chain,
		Address:    address,
		CreatedAt:  now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.wallets {
		if existing.BusinessID == businessID && existing.Chain == chain && existing.Address == address && existing.ArchivedAt == nil {
			return Wallet{}, Conflict("wallet is already registered for this business")
		}
	}

	s.wallets[wallet.ID] = wallet
	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   businessID,
		ActorType:    "api_key",
		ActorID:      actorID,
		Action:       "wallet.registered",
		ResourceType: "wallet",
		ResourceID:   wallet.ID,
		Metadata:     map[string]string{"chain": wallet.Chain},
		CreatedAt:    now,
	})

	return wallet, nil
}

func (s *MemoryStore) ListWallets(_ context.Context, businessID string) ([]Wallet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wallets := make([]Wallet, 0)
	for _, wallet := range s.wallets {
		if wallet.BusinessID == businessID && wallet.ArchivedAt == nil {
			wallets = append(wallets, wallet)
		}
	}
	sort.Slice(wallets, func(i, j int) bool {
		return wallets[i].CreatedAt.Before(wallets[j].CreatedAt)
	})
	return wallets, nil
}

func (s *MemoryStore) CreatePaymentRequest(_ context.Context, businessID, actorID, idempotencyKey string, input CreatePaymentRequestInput) (CreatePaymentRequestResult, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return CreatePaymentRequestResult{}, InvalidArgument("Idempotency-Key header is required")
	}
	if len(idempotencyKey) > 160 {
		return CreatePaymentRequestResult{}, InvalidArgument("Idempotency-Key header is too long")
	}

	normalized, err := normalizePaymentRequestInput(input)
	if err != nil {
		return CreatePaymentRequestResult{}, err
	}
	bodyHash, err := canonicalHash(normalized)
	if err != nil {
		return CreatePaymentRequestResult{}, err
	}
	idempotencyScope := businessID + "|payment_request.create|" + idempotencyKey

	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	if record, ok := s.idempotency[idempotencyScope]; ok {
		if record.BodyHash != bodyHash {
			return CreatePaymentRequestResult{}, Conflict("idempotency key was already used with a different payment request body")
		}
		paymentRequest, ok := s.paymentRequests[record.PaymentRequestID]
		if !ok {
			return CreatePaymentRequestResult{}, NotFound("idempotent payment request no longer exists")
		}
		return CreatePaymentRequestResult{PaymentRequest: paymentRequest, IdempotentReplayed: true}, nil
	}

	wallet, ok := s.wallets[normalized.WalletID]
	if !ok || wallet.BusinessID != businessID || wallet.ArchivedAt != nil {
		return CreatePaymentRequestResult{}, NotFound("wallet not found")
	}
	if wallet.Chain != normalized.Chain {
		return CreatePaymentRequestResult{}, InvalidArgument("payment request chain must match wallet chain")
	}

	paymentRequest := PaymentRequest{
		ID:                 newID("prq"),
		BusinessID:         businessID,
		WalletID:           wallet.ID,
		CustomerID:         normalized.CustomerID,
		InvoiceID:          normalized.InvoiceID,
		OrderID:            normalized.OrderID,
		Amount:             normalized.Amount,
		Token:              normalized.Token,
		Chain:              normalized.Chain,
		DestinationAddress: wallet.Address,
		ExpiresAt:          normalized.ExpiresAt,
		Metadata:           normalized.Metadata,
		Status:             PaymentStatusAwaitingPayment,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	s.paymentRequests[paymentRequest.ID] = paymentRequest
	s.idempotency[idempotencyScope] = idempotencyRecord{
		BodyHash:         bodyHash,
		PaymentRequestID: paymentRequest.ID,
	}
	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   businessID,
		ActorType:    "api_key",
		ActorID:      actorID,
		Action:       "payment_request.created",
		ResourceType: "payment_request",
		ResourceID:   paymentRequest.ID,
		Metadata: map[string]string{
			"status":          paymentRequest.Status,
			"idempotency_key": idempotencyKey,
		},
		CreatedAt: now,
	})

	return CreatePaymentRequestResult{PaymentRequest: paymentRequest}, nil
}

func (s *MemoryStore) GetPaymentRequest(_ context.Context, businessID, paymentRequestID string) (PaymentRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	paymentRequest, ok := s.paymentRequests[paymentRequestID]
	if !ok || paymentRequest.BusinessID != businessID {
		return PaymentRequest{}, NotFound("payment request not found")
	}
	return paymentRequest, nil
}

func (s *MemoryStore) ListPaymentRequests(_ context.Context, businessID string) ([]PaymentRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	requests := make([]PaymentRequest, 0)
	for _, request := range s.paymentRequests {
		if request.BusinessID == businessID {
			requests = append(requests, request)
		}
	}
	sort.Slice(requests, func(i, j int) bool {
		return requests[i].CreatedAt.After(requests[j].CreatedAt)
	})
	return requests, nil
}

func (s *MemoryStore) ListAuditLogs(_ context.Context, businessID string) ([]AuditLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	logs := make([]AuditLog, 0)
	for _, log := range s.auditLogs {
		if log.BusinessID == businessID {
			logs = append(logs, log)
		}
	}
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].CreatedAt.After(logs[j].CreatedAt)
	})
	return logs, nil
}

func (s *MemoryStore) appendAuditLocked(log AuditLog) {
	s.auditLogs[log.ID] = log
}

func normalizePaymentRequestInput(input CreatePaymentRequestInput) (CreatePaymentRequestInput, error) {
	input.WalletID = strings.TrimSpace(input.WalletID)
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.InvoiceID = strings.TrimSpace(input.InvoiceID)
	input.OrderID = strings.TrimSpace(input.OrderID)
	input.Amount = strings.TrimSpace(input.Amount)
	input.Token = strings.ToUpper(strings.TrimSpace(input.Token))
	input.Chain = strings.ToLower(strings.TrimSpace(input.Chain))
	input.ExpiresAt = input.ExpiresAt.UTC()

	if input.WalletID == "" {
		return input, InvalidArgument("wallet_id is required")
	}
	if input.CustomerID == "" {
		return input, InvalidArgument("customer_id is required")
	}
	if input.InvoiceID == "" {
		return input, InvalidArgument("invoice_id is required")
	}
	if err := validateAmount(input.Amount); err != nil {
		return input, err
	}
	if input.Token != TokenUSDC {
		return input, InvalidArgument("only USDC payment requests are supported in v1")
	}
	if input.Chain != ChainSolana {
		return input, InvalidArgument("only solana payment requests are supported in v1")
	}
	if input.ExpiresAt.IsZero() {
		return input, InvalidArgument("expires_at is required")
	}
	if !input.ExpiresAt.After(time.Now().UTC()) {
		return input, InvalidArgument("expires_at must be in the future")
	}
	if input.Metadata == nil {
		input.Metadata = map[string]string{}
	}
	return input, nil
}

func validateAmount(amount string) error {
	value, ok := new(big.Rat).SetString(amount)
	if !ok {
		return InvalidArgument("amount must be a decimal number")
	}
	if value.Sign() <= 0 {
		return InvalidArgument("amount must be greater than zero")
	}
	if strings.Contains(amount, "e") || strings.Contains(amount, "E") {
		return InvalidArgument("amount must not use exponent notation")
	}
	parts := strings.Split(amount, ".")
	if len(parts) > 2 || len(parts[0]) == 0 {
		return InvalidArgument("amount must be a decimal number")
	}
	if len(parts) == 2 && len(parts[1]) > 6 {
		return InvalidArgument("USDC amount supports at most 6 decimal places")
	}
	return nil
}

func validateAddress(address string) error {
	if address == "" {
		return InvalidArgument("wallet address is required")
	}
	if len(address) < 32 || len(address) > 64 {
		return InvalidArgument("wallet address must look like a Solana address")
	}
	for _, r := range address {
		if strings.ContainsRune(" 0OIl", r) {
			return InvalidArgument("wallet address contains invalid base58 characters")
		}
	}
	return nil
}

func canonicalHash(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func newAPIKey() (string, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	return "twins_test_" + token, nil
}

func newID(prefix string) string {
	token, err := randomToken(12)
	if err != nil {
		panic(err)
	}
	return prefix + "_" + token
}

func randomToken(byteCount int) (string, error) {
	bytes := make([]byte, byteCount)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes)
	return strings.ToLower(encoded), nil
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

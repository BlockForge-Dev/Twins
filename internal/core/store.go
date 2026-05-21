package core

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu sync.RWMutex

	businesses map[string]Business
	apiKeys    map[string]APIKey
	apiKeyHash map[string]string
	users      map[string]User

	wallets map[string]Wallet

	paymentRequests        map[string]PaymentRequest
	stablecoinTransactions map[string]StablecoinTransaction
	transactionBySignature map[string]string
	transactionMatches     map[string]TransactionMatch
	exceptions             map[string]PaymentException
	receiptEvents          map[string]ReceiptEvent
	webhookSubscriptions   map[string]WebhookSubscription
	webhookDeliveries      map[string]WebhookDelivery
	reconciliationRuns     map[string]ReconciliationRun
	walletSnapshots        map[string]WalletBalanceSnapshot
	settlementRows         map[string]SettlementReportRow
	exports                map[string]ExportRecord
	idempotency            map[string]idempotencyRecord

	auditLogs        map[string]AuditLog
	accessLogs       map[string]AccessLog
	incidents        map[string]Incident
	securityPolicies map[string]SecurityPolicy
	designPartners   map[string]DesignPartner
	betaEvidence     map[string]BetaEvidence
	httpClient       *http.Client

	persistencePath string
	persistenceErr  string
}

type idempotencyRecord struct {
	BodyHash         string
	PaymentRequestID string
}

type persistentSnapshot struct {
	Version                int                              `json:"version"`
	Businesses             map[string]Business              `json:"businesses"`
	APIKeys                map[string]persistentAPIKey      `json:"api_keys"`
	Users                  map[string]User                  `json:"users"`
	Wallets                map[string]Wallet                `json:"wallets"`
	PaymentRequests        map[string]PaymentRequest        `json:"payment_requests"`
	StablecoinTransactions map[string]StablecoinTransaction `json:"stablecoin_transactions"`
	TransactionMatches     map[string]TransactionMatch      `json:"transaction_matches"`
	Exceptions             map[string]PaymentException      `json:"exceptions"`
	ReceiptEvents          map[string]ReceiptEvent          `json:"receipt_events"`
	WebhookSubscriptions   map[string]WebhookSubscription   `json:"webhook_subscriptions"`
	WebhookDeliveries      map[string]WebhookDelivery       `json:"webhook_deliveries"`
	ReconciliationRuns     map[string]ReconciliationRun     `json:"reconciliation_runs"`
	WalletSnapshots        map[string]WalletBalanceSnapshot `json:"wallet_snapshots"`
	SettlementRows         map[string]SettlementReportRow   `json:"settlement_rows"`
	Exports                map[string]ExportRecord          `json:"exports"`
	Idempotency            map[string]idempotencyRecord     `json:"idempotency"`
	AuditLogs              map[string]AuditLog              `json:"audit_logs"`
	AccessLogs             map[string]AccessLog             `json:"access_logs"`
	Incidents              map[string]Incident              `json:"incidents"`
	SecurityPolicies       map[string]SecurityPolicy        `json:"security_policies"`
	DesignPartners         map[string]DesignPartner         `json:"design_partners"`
	BetaEvidence           map[string]BetaEvidence          `json:"beta_evidence"`
}

type persistentAPIKey struct {
	ID         string     `json:"id"`
	BusinessID string     `json:"business_id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	SecretHash string     `json:"secret_hash"`
	Scopes     []string   `json:"scopes"`
	CreatedBy  string     `json:"created_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		businesses:             make(map[string]Business),
		apiKeys:                make(map[string]APIKey),
		apiKeyHash:             make(map[string]string),
		users:                  make(map[string]User),
		wallets:                make(map[string]Wallet),
		paymentRequests:        make(map[string]PaymentRequest),
		stablecoinTransactions: make(map[string]StablecoinTransaction),
		transactionBySignature: make(map[string]string),
		transactionMatches:     make(map[string]TransactionMatch),
		exceptions:             make(map[string]PaymentException),
		receiptEvents:          make(map[string]ReceiptEvent),
		webhookSubscriptions:   make(map[string]WebhookSubscription),
		webhookDeliveries:      make(map[string]WebhookDelivery),
		reconciliationRuns:     make(map[string]ReconciliationRun),
		walletSnapshots:        make(map[string]WalletBalanceSnapshot),
		settlementRows:         make(map[string]SettlementReportRow),
		exports:                make(map[string]ExportRecord),
		idempotency:            make(map[string]idempotencyRecord),
		auditLogs:              make(map[string]AuditLog),
		accessLogs:             make(map[string]AccessLog),
		incidents:              make(map[string]Incident),
		securityPolicies:       make(map[string]SecurityPolicy),
		designPartners:         make(map[string]DesignPartner),
		betaEvidence:           make(map[string]BetaEvidence),
		httpClient:             &http.Client{Timeout: 2 * time.Second},
	}
}

func NewPersistentStore(path string) (*MemoryStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, InvalidArgument("persistence path is required")
	}

	store := NewMemoryStore()
	store.persistencePath = path

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return store, nil
	}

	var snapshot persistentSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}

	store.loadSnapshot(snapshot)
	store.rebuildIndexesLocked()
	return store, nil
}

func (s *MemoryStore) CreateBusiness(_ context.Context, input CreateBusinessInput) (CreateBusinessResult, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return CreateBusinessResult{}, InvalidArgument("business name is required")
	}
	ownerEmail := strings.ToLower(strings.TrimSpace(input.OwnerEmail))
	if ownerEmail == "" {
		ownerEmail = "owner@" + strings.ReplaceAll(strings.ToLower(name), " ", "-") + ".local"
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
	owner := User{
		ID:         newID("usr"),
		BusinessID: business.ID,
		Email:      ownerEmail,
		Name:       "Owner",
		Role:       UserRoleOwner,
		Status:     UserStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	apiKey := APIKey{
		ID:         newID("key"),
		BusinessID: business.ID,
		Name:       "Default API key",
		Prefix:     rawKey[:min(18, len(rawKey))],
		SecretHash: keyHash,
		Scopes:     allAPIScopes(),
		CreatedBy:  owner.ID,
		CreatedAt:  now,
	}
	policy := defaultSecurityPolicy(business.ID, now)

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()

	s.businesses[business.ID] = business
	s.users[owner.ID] = owner
	s.apiKeys[apiKey.ID] = apiKey
	s.apiKeyHash[keyHash] = apiKey.ID
	s.securityPolicies[business.ID] = policy
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
		Action:       "user.created",
		ResourceType: "user",
		ResourceID:   owner.ID,
		Metadata:     map[string]string{"role": owner.Role},
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

	return CreateBusinessResult{Business: business, APIKey: rawKey, Owner: owner}, nil
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

func (s *MemoryStore) CreateAPIKey(_ context.Context, businessID, actorID string, input CreateAPIKeyInput) (CreateAPIKeyResult, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return CreateAPIKeyResult{}, InvalidArgument("api key name is required")
	}
	scopes, err := normalizeAPIScopes(input.Scopes)
	if err != nil {
		return CreateAPIKeyResult{}, err
	}

	now := time.Now().UTC()
	rawKey, err := newAPIKey()
	if err != nil {
		return CreateAPIKeyResult{}, err
	}
	keyHash := hashSecret(rawKey)
	apiKey := APIKey{
		ID:         newID("key"),
		BusinessID: businessID,
		Name:       name,
		Prefix:     rawKey[:min(18, len(rawKey))],
		SecretHash: keyHash,
		Scopes:     scopes,
		CreatedBy:  actorID,
		CreatedAt:  now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()

	s.apiKeys[apiKey.ID] = apiKey
	s.apiKeyHash[keyHash] = apiKey.ID
	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   businessID,
		ActorType:    "api_key",
		ActorID:      actorID,
		Action:       "api_key.created",
		ResourceType: "api_key",
		ResourceID:   apiKey.ID,
		Metadata: map[string]string{
			"prefix": apiKey.Prefix,
			"scopes": strings.Join(apiKey.Scopes, ","),
		},
		CreatedAt: now,
	})

	return CreateAPIKeyResult{APIKey: apiKey, Secret: rawKey}, nil
}

func (s *MemoryStore) ListAPIKeys(_ context.Context, businessID string) ([]APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]APIKey, 0)
	for _, apiKey := range s.apiKeys {
		if apiKey.BusinessID == businessID {
			keys = append(keys, apiKey)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].CreatedAt.After(keys[j].CreatedAt)
	})
	return keys, nil
}

func (s *MemoryStore) RevokeAPIKey(_ context.Context, businessID, actorID, apiKeyID string) (APIKey, error) {
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()

	apiKey, ok := s.apiKeys[apiKeyID]
	if !ok || apiKey.BusinessID != businessID {
		return APIKey{}, NotFound("api key not found")
	}
	if apiKey.RevokedAt == nil {
		apiKey.RevokedAt = &now
		s.apiKeys[apiKey.ID] = apiKey
		s.appendAuditLocked(AuditLog{
			ID:           newID("aud"),
			BusinessID:   businessID,
			ActorType:    "api_key",
			ActorID:      actorID,
			Action:       "api_key.revoked",
			ResourceType: "api_key",
			ResourceID:   apiKey.ID,
			CreatedAt:    now,
		})
	}
	return apiKey, nil
}

func (s *MemoryStore) CreateUser(_ context.Context, businessID, actorID string, input CreateUserInput) (User, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	name := strings.TrimSpace(input.Name)
	role := strings.ToLower(strings.TrimSpace(input.Role))
	if email == "" || !strings.Contains(email, "@") {
		return User{}, InvalidArgument("valid user email is required")
	}
	if !validUserRole(role) {
		return User{}, InvalidArgument("user role must be owner, admin, operator, or viewer")
	}

	now := time.Now().UTC()
	user := User{
		ID:         newID("usr"),
		BusinessID: businessID,
		Email:      email,
		Name:       name,
		Role:       role,
		Status:     UserStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()

	for _, existing := range s.users {
		if existing.BusinessID == businessID && existing.Email == email {
			return User{}, Conflict("user email is already registered for this business")
		}
	}
	s.users[user.ID] = user
	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   businessID,
		ActorType:    "api_key",
		ActorID:      actorID,
		Action:       "user.created",
		ResourceType: "user",
		ResourceID:   user.ID,
		Metadata:     map[string]string{"role": user.Role},
		CreatedAt:    now,
	})

	return user, nil
}

func (s *MemoryStore) ListUsers(_ context.Context, businessID string) ([]User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]User, 0)
	for _, user := range s.users {
		if user.BusinessID == businessID {
			users = append(users, user)
		}
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].CreatedAt.Before(users[j].CreatedAt)
	})
	return users, nil
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
	defer s.mustPersistLocked()

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
	defer s.mustPersistLocked()

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
	s.createReceiptEventLocked(ReceiptEvent{
		BusinessID:       businessID,
		PaymentRequestID: paymentRequest.ID,
		Type:             ReceiptEventPaymentRequestCreated,
		Status:           paymentRequest.Status,
		Description:      "payment request created and awaiting stablecoin payment",
		Metadata: map[string]string{
			"amount":     paymentRequest.Amount,
			"token":      paymentRequest.Token,
			"chain":      paymentRequest.Chain,
			"invoice_id": paymentRequest.InvoiceID,
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

func (s *MemoryStore) IngestStablecoinTransaction(_ context.Context, businessID, actorID string, input IngestStablecoinTransactionInput) (IngestStablecoinTransactionResult, error) {
	normalized, err := normalizeStablecoinTransactionInput(input)
	if err != nil {
		return IngestStablecoinTransactionResult{}, err
	}

	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()

	wallet, ok := s.findWalletForTransactionLocked(businessID, normalized.Chain, normalized.DestinationOwner, normalized.DestinationAddress)
	if !ok {
		return IngestStablecoinTransactionResult{}, NotFound("destination wallet is not registered for this business")
	}

	signatureScope := transactionSignatureScope(businessID, normalized.Chain, normalized.Signature)
	if existingID, ok := s.transactionBySignature[signatureScope]; ok {
		existing := s.stablecoinTransactions[existingID]
		if existing.AmountAtomic != normalized.AmountAtomic ||
			existing.DestinationAddress != normalized.DestinationAddress ||
			existing.DestinationOwner != normalized.DestinationOwner ||
			existing.Mint != normalized.Mint {
			return IngestStablecoinTransactionResult{}, Conflict("transaction signature was already ingested with different evidence")
		}
		return IngestStablecoinTransactionResult{StablecoinTransaction: existing, DuplicateReplayed: true}, nil
	}

	status := TransactionStatusPendingFinality
	if normalized.ConfirmationStatus == "finalized" {
		status = TransactionStatusConfirmedOnchain
	}

	transaction := StablecoinTransaction{
		ID:                 newID("txn"),
		BusinessID:         businessID,
		WalletID:           wallet.ID,
		Chain:              normalized.Chain,
		Signature:          normalized.Signature,
		Slot:               normalized.Slot,
		BlockTime:          normalized.BlockTime,
		ConfirmationStatus: normalized.ConfirmationStatus,
		SourceAddress:      normalized.SourceAddress,
		SourceOwner:        normalized.SourceOwner,
		DestinationAddress: normalized.DestinationAddress,
		DestinationOwner:   normalized.DestinationOwner,
		Token:              normalized.Token,
		Mint:               normalized.Mint,
		Amount:             normalized.Amount,
		AmountAtomic:       normalized.AmountAtomic,
		Decimals:           normalized.Decimals,
		Status:             status,
		DetectedAt:         now,
		CreatedAt:          now,
	}

	s.stablecoinTransactions[transaction.ID] = transaction
	s.transactionBySignature[signatureScope] = transaction.ID
	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   businessID,
		ActorType:    "api_key",
		ActorID:      actorID,
		Action:       "stablecoin_transaction.ingested",
		ResourceType: "stablecoin_transaction",
		ResourceID:   transaction.ID,
		Metadata: map[string]string{
			"signature": transaction.Signature,
			"status":    transaction.Status,
			"wallet_id": wallet.ID,
		},
		CreatedAt: now,
	})

	match, exception := s.matchTransactionLocked(businessID, transaction.ID, now)
	transaction = s.stablecoinTransactions[transaction.ID]

	return IngestStablecoinTransactionResult{
		StablecoinTransaction: transaction,
		TransactionMatch:      match,
		Exception:             exception,
	}, nil
}

func (s *MemoryStore) ListStablecoinTransactions(_ context.Context, businessID string) ([]StablecoinTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	transactions := make([]StablecoinTransaction, 0)
	for _, transaction := range s.stablecoinTransactions {
		if transaction.BusinessID == businessID {
			transactions = append(transactions, transaction)
		}
	}
	sort.Slice(transactions, func(i, j int) bool {
		return transactions[i].CreatedAt.After(transactions[j].CreatedAt)
	})
	return transactions, nil
}

func (s *MemoryStore) ListTransactionMatches(_ context.Context, businessID string) ([]TransactionMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	matches := make([]TransactionMatch, 0)
	for _, match := range s.transactionMatches {
		if match.BusinessID == businessID {
			matches = append(matches, match)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].CreatedAt.After(matches[j].CreatedAt)
	})
	return matches, nil
}

func (s *MemoryStore) ListExceptions(_ context.Context, businessID string) ([]PaymentException, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exceptions := make([]PaymentException, 0)
	for _, exception := range s.exceptions {
		if exception.BusinessID == businessID {
			exceptions = append(exceptions, exception)
		}
	}
	sort.Slice(exceptions, func(i, j int) bool {
		return exceptions[i].CreatedAt.After(exceptions[j].CreatedAt)
	})
	return exceptions, nil
}

func (s *MemoryStore) ListReceiptEvents(_ context.Context, businessID string) ([]ReceiptEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := make([]ReceiptEvent, 0)
	for _, event := range s.receiptEvents {
		if event.BusinessID == businessID {
			events = append(events, event)
		}
	}
	sortReceiptEventsChronologically(events)
	return events, nil
}

func (s *MemoryStore) GetReceipt(_ context.Context, businessID, paymentRequestID string) (Receipt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	paymentRequest, ok := s.paymentRequests[paymentRequestID]
	if !ok || paymentRequest.BusinessID != businessID {
		return Receipt{}, NotFound("receipt not found")
	}
	return Receipt{
		PaymentRequest: paymentRequest,
		Events:         s.receiptEventsForRequestLocked(businessID, paymentRequestID),
	}, nil
}

func (s *MemoryStore) GetPublicReceipt(_ context.Context, paymentRequestID string) (Receipt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	paymentRequest, ok := s.paymentRequests[paymentRequestID]
	if !ok {
		return Receipt{}, NotFound("receipt not found")
	}
	return Receipt{
		PaymentRequest: paymentRequest,
		Events:         s.receiptEventsForRequestLocked(paymentRequest.BusinessID, paymentRequestID),
	}, nil
}

func (s *MemoryStore) CreateWebhookSubscription(_ context.Context, businessID, actorID string, input CreateWebhookSubscriptionInput) (CreateWebhookSubscriptionResult, error) {
	endpoint := strings.TrimSpace(input.URL)
	parsedURL, err := url.Parse(endpoint)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return CreateWebhookSubscriptionResult{}, InvalidArgument("webhook url must be an absolute http or https URL")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return CreateWebhookSubscriptionResult{}, InvalidArgument("webhook url must use http or https")
	}

	eventTypes, err := normalizeWebhookEventTypes(input.EventTypes)
	if err != nil {
		return CreateWebhookSubscriptionResult{}, err
	}
	secret := strings.TrimSpace(input.Secret)
	generatedSecret := ""
	if secret == "" {
		token, err := randomToken(24)
		if err != nil {
			return CreateWebhookSubscriptionResult{}, err
		}
		secret = "whsec_" + token
		generatedSecret = secret
	}

	now := time.Now().UTC()
	subscription := WebhookSubscription{
		ID:               newID("whs"),
		BusinessID:       businessID,
		URL:              endpoint,
		SecretCiphertext: protectSecret(secret, businessID),
		EventTypes:       eventTypes,
		Enabled:          true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()

	s.webhookSubscriptions[subscription.ID] = subscription
	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   businessID,
		ActorType:    "api_key",
		ActorID:      actorID,
		Action:       "webhook_subscription.created",
		ResourceType: "webhook_subscription",
		ResourceID:   subscription.ID,
		Metadata: map[string]string{
			"url": endpoint,
		},
		CreatedAt: now,
	})

	return CreateWebhookSubscriptionResult{
		WebhookSubscription: subscription,
		SigningSecret:       generatedSecret,
	}, nil
}

func (s *MemoryStore) ListWebhookSubscriptions(_ context.Context, businessID string) ([]WebhookSubscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	subscriptions := make([]WebhookSubscription, 0)
	for _, subscription := range s.webhookSubscriptions {
		if subscription.BusinessID == businessID {
			subscriptions = append(subscriptions, subscription)
		}
	}
	sort.Slice(subscriptions, func(i, j int) bool {
		return subscriptions[i].CreatedAt.After(subscriptions[j].CreatedAt)
	})
	return subscriptions, nil
}

func (s *MemoryStore) ListWebhookDeliveries(_ context.Context, businessID string) ([]WebhookDelivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	deliveries := make([]WebhookDelivery, 0)
	for _, delivery := range s.webhookDeliveries {
		if delivery.BusinessID == businessID {
			deliveries = append(deliveries, delivery)
		}
	}
	sort.Slice(deliveries, func(i, j int) bool {
		return deliveries[i].CreatedAt.After(deliveries[j].CreatedAt)
	})
	return deliveries, nil
}

func (s *MemoryStore) ReplayWebhookDelivery(_ context.Context, businessID, actorID, deliveryID string) (WebhookDelivery, error) {
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()

	delivery, ok := s.webhookDeliveries[deliveryID]
	if !ok || delivery.BusinessID != businessID {
		return WebhookDelivery{}, NotFound("webhook delivery not found")
	}

	delivery.Status = WebhookStatusPending
	delivery.LastError = ""
	delivery.NextAttemptAt = nil
	delivery.UpdatedAt = now
	s.webhookDeliveries[delivery.ID] = delivery
	s.attemptWebhookDeliveryLocked(delivery.ID, now)
	delivery = s.webhookDeliveries[delivery.ID]

	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   businessID,
		ActorType:    "api_key",
		ActorID:      actorID,
		Action:       "webhook_delivery.replayed",
		ResourceType: "webhook_delivery",
		ResourceID:   delivery.ID,
		Metadata: map[string]string{
			"status": delivery.Status,
		},
		CreatedAt: now,
	})

	return delivery, nil
}

func (s *MemoryStore) CreateReconciliationRun(_ context.Context, businessID, actorID string, input CreateReconciliationRunInput) (ReconciliationReport, error) {
	normalized, err := normalizeReconciliationRunInput(input)
	if err != nil {
		return ReconciliationReport{}, err
	}

	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()

	if normalized.WalletID != "" {
		wallet, ok := s.wallets[normalized.WalletID]
		if !ok || wallet.BusinessID != businessID || wallet.ArchivedAt != nil {
			return ReconciliationReport{}, NotFound("wallet not found")
		}
	}

	run := ReconciliationRun{
		ID:          newID("rec"),
		BusinessID:  businessID,
		Status:      ReconciliationStatusCompleted,
		PeriodStart: normalized.PeriodStart,
		PeriodEnd:   normalized.PeriodEnd,
		WalletID:    normalized.WalletID,
		CreatedAt:   now,
		CompletedAt: &now,
	}

	report := s.buildReconciliationReportLocked(run, now)
	run = report.ReconciliationRun
	s.reconciliationRuns[run.ID] = run
	for _, snapshot := range report.WalletSnapshots {
		s.walletSnapshots[snapshot.ID] = snapshot
	}
	for _, row := range report.Rows {
		s.settlementRows[row.ID] = row
	}

	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   businessID,
		ActorType:    "api_key",
		ActorID:      actorID,
		Action:       "reconciliation_run.created",
		ResourceType: "reconciliation_run",
		ResourceID:   run.ID,
		Metadata: map[string]string{
			"period_start": run.PeriodStart.Format(time.RFC3339),
			"period_end":   run.PeriodEnd.Format(time.RFC3339),
			"wallet_id":    run.WalletID,
		},
		CreatedAt: now,
	})

	return report, nil
}

func (s *MemoryStore) ListReconciliationRuns(_ context.Context, businessID string) ([]ReconciliationRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	runs := make([]ReconciliationRun, 0)
	for _, run := range s.reconciliationRuns {
		if run.BusinessID == businessID {
			runs = append(runs, run)
		}
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})
	return runs, nil
}

func (s *MemoryStore) GetReconciliationReport(_ context.Context, businessID, runID string) (ReconciliationReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	run, ok := s.reconciliationRuns[runID]
	if !ok || run.BusinessID != businessID {
		return ReconciliationReport{}, NotFound("reconciliation run not found")
	}

	return s.reconciliationReportForRunLocked(run), nil
}

func (s *MemoryStore) CreateExport(_ context.Context, businessID, actorID string, input CreateExportInput) (ExportRecord, error) {
	normalized, err := normalizeExportInput(input)
	if err != nil {
		return ExportRecord{}, err
	}

	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()

	run, ok := s.reconciliationRuns[normalized.ReconciliationRunID]
	if !ok || run.BusinessID != businessID {
		return ExportRecord{}, NotFound("reconciliation run not found")
	}
	report := s.reconciliationReportForRunLocked(run)

	content, fileName, err := renderSettlementExport(report, normalized.Format)
	if err != nil {
		return ExportRecord{}, err
	}

	export := ExportRecord{
		ID:                  newID("exp"),
		BusinessID:          businessID,
		ReconciliationRunID: run.ID,
		Type:                ExportTypeSettlementReport,
		Format:              normalized.Format,
		Status:              ExportStatusReady,
		FileName:            fileName,
		RowCount:            len(report.Rows),
		Content:             content,
		CreatedAt:           now,
	}
	s.exports[export.ID] = export
	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   businessID,
		ActorType:    "api_key",
		ActorID:      actorID,
		Action:       "export.created",
		ResourceType: "export",
		ResourceID:   export.ID,
		Metadata: map[string]string{
			"reconciliation_run_id": run.ID,
			"format":                export.Format,
			"row_count":             strconv.Itoa(export.RowCount),
		},
		CreatedAt: now,
	})

	return export, nil
}

func (s *MemoryStore) ListExports(_ context.Context, businessID string) ([]ExportRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exports := make([]ExportRecord, 0)
	for _, export := range s.exports {
		if export.BusinessID == businessID {
			exports = append(exports, export)
		}
	}
	sort.Slice(exports, func(i, j int) bool {
		return exports[i].CreatedAt.After(exports[j].CreatedAt)
	})
	return exports, nil
}

func (s *MemoryStore) GetExport(_ context.Context, businessID, exportID string) (ExportRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	export, ok := s.exports[exportID]
	if !ok || export.BusinessID != businessID {
		return ExportRecord{}, NotFound("export not found")
	}
	return export, nil
}

func (s *MemoryStore) ResolveException(_ context.Context, businessID, actorID, exceptionID string, input ResolveExceptionInput) (PaymentException, error) {
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return PaymentException{}, InvalidArgument("resolution reason is required")
	}

	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()

	exception, ok := s.exceptions[exceptionID]
	if !ok || exception.BusinessID != businessID {
		return PaymentException{}, NotFound("exception not found")
	}
	if exception.Status == ExceptionStatusResolved {
		return exception, nil
	}

	exception.Status = ExceptionStatusResolved
	exception.ResolutionReason = reason
	exception.ResolvedBy = actorID
	exception.ResolvedAt = &now
	s.exceptions[exception.ID] = exception

	if exception.PaymentRequestID != "" {
		paymentRequest := s.paymentRequests[exception.PaymentRequestID]
		paymentRequest.Status = PaymentStatusManuallyResolved
		paymentRequest.UpdatedAt = now
		s.paymentRequests[paymentRequest.ID] = paymentRequest
		s.createReceiptEventLocked(ReceiptEvent{
			BusinessID:              businessID,
			PaymentRequestID:        paymentRequest.ID,
			StablecoinTransactionID: exception.StablecoinTransactionID,
			ExceptionID:             exception.ID,
			Type:                    ReceiptEventExceptionResolved,
			Status:                  paymentRequest.Status,
			Description:             "payment exception manually resolved",
			Metadata: map[string]string{
				"exception_type":    exception.Type,
				"resolution_reason": reason,
			},
			CreatedAt: now,
		})
	}

	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   businessID,
		ActorType:    "api_key",
		ActorID:      actorID,
		Action:       "exception.resolved",
		ResourceType: "exception",
		ResourceID:   exception.ID,
		Metadata: map[string]string{
			"type": exception.Type,
		},
		CreatedAt: now,
	})

	return exception, nil
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

func (s *MemoryStore) RecordAccessLog(_ context.Context, input RecordAccessLogInput) {
	now := time.Now().UTC()
	log := AccessLog{
		ID:          newID("acl"),
		BusinessID:  input.BusinessID,
		APIKeyID:    input.APIKeyID,
		Method:      input.Method,
		Path:        input.Path,
		StatusCode:  input.StatusCode,
		RemoteAddr:  input.RemoteAddr,
		UserAgent:   input.UserAgent,
		DurationMS:  input.DurationMS,
		RateLimited: input.RateLimited,
		AccessedAt:  now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()
	s.accessLogs[log.ID] = log
}

func (s *MemoryStore) ListAccessLogs(_ context.Context, businessID string) ([]AccessLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	logs := make([]AccessLog, 0)
	for _, log := range s.accessLogs {
		if log.BusinessID == businessID {
			logs = append(logs, log)
		}
	}
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].AccessedAt.After(logs[j].AccessedAt)
	})
	return logs, nil
}

func (s *MemoryStore) CreateIncident(_ context.Context, businessID, actorID string, input CreateIncidentInput) (Incident, error) {
	title := strings.TrimSpace(input.Title)
	severity := strings.ToLower(strings.TrimSpace(input.Severity))
	description := strings.TrimSpace(input.Description)
	if title == "" {
		return Incident{}, InvalidArgument("incident title is required")
	}
	if severity == "" {
		severity = "medium"
	}
	if severity != "low" && severity != "medium" && severity != "high" && severity != "critical" {
		return Incident{}, InvalidArgument("incident severity must be low, medium, high, or critical")
	}

	now := time.Now().UTC()
	incident := Incident{
		ID:          newID("inc"),
		BusinessID:  businessID,
		Title:       title,
		Severity:    severity,
		Status:      IncidentStatusOpen,
		Description: description,
		CreatedBy:   actorID,
		CreatedAt:   now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()

	s.incidents[incident.ID] = incident
	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   businessID,
		ActorType:    "api_key",
		ActorID:      actorID,
		Action:       "incident.created",
		ResourceType: "incident",
		ResourceID:   incident.ID,
		Metadata:     map[string]string{"severity": severity},
		CreatedAt:    now,
	})
	return incident, nil
}

func (s *MemoryStore) ListIncidents(_ context.Context, businessID string) ([]Incident, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	incidents := make([]Incident, 0)
	for _, incident := range s.incidents {
		if incident.BusinessID == businessID {
			incidents = append(incidents, incident)
		}
	}
	sort.Slice(incidents, func(i, j int) bool {
		return incidents[i].CreatedAt.After(incidents[j].CreatedAt)
	})
	return incidents, nil
}

func (s *MemoryStore) ResolveIncident(_ context.Context, businessID, actorID, incidentID string, input ResolveIncidentInput) (Incident, error) {
	summary := strings.TrimSpace(input.Summary)
	if summary == "" {
		return Incident{}, InvalidArgument("resolution summary is required")
	}
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()

	incident, ok := s.incidents[incidentID]
	if !ok || incident.BusinessID != businessID {
		return Incident{}, NotFound("incident not found")
	}
	if incident.Status != IncidentStatusResolved {
		incident.Status = IncidentStatusResolved
		incident.ResolutionSummary = summary
		incident.ResolvedBy = actorID
		incident.ResolvedAt = &now
		s.incidents[incident.ID] = incident
		s.appendAuditLocked(AuditLog{
			ID:           newID("aud"),
			BusinessID:   businessID,
			ActorType:    "api_key",
			ActorID:      actorID,
			Action:       "incident.resolved",
			ResourceType: "incident",
			ResourceID:   incident.ID,
			CreatedAt:    now,
		})
	}
	return incident, nil
}

func (s *MemoryStore) GetSecurityPolicy(_ context.Context, businessID string) (SecurityPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policy, ok := s.securityPolicies[businessID]
	if !ok {
		return SecurityPolicy{}, NotFound("security policy not found")
	}
	return policy, nil
}

func (s *MemoryStore) UpdateSecurityPolicy(_ context.Context, businessID, actorID string, input UpdateSecurityPolicyInput) (SecurityPolicy, error) {
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()

	policy, ok := s.securityPolicies[businessID]
	if !ok {
		policy = defaultSecurityPolicy(businessID, now)
	}
	if input.RequireScopedAPIKeys != nil {
		policy.RequireScopedAPIKeys = *input.RequireScopedAPIKeys
	}
	if input.RateLimitPerMinute != nil {
		if *input.RateLimitPerMinute < 10 || *input.RateLimitPerMinute > 10000 {
			return SecurityPolicy{}, InvalidArgument("rate_limit_per_minute must be between 10 and 10000")
		}
		policy.RateLimitPerMinute = *input.RateLimitPerMinute
	}
	if input.DataRetentionDays != nil {
		if *input.DataRetentionDays < 30 {
			return SecurityPolicy{}, InvalidArgument("data_retention_days must be at least 30")
		}
		policy.DataRetentionDays = *input.DataRetentionDays
	}
	if input.AccessLogRetentionDays != nil {
		if *input.AccessLogRetentionDays < 7 {
			return SecurityPolicy{}, InvalidArgument("access_log_retention_days must be at least 7")
		}
		policy.AccessLogRetentionDays = *input.AccessLogRetentionDays
	}
	if input.WebhookRetentionDays != nil {
		if *input.WebhookRetentionDays < 7 {
			return SecurityPolicy{}, InvalidArgument("webhook_retention_days must be at least 7")
		}
		policy.WebhookRetentionDays = *input.WebhookRetentionDays
	}
	if input.IncidentRetentionDays != nil {
		if *input.IncidentRetentionDays < 30 {
			return SecurityPolicy{}, InvalidArgument("incident_retention_days must be at least 30")
		}
		policy.IncidentRetentionDays = *input.IncidentRetentionDays
	}
	policy.UpdatedAt = now
	s.securityPolicies[businessID] = policy
	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   businessID,
		ActorType:    "api_key",
		ActorID:      actorID,
		Action:       "security_policy.updated",
		ResourceType: "security_policy",
		ResourceID:   policy.ID,
		CreatedAt:    now,
	})
	return policy, nil
}

func (s *MemoryStore) CreateDesignPartner(_ context.Context, businessID, actorID string, input CreateDesignPartnerInput) (DesignPartner, error) {
	companyName := strings.TrimSpace(input.CompanyName)
	if companyName == "" {
		return DesignPartner{}, InvalidArgument("company_name is required")
	}
	status := strings.TrimSpace(strings.ToLower(input.Status))
	if status == "" {
		status = DesignPartnerStatusProspect
	}
	if !validDesignPartnerStatus(status) {
		return DesignPartner{}, InvalidArgument("unsupported design partner status: " + status)
	}
	if input.ExpectedMonthlyVolume < 0 {
		return DesignPartner{}, InvalidArgument("expected_monthly_volume cannot be negative")
	}

	now := time.Now().UTC()
	partner := DesignPartner{
		ID:                    newID("dsp"),
		BusinessID:            businessID,
		CompanyName:           companyName,
		Segment:               strings.TrimSpace(input.Segment),
		ContactName:           strings.TrimSpace(input.ContactName),
		ContactEmail:          strings.ToLower(strings.TrimSpace(input.ContactEmail)),
		UseCase:               strings.TrimSpace(input.UseCase),
		Status:                status,
		AgreedToTest:          input.AgreedToTest,
		PricingCommitment:     input.PricingCommitment,
		ExpectedMonthlyVolume: input.ExpectedMonthlyVolume,
		Notes:                 strings.TrimSpace(input.Notes),
		CreatedBy:             actorID,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()

	s.designPartners[partner.ID] = partner
	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   businessID,
		ActorType:    "api_key",
		ActorID:      actorID,
		Action:       "design_partner.created",
		ResourceType: "design_partner",
		ResourceID:   partner.ID,
		Metadata: map[string]string{
			"company_name": partner.CompanyName,
			"status":       partner.Status,
		},
		CreatedAt: now,
	})
	return partner, nil
}

func (s *MemoryStore) ListDesignPartners(_ context.Context, businessID string) ([]DesignPartner, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	partners := s.designPartnersForBusinessLocked(businessID)
	sort.Slice(partners, func(i, j int) bool {
		return partners[i].CreatedAt.After(partners[j].CreatedAt)
	})
	return partners, nil
}

func (s *MemoryStore) UpdateDesignPartner(_ context.Context, businessID, actorID, partnerID string, input UpdateDesignPartnerInput) (DesignPartner, error) {
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()

	partner, ok := s.designPartners[partnerID]
	if !ok || partner.BusinessID != businessID {
		return DesignPartner{}, NotFound("design partner not found")
	}
	if input.Status != nil {
		status := strings.TrimSpace(strings.ToLower(*input.Status))
		if !validDesignPartnerStatus(status) {
			return DesignPartner{}, InvalidArgument("unsupported design partner status: " + status)
		}
		partner.Status = status
	}
	if input.AgreedToTest != nil {
		partner.AgreedToTest = *input.AgreedToTest
	}
	if input.PricingCommitment != nil {
		partner.PricingCommitment = *input.PricingCommitment
	}
	if input.ExpectedMonthlyVolume != nil {
		if *input.ExpectedMonthlyVolume < 0 {
			return DesignPartner{}, InvalidArgument("expected_monthly_volume cannot be negative")
		}
		partner.ExpectedMonthlyVolume = *input.ExpectedMonthlyVolume
	}
	if input.Notes != nil {
		partner.Notes = strings.TrimSpace(*input.Notes)
	}
	partner.UpdatedAt = now
	s.designPartners[partner.ID] = partner
	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   businessID,
		ActorType:    "api_key",
		ActorID:      actorID,
		Action:       "design_partner.updated",
		ResourceType: "design_partner",
		ResourceID:   partner.ID,
		Metadata:     map[string]string{"status": partner.Status},
		CreatedAt:    now,
	})
	return partner, nil
}

func (s *MemoryStore) CreateBetaEvidence(_ context.Context, businessID, actorID string, input CreateBetaEvidenceInput) (BetaEvidence, error) {
	evidenceType := strings.TrimSpace(strings.ToLower(input.Type))
	if !validBetaEvidenceType(evidenceType) {
		return BetaEvidence{}, InvalidArgument("unsupported beta evidence type: " + evidenceType)
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return BetaEvidence{}, InvalidArgument("title is required")
	}

	now := time.Now().UTC()
	evidence := BetaEvidence{
		ID:                      newID("bev"),
		BusinessID:              businessID,
		DesignPartnerID:         strings.TrimSpace(input.DesignPartnerID),
		Type:                    evidenceType,
		Title:                   title,
		Description:             strings.TrimSpace(input.Description),
		PaymentRequestID:        strings.TrimSpace(input.PaymentRequestID),
		StablecoinTransactionID: strings.TrimSpace(input.StablecoinTransactionID),
		ExceptionID:             strings.TrimSpace(input.ExceptionID),
		Quote:                   strings.TrimSpace(input.Quote),
		CreatedBy:               actorID,
		CreatedAt:               now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.mustPersistLocked()

	if evidence.DesignPartnerID != "" {
		partner, ok := s.designPartners[evidence.DesignPartnerID]
		if !ok || partner.BusinessID != businessID {
			return BetaEvidence{}, NotFound("design partner not found")
		}
		if evidence.Type == BetaEvidenceTypePricingCommitment && !partner.PricingCommitment {
			partner.PricingCommitment = true
			partner.UpdatedAt = now
			s.designPartners[partner.ID] = partner
		}
	}
	if evidence.PaymentRequestID != "" {
		request, ok := s.paymentRequests[evidence.PaymentRequestID]
		if !ok || request.BusinessID != businessID {
			return BetaEvidence{}, NotFound("payment request not found")
		}
	}
	if evidence.StablecoinTransactionID != "" {
		transaction, ok := s.stablecoinTransactions[evidence.StablecoinTransactionID]
		if !ok || transaction.BusinessID != businessID {
			return BetaEvidence{}, NotFound("stablecoin transaction not found")
		}
	}
	if evidence.ExceptionID != "" {
		exception, ok := s.exceptions[evidence.ExceptionID]
		if !ok || exception.BusinessID != businessID {
			return BetaEvidence{}, NotFound("exception not found")
		}
	}

	s.betaEvidence[evidence.ID] = evidence
	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   businessID,
		ActorType:    "api_key",
		ActorID:      actorID,
		Action:       "beta_evidence.created",
		ResourceType: "beta_evidence",
		ResourceID:   evidence.ID,
		Metadata: map[string]string{
			"type":              evidence.Type,
			"design_partner_id": evidence.DesignPartnerID,
		},
		CreatedAt: now,
	})
	return evidence, nil
}

func (s *MemoryStore) ListBetaEvidence(_ context.Context, businessID string) ([]BetaEvidence, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	evidence := s.betaEvidenceForBusinessLocked(businessID)
	sort.Slice(evidence, func(i, j int) bool {
		return evidence[i].CreatedAt.After(evidence[j].CreatedAt)
	})
	return evidence, nil
}

func (s *MemoryStore) GetUsageMetrics(_ context.Context, businessID string) (UsageMetrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.usageMetricsLocked(businessID, time.Now().UTC()), nil
}

func (s *MemoryStore) GetPrivateBetaReport(_ context.Context, businessID string) (PrivateBetaReport, error) {
	now := time.Now().UTC()

	s.mu.RLock()
	defer s.mu.RUnlock()

	partners := s.designPartnersForBusinessLocked(businessID)
	evidence := s.betaEvidenceForBusinessLocked(businessID)
	sort.Slice(partners, func(i, j int) bool {
		return partners[i].CreatedAt.After(partners[j].CreatedAt)
	})
	sort.Slice(evidence, func(i, j int) bool {
		return evidence[i].CreatedAt.After(evidence[j].CreatedAt)
	})

	usage := s.usageMetricsLocked(businessID, now)
	onboarded := 0
	agreedToTest := 0
	pricingCommitments := 0
	for _, partner := range partners {
		if partner.Status == DesignPartnerStatusOnboarding || partner.Status == DesignPartnerStatusActive {
			onboarded++
		}
		if partner.AgreedToTest {
			agreedToTest++
		}
		if partner.PricingCommitment {
			pricingCommitments++
		}
	}

	partnersWithTransactions := make(map[string]bool)
	exceptionCases := 0
	testimonials := 0
	for _, item := range evidence {
		switch item.Type {
		case BetaEvidenceTypeRealTransaction:
			if item.DesignPartnerID != "" {
				partnersWithTransactions[item.DesignPartnerID] = true
			}
		case BetaEvidenceTypeExceptionCase:
			exceptionCases++
		case BetaEvidenceTypeTestimonial:
			testimonials++
		case BetaEvidenceTypePricingCommitment:
			if item.DesignPartnerID == "" {
				pricingCommitments++
			}
		}
	}

	remainingDesignPartners := remainingNeeded(5, onboarded)
	remainingTransactionPartners := remainingNeeded(2, len(partnersWithTransactions))
	remainingPricingCommitments := remainingNeeded(1, pricingCommitments)
	ready := remainingDesignPartners == 0 &&
		remainingTransactionPartners == 0 &&
		remainingPricingCommitments == 0 &&
		exceptionCases > 0

	return PrivateBetaReport{
		BusinessID:                         businessID,
		Usage:                              usage,
		DesignPartners:                     partners,
		Evidence:                           evidence,
		DesignPartnersOnboarded:            onboarded,
		DesignPartnersAgreedToTest:         agreedToTest,
		PartnersWithRealTransactions:       len(partnersWithTransactions),
		PricingCommitments:                 pricingCommitments,
		ExceptionCasesCollected:            exceptionCases,
		TestimonialsCollected:              testimonials,
		ReconciledBusinessRecords:          usage.ReconciledBusinessRecords,
		ReadyForPrivateBetaEvidence:        ready,
		RemainingDesignPartnersNeeded:      remainingDesignPartners,
		RemainingTransactionPartnersNeeded: remainingTransactionPartners,
		RemainingPricingCommitmentsNeeded:  remainingPricingCommitments,
		GeneratedAt:                        now,
	}, nil
}

func (s *MemoryStore) PersistenceStatus() (bool, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.persistencePath == "" {
		return false, "", nil
	}
	if s.persistenceErr != "" {
		return true, s.persistencePath, InvalidArgument(s.persistenceErr)
	}
	return true, s.persistencePath, nil
}

func (s *MemoryStore) loadSnapshot(snapshot persistentSnapshot) {
	if snapshot.Businesses != nil {
		s.businesses = snapshot.Businesses
	}
	if snapshot.APIKeys != nil {
		s.apiKeys = make(map[string]APIKey, len(snapshot.APIKeys))
		for id, apiKey := range snapshot.APIKeys {
			s.apiKeys[id] = APIKey{
				ID:         apiKey.ID,
				BusinessID: apiKey.BusinessID,
				Name:       apiKey.Name,
				Prefix:     apiKey.Prefix,
				SecretHash: apiKey.SecretHash,
				Scopes:     apiKey.Scopes,
				CreatedBy:  apiKey.CreatedBy,
				CreatedAt:  apiKey.CreatedAt,
				RevokedAt:  apiKey.RevokedAt,
			}
		}
	}
	if snapshot.Users != nil {
		s.users = snapshot.Users
	}
	if snapshot.Wallets != nil {
		s.wallets = snapshot.Wallets
	}
	if snapshot.PaymentRequests != nil {
		s.paymentRequests = snapshot.PaymentRequests
	}
	if snapshot.StablecoinTransactions != nil {
		s.stablecoinTransactions = snapshot.StablecoinTransactions
	}
	if snapshot.TransactionMatches != nil {
		s.transactionMatches = snapshot.TransactionMatches
	}
	if snapshot.Exceptions != nil {
		s.exceptions = snapshot.Exceptions
	}
	if snapshot.ReceiptEvents != nil {
		s.receiptEvents = snapshot.ReceiptEvents
	}
	if snapshot.WebhookSubscriptions != nil {
		s.webhookSubscriptions = snapshot.WebhookSubscriptions
	}
	if snapshot.WebhookDeliveries != nil {
		s.webhookDeliveries = snapshot.WebhookDeliveries
	}
	if snapshot.ReconciliationRuns != nil {
		s.reconciliationRuns = snapshot.ReconciliationRuns
	}
	if snapshot.WalletSnapshots != nil {
		s.walletSnapshots = snapshot.WalletSnapshots
	}
	if snapshot.SettlementRows != nil {
		s.settlementRows = snapshot.SettlementRows
	}
	if snapshot.Exports != nil {
		s.exports = snapshot.Exports
	}
	if snapshot.Idempotency != nil {
		s.idempotency = snapshot.Idempotency
	}
	if snapshot.AuditLogs != nil {
		s.auditLogs = snapshot.AuditLogs
	}
	if snapshot.AccessLogs != nil {
		s.accessLogs = snapshot.AccessLogs
	}
	if snapshot.Incidents != nil {
		s.incidents = snapshot.Incidents
	}
	if snapshot.SecurityPolicies != nil {
		s.securityPolicies = snapshot.SecurityPolicies
	}
	if snapshot.DesignPartners != nil {
		s.designPartners = snapshot.DesignPartners
	}
	if snapshot.BetaEvidence != nil {
		s.betaEvidence = snapshot.BetaEvidence
	}
}

func (s *MemoryStore) rebuildIndexesLocked() {
	s.apiKeyHash = make(map[string]string)
	for _, apiKey := range s.apiKeys {
		s.apiKeyHash[apiKey.SecretHash] = apiKey.ID
	}

	s.transactionBySignature = make(map[string]string)
	for _, transaction := range s.stablecoinTransactions {
		s.transactionBySignature[transactionSignatureScope(transaction.BusinessID, transaction.Chain, transaction.Signature)] = transaction.ID
	}
}

func (s *MemoryStore) mustPersistLocked() {
	_ = s.persistLocked()
}

func (s *MemoryStore) persistLocked() error {
	if s.persistencePath == "" {
		return nil
	}

	apiKeys := make(map[string]persistentAPIKey, len(s.apiKeys))
	for id, apiKey := range s.apiKeys {
		apiKeys[id] = persistentAPIKey{
			ID:         apiKey.ID,
			BusinessID: apiKey.BusinessID,
			Name:       apiKey.Name,
			Prefix:     apiKey.Prefix,
			SecretHash: apiKey.SecretHash,
			Scopes:     apiKey.Scopes,
			CreatedBy:  apiKey.CreatedBy,
			CreatedAt:  apiKey.CreatedAt,
			RevokedAt:  apiKey.RevokedAt,
		}
	}

	snapshot := persistentSnapshot{
		Version:                1,
		Businesses:             s.businesses,
		APIKeys:                apiKeys,
		Users:                  s.users,
		Wallets:                s.wallets,
		PaymentRequests:        s.paymentRequests,
		StablecoinTransactions: s.stablecoinTransactions,
		TransactionMatches:     s.transactionMatches,
		Exceptions:             s.exceptions,
		ReceiptEvents:          s.receiptEvents,
		WebhookSubscriptions:   s.webhookSubscriptions,
		WebhookDeliveries:      s.webhookDeliveries,
		ReconciliationRuns:     s.reconciliationRuns,
		WalletSnapshots:        s.walletSnapshots,
		SettlementRows:         s.settlementRows,
		Exports:                s.exports,
		Idempotency:            s.idempotency,
		AuditLogs:              s.auditLogs,
		AccessLogs:             s.accessLogs,
		Incidents:              s.incidents,
		SecurityPolicies:       s.securityPolicies,
		DesignPartners:         s.designPartners,
		BetaEvidence:           s.betaEvidence,
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		s.persistenceErr = err.Error()
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.persistencePath), 0o700); err != nil {
		s.persistenceErr = err.Error()
		return err
	}

	tmpPath := s.persistencePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		s.persistenceErr = err.Error()
		return err
	}
	if err := os.Rename(tmpPath, s.persistencePath); err != nil {
		s.persistenceErr = err.Error()
		return err
	}

	s.persistenceErr = ""
	return nil
}

func (s *MemoryStore) appendAuditLocked(log AuditLog) {
	s.auditLogs[log.ID] = log
}

func (s *MemoryStore) findWalletForTransactionLocked(businessID, chain, destinationOwner, destinationAddress string) (Wallet, bool) {
	for _, wallet := range s.wallets {
		if wallet.BusinessID != businessID || wallet.Chain != chain || wallet.ArchivedAt != nil {
			continue
		}
		if wallet.Address == destinationOwner || wallet.Address == destinationAddress {
			return wallet, true
		}
	}
	return Wallet{}, false
}

func (s *MemoryStore) matchTransactionLocked(businessID, transactionID string, now time.Time) (*TransactionMatch, *PaymentException) {
	transaction := s.stablecoinTransactions[transactionID]
	if transaction.BusinessID != businessID || transaction.Status != TransactionStatusConfirmedOnchain {
		return nil, nil
	}

	paymentTime := transaction.DetectedAt
	if transaction.BlockTime != nil {
		paymentTime = time.Unix(*transaction.BlockTime, 0).UTC()
	}

	activeCandidates := make([]PaymentRequest, 0)
	expiredCandidates := make([]PaymentRequest, 0)
	activeExact := make([]PaymentRequest, 0)
	expiredExact := make([]PaymentRequest, 0)

	for _, request := range s.paymentRequests {
		if request.BusinessID != businessID ||
			request.WalletID != transaction.WalletID ||
			request.Chain != transaction.Chain ||
			request.Token != transaction.Token ||
			request.Status != PaymentStatusAwaitingPayment {
			continue
		}

		compare, err := compareAmountStrings(transaction.Amount, request.Amount)
		if err != nil {
			continue
		}

		if paymentTime.After(request.ExpiresAt) {
			expiredCandidates = append(expiredCandidates, request)
			if compare == 0 {
				expiredExact = append(expiredExact, request)
			}
			continue
		}

		activeCandidates = append(activeCandidates, request)
		if compare == 0 {
			activeExact = append(activeExact, request)
		}
	}

	switch {
	case len(activeExact) == 1:
		match := s.createTransactionMatchLocked(activeExact[0], transaction, MatchStatusConfirmed, "amount, token, chain, wallet, and finality matched", now)
		s.updatePaymentRequestStatusLocked(activeExact[0].ID, PaymentStatusConfirmed, now)
		s.updateTransactionStatusLocked(transaction.ID, TransactionStatusMatchedToRequest)
		s.recordPaymentReceiptEventsLocked(activeExact[0], transaction, match, nil, PaymentStatusConfirmed, now)
		return match, nil
	case len(activeExact) > 1:
		exception := s.createExceptionLocked(businessID, "", transaction.ID, ExceptionTypeAmbiguousMatch, "high", "multiple active payment requests match this transaction amount", now)
		return nil, exception
	case len(activeCandidates) == 1:
		request := activeCandidates[0]
		compare, err := compareAmountStrings(transaction.Amount, request.Amount)
		if err != nil {
			return nil, nil
		}
		if compare < 0 {
			match := s.createTransactionMatchLocked(request, transaction, MatchStatusUnderpaid, "transaction amount is lower than payment request amount", now)
			exception := s.createExceptionLocked(businessID, request.ID, transaction.ID, ExceptionTypeUnderpaid, "high", "transaction amount is lower than expected", now)
			s.updatePaymentRequestStatusLocked(request.ID, PaymentStatusUnderpaid, now)
			s.updateTransactionStatusLocked(transaction.ID, TransactionStatusMatchedToRequest)
			s.recordPaymentReceiptEventsLocked(request, transaction, match, exception, PaymentStatusUnderpaid, now)
			return match, exception
		}
		if compare > 0 {
			match := s.createTransactionMatchLocked(request, transaction, MatchStatusOverpaid, "transaction amount is higher than payment request amount", now)
			exception := s.createExceptionLocked(businessID, request.ID, transaction.ID, ExceptionTypeOverpaid, "medium", "transaction amount is higher than expected", now)
			s.updatePaymentRequestStatusLocked(request.ID, PaymentStatusOverpaid, now)
			s.updateTransactionStatusLocked(transaction.ID, TransactionStatusMatchedToRequest)
			s.recordPaymentReceiptEventsLocked(request, transaction, match, exception, PaymentStatusOverpaid, now)
			return match, exception
		}
	case len(activeCandidates) > 1:
		exception := s.createExceptionLocked(businessID, "", transaction.ID, ExceptionTypeAmbiguousMatch, "high", "multiple active payment requests could match this transaction", now)
		return nil, exception
	case len(expiredExact) == 1:
		request := expiredExact[0]
		match := s.createTransactionMatchLocked(request, transaction, MatchStatusExpired, "transaction matched after payment request expiry", now)
		exception := s.createExceptionLocked(businessID, request.ID, transaction.ID, ExceptionTypeExpired, "medium", "transaction arrived after payment request expiry", now)
		s.updatePaymentRequestStatusLocked(request.ID, PaymentStatusExpired, now)
		s.updateTransactionStatusLocked(transaction.ID, TransactionStatusMatchedToRequest)
		s.recordPaymentReceiptEventsLocked(request, transaction, match, exception, PaymentStatusExpired, now)
		return match, exception
	case len(expiredExact) > 1 || len(expiredCandidates) > 1:
		exception := s.createExceptionLocked(businessID, "", transaction.ID, ExceptionTypeAmbiguousMatch, "medium", "multiple expired payment requests could match this transaction", now)
		return nil, exception
	case len(expiredCandidates) == 1:
		request := expiredCandidates[0]
		match := s.createTransactionMatchLocked(request, transaction, MatchStatusExpired, "transaction arrived after the only candidate payment request expired", now)
		exception := s.createExceptionLocked(businessID, request.ID, transaction.ID, ExceptionTypeExpired, "medium", "transaction arrived after payment request expiry", now)
		s.updatePaymentRequestStatusLocked(request.ID, PaymentStatusExpired, now)
		s.updateTransactionStatusLocked(transaction.ID, TransactionStatusMatchedToRequest)
		s.recordPaymentReceiptEventsLocked(request, transaction, match, exception, PaymentStatusExpired, now)
		return match, exception
	default:
		exception := s.createExceptionLocked(businessID, "", transaction.ID, ExceptionTypeOrphan, "medium", "incoming transaction has no matching payment request", now)
		s.updateTransactionStatusLocked(transaction.ID, TransactionStatusOrphan)
		return nil, exception
	}

	return nil, nil
}

func (s *MemoryStore) createTransactionMatchLocked(request PaymentRequest, transaction StablecoinTransaction, status, reason string, now time.Time) *TransactionMatch {
	match := TransactionMatch{
		ID:                      newID("mat"),
		BusinessID:              request.BusinessID,
		PaymentRequestID:        request.ID,
		StablecoinTransactionID: transaction.ID,
		Status:                  status,
		ExpectedAmount:          request.Amount,
		ReceivedAmount:          transaction.Amount,
		Reason:                  reason,
		CreatedAt:               now,
	}
	s.transactionMatches[match.ID] = match
	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   request.BusinessID,
		ActorType:    "system",
		ActorID:      "matching_engine",
		Action:       "transaction_match.created",
		ResourceType: "transaction_match",
		ResourceID:   match.ID,
		Metadata: map[string]string{
			"payment_request_id":        request.ID,
			"stablecoin_transaction_id": transaction.ID,
			"status":                    status,
		},
		CreatedAt: now,
	})
	return &match
}

func (s *MemoryStore) createExceptionLocked(businessID, paymentRequestID, transactionID, exceptionType, severity, reason string, now time.Time) *PaymentException {
	exception := PaymentException{
		ID:                      newID("exc"),
		BusinessID:              businessID,
		PaymentRequestID:        paymentRequestID,
		StablecoinTransactionID: transactionID,
		Type:                    exceptionType,
		Status:                  ExceptionStatusOpen,
		Severity:                severity,
		Reason:                  reason,
		CreatedAt:               now,
	}
	s.exceptions[exception.ID] = exception
	s.appendAuditLocked(AuditLog{
		ID:           newID("aud"),
		BusinessID:   businessID,
		ActorType:    "system",
		ActorID:      "exception_engine",
		Action:       "exception.created",
		ResourceType: "exception",
		ResourceID:   exception.ID,
		Metadata: map[string]string{
			"type":                      exceptionType,
			"payment_request_id":        paymentRequestID,
			"stablecoin_transaction_id": transactionID,
		},
		CreatedAt: now,
	})
	return &exception
}

func (s *MemoryStore) recordPaymentReceiptEventsLocked(request PaymentRequest, transaction StablecoinTransaction, match *TransactionMatch, exception *PaymentException, finalStatus string, now time.Time) {
	metadata := map[string]string{
		"amount":     transaction.Amount,
		"token":      transaction.Token,
		"chain":      transaction.Chain,
		"signature":  transaction.Signature,
		"invoice_id": request.InvoiceID,
	}
	s.createReceiptEventLocked(ReceiptEvent{
		BusinessID:              request.BusinessID,
		PaymentRequestID:        request.ID,
		StablecoinTransactionID: transaction.ID,
		Type:                    ReceiptEventPaymentDetected,
		Status:                  "detected",
		Description:             "incoming stablecoin transaction detected for this payment request",
		Metadata:                metadata,
		CreatedAt:               now,
	})
	s.createReceiptEventLocked(ReceiptEvent{
		BusinessID:              request.BusinessID,
		PaymentRequestID:        request.ID,
		StablecoinTransactionID: transaction.ID,
		Type:                    ReceiptEventTransactionVerified,
		Status:                  transaction.ConfirmationStatus,
		Description:             "transaction token, chain, wallet, and finality verified",
		Metadata: map[string]string{
			"confirmation_status": transaction.ConfirmationStatus,
			"slot":                strconv.FormatUint(transaction.Slot, 10),
			"mint":                transaction.Mint,
		},
		CreatedAt: now,
	})
	if match != nil {
		s.createReceiptEventLocked(ReceiptEvent{
			BusinessID:              request.BusinessID,
			PaymentRequestID:        request.ID,
			StablecoinTransactionID: transaction.ID,
			TransactionMatchID:      match.ID,
			Type:                    ReceiptEventTransactionMatched,
			Status:                  match.Status,
			Description:             match.Reason,
			Metadata: map[string]string{
				"expected_amount": match.ExpectedAmount,
				"received_amount": match.ReceivedAmount,
			},
			CreatedAt: now,
		})
	}
	if exception == nil {
		s.createReceiptEventLocked(ReceiptEvent{
			BusinessID:              request.BusinessID,
			PaymentRequestID:        request.ID,
			StablecoinTransactionID: transaction.ID,
			TransactionMatchID:      match.ID,
			Type:                    ReceiptEventPaymentConfirmed,
			Status:                  finalStatus,
			Description:             "payment request confirmed from verified on-chain evidence",
			CreatedAt:               now,
		})
		return
	}
	s.createReceiptEventLocked(ReceiptEvent{
		BusinessID:              request.BusinessID,
		PaymentRequestID:        request.ID,
		StablecoinTransactionID: transaction.ID,
		TransactionMatchID:      match.ID,
		ExceptionID:             exception.ID,
		Type:                    ReceiptEventPaymentExceptioned,
		Status:                  finalStatus,
		Description:             exception.Reason,
		Metadata: map[string]string{
			"exception_type": exception.Type,
			"severity":       exception.Severity,
		},
		CreatedAt: now,
	})
}

func (s *MemoryStore) createReceiptEventLocked(event ReceiptEvent) ReceiptEvent {
	if event.ID == "" {
		event.ID = newID("rcp")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if event.Metadata == nil {
		event.Metadata = map[string]string{}
	}
	s.receiptEvents[event.ID] = event
	s.enqueueWebhookDeliveriesLocked(event, event.CreatedAt)
	return event
}

func (s *MemoryStore) receiptEventsForRequestLocked(businessID, paymentRequestID string) []ReceiptEvent {
	events := make([]ReceiptEvent, 0)
	for _, event := range s.receiptEvents {
		if event.BusinessID == businessID && event.PaymentRequestID == paymentRequestID {
			events = append(events, event)
		}
	}
	sortReceiptEventsChronologically(events)
	return events
}

func (s *MemoryStore) enqueueWebhookDeliveriesLocked(event ReceiptEvent, now time.Time) {
	resourceType, resourceID := receiptEventResource(event)
	for _, subscription := range s.webhookSubscriptions {
		if subscription.BusinessID != event.BusinessID || !subscription.Enabled || !webhookEventMatches(subscription.EventTypes, event.Type) {
			continue
		}

		payload, err := json.Marshal(map[string]any{
			"id":                            event.ID,
			"event_type":                    event.Type,
			"business_id":                   event.BusinessID,
			"payment_request_id":            event.PaymentRequestID,
			"stablecoin_transaction_id":     event.StablecoinTransactionID,
			"transaction_match_id":          event.TransactionMatchID,
			"exception_id":                  event.ExceptionID,
			"resource_type":                 resourceType,
			"resource_id":                   resourceID,
			"status":                        event.Status,
			"description":                   event.Description,
			"metadata":                      event.Metadata,
			"created_at":                    event.CreatedAt,
			"webhook_subscription_id":       subscription.ID,
			"webhook_subscription_endpoint": subscription.URL,
		})
		if err != nil {
			continue
		}

		delivery := WebhookDelivery{
			ID:                      newID("whd"),
			BusinessID:              event.BusinessID,
			WebhookSubscriptionID:   subscription.ID,
			ReceiptEventID:          event.ID,
			EventType:               event.Type,
			ResourceType:            resourceType,
			ResourceID:              resourceID,
			PaymentRequestID:        event.PaymentRequestID,
			StablecoinTransactionID: event.StablecoinTransactionID,
			ExceptionID:             event.ExceptionID,
			Payload:                 string(payload),
			Signature:               signWebhookPayload(revealSecret(subscription.SecretCiphertext, subscription.BusinessID), payload),
			Status:                  WebhookStatusPending,
			CreatedAt:               now,
			UpdatedAt:               now,
		}
		s.webhookDeliveries[delivery.ID] = delivery
		s.attemptWebhookDeliveryLocked(delivery.ID, now)
	}
}

func (s *MemoryStore) attemptWebhookDeliveryLocked(deliveryID string, now time.Time) {
	delivery, ok := s.webhookDeliveries[deliveryID]
	if !ok {
		return
	}
	subscription, ok := s.webhookSubscriptions[delivery.WebhookSubscriptionID]
	if !ok || !subscription.Enabled {
		delivery.Status = WebhookStatusFailed
		delivery.LastError = "webhook subscription is disabled or missing"
		delivery.UpdatedAt = now
		s.webhookDeliveries[delivery.ID] = delivery
		return
	}

	delivery.Attempts++
	request, err := http.NewRequest(http.MethodPost, subscription.URL, bytes.NewBufferString(delivery.Payload))
	if err != nil {
		delivery.Status = WebhookStatusFailed
		delivery.LastError = err.Error()
		next := now.Add(time.Minute)
		delivery.NextAttemptAt = &next
		delivery.UpdatedAt = now
		s.webhookDeliveries[delivery.ID] = delivery
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Twins-Delivery-ID", delivery.ID)
	request.Header.Set("Twins-Event-Type", delivery.EventType)
	request.Header.Set("Twins-Signature", delivery.Signature)

	response, err := s.httpClient.Do(request)
	if err != nil {
		delivery.Status = WebhookStatusFailed
		delivery.LastError = err.Error()
		next := now.Add(time.Minute)
		delivery.NextAttemptAt = &next
		delivery.UpdatedAt = now
		s.webhookDeliveries[delivery.ID] = delivery
		return
	}
	defer response.Body.Close()

	delivery.LastStatusCode = response.StatusCode
	delivery.UpdatedAt = now
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		delivery.Status = WebhookStatusDelivered
		delivery.LastError = ""
		delivery.NextAttemptAt = nil
		delivery.DeliveredAt = &now
	} else {
		delivery.Status = WebhookStatusFailed
		delivery.LastError = response.Status
		next := now.Add(time.Minute)
		delivery.NextAttemptAt = &next
	}
	s.webhookDeliveries[delivery.ID] = delivery
}

func (s *MemoryStore) buildReconciliationReportLocked(run ReconciliationRun, now time.Time) ReconciliationReport {
	transactions := s.transactionsForRunLocked(run)
	paymentRequests := s.paymentRequestsForRunLocked(run)
	matches := s.matchesForRunLocked(run)
	exceptions := s.exceptionsForRunLocked(run)

	txByID := make(map[string]StablecoinTransaction)
	for _, transaction := range transactions {
		txByID[transaction.ID] = transaction
	}

	totalReceived := new(big.Rat)
	for _, transaction := range transactions {
		if amount, ok := new(big.Rat).SetString(transaction.Amount); ok {
			totalReceived.Add(totalReceived, amount)
		}
	}

	matchedTransactionIDs := make(map[string]bool)
	matchedRequestIDs := make(map[string]bool)
	rows := make([]SettlementReportRow, 0)
	for _, match := range matches {
		request, hasRequest := s.paymentRequests[match.PaymentRequestID]
		transaction, hasTransaction := s.stablecoinTransactions[match.StablecoinTransactionID]
		if !hasRequest || !hasTransaction || !runIncludesWallet(run, transaction.WalletID) {
			continue
		}
		exception, hasException := s.findExceptionForReconciliationLocked(request.ID, transaction.ID)
		row := s.settlementRowLocked(run, request, transaction, &match, exception, hasException, "reconciled", match.CreatedAt)
		if match.Status != MatchStatusConfirmed || hasException {
			row.ReconciliationStatus = "exception"
		}
		rows = append(rows, row)
		matchedTransactionIDs[transaction.ID] = true
		matchedRequestIDs[request.ID] = true
	}

	for _, transaction := range transactions {
		if matchedTransactionIDs[transaction.ID] {
			continue
		}
		exception, hasException := s.findExceptionForReconciliationLocked("", transaction.ID)
		row := s.settlementRowLocked(run, PaymentRequest{}, transaction, nil, exception, hasException, "unmatched", transaction.CreatedAt)
		rows = append(rows, row)
	}

	for _, request := range paymentRequests {
		if matchedRequestIDs[request.ID] {
			continue
		}
		exception, hasException := s.findExceptionForReconciliationLocked(request.ID, "")
		row := s.settlementRowLocked(run, request, StablecoinTransaction{}, nil, exception, hasException, "pending", request.UpdatedAt)
		if hasException {
			row.ReconciliationStatus = "exception"
		}
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].CreatedAt.Before(rows[j].CreatedAt)
		}
		return rows[i].ID < rows[j].ID
	})

	snapshots := s.walletSnapshotsForRunLocked(run, transactions, now)

	confirmedPaymentRequests := 0
	for _, request := range paymentRequests {
		if request.Status == PaymentStatusConfirmed {
			confirmedPaymentRequests++
		}
	}

	openExceptionCount := 0
	for _, exception := range exceptions {
		if exception.Status == ExceptionStatusOpen {
			openExceptionCount++
		}
	}

	run.TotalPaymentRequests = len(paymentRequests)
	run.ConfirmedPaymentRequests = confirmedPaymentRequests
	run.TotalTransactions = len(transactions)
	run.MatchedTransactions = len(matchedTransactionIDs)
	run.UnmatchedTransactions = run.TotalTransactions - run.MatchedTransactions
	run.TotalMatches = len(matches)
	run.ExceptionCount = len(exceptions)
	run.OpenExceptionCount = openExceptionCount
	run.TotalReceivedUSDC = formatRatAmount(totalReceived)

	return ReconciliationReport{
		ReconciliationRun: run,
		WalletSnapshots:   snapshots,
		Rows:              rows,
		Exceptions:        exceptions,
	}
}

func (s *MemoryStore) reconciliationReportForRunLocked(run ReconciliationRun) ReconciliationReport {
	snapshots := make([]WalletBalanceSnapshot, 0)
	for _, snapshot := range s.walletSnapshots {
		if snapshot.BusinessID == run.BusinessID && snapshot.ReconciliationRunID == run.ID {
			snapshots = append(snapshots, snapshot)
		}
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].WalletAddress < snapshots[j].WalletAddress
	})

	rows := make([]SettlementReportRow, 0)
	for _, row := range s.settlementRows {
		if row.BusinessID == run.BusinessID && row.ReconciliationRunID == run.ID {
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].CreatedAt.Before(rows[j].CreatedAt)
		}
		return rows[i].ID < rows[j].ID
	})

	return ReconciliationReport{
		ReconciliationRun: run,
		WalletSnapshots:   snapshots,
		Rows:              rows,
		Exceptions:        s.exceptionsForRunLocked(run),
	}
}

func (s *MemoryStore) transactionsForRunLocked(run ReconciliationRun) []StablecoinTransaction {
	transactions := make([]StablecoinTransaction, 0)
	for _, transaction := range s.stablecoinTransactions {
		if transaction.BusinessID != run.BusinessID || !runIncludesWallet(run, transaction.WalletID) {
			continue
		}
		if periodContains(run.PeriodStart, run.PeriodEnd, transaction.DetectedAt) || periodContains(run.PeriodStart, run.PeriodEnd, transaction.CreatedAt) {
			transactions = append(transactions, transaction)
		}
	}
	sort.Slice(transactions, func(i, j int) bool {
		return transactions[i].CreatedAt.Before(transactions[j].CreatedAt)
	})
	return transactions
}

func (s *MemoryStore) paymentRequestsForRunLocked(run ReconciliationRun) []PaymentRequest {
	requests := make([]PaymentRequest, 0)
	for _, request := range s.paymentRequests {
		if request.BusinessID != run.BusinessID || !runIncludesWallet(run, request.WalletID) {
			continue
		}
		if periodContains(run.PeriodStart, run.PeriodEnd, request.CreatedAt) || periodContains(run.PeriodStart, run.PeriodEnd, request.UpdatedAt) {
			requests = append(requests, request)
		}
	}
	sort.Slice(requests, func(i, j int) bool {
		return requests[i].CreatedAt.Before(requests[j].CreatedAt)
	})
	return requests
}

func (s *MemoryStore) matchesForRunLocked(run ReconciliationRun) []TransactionMatch {
	matches := make([]TransactionMatch, 0)
	for _, match := range s.transactionMatches {
		if match.BusinessID != run.BusinessID || !periodContains(run.PeriodStart, run.PeriodEnd, match.CreatedAt) {
			continue
		}
		transaction, ok := s.stablecoinTransactions[match.StablecoinTransactionID]
		if !ok || !runIncludesWallet(run, transaction.WalletID) {
			continue
		}
		matches = append(matches, match)
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].CreatedAt.Before(matches[j].CreatedAt)
	})
	return matches
}

func (s *MemoryStore) exceptionsForRunLocked(run ReconciliationRun) []PaymentException {
	exceptions := make([]PaymentException, 0)
	for _, exception := range s.exceptions {
		if exception.BusinessID != run.BusinessID {
			continue
		}
		inPeriod := periodContains(run.PeriodStart, run.PeriodEnd, exception.CreatedAt)
		if exception.ResolvedAt != nil {
			inPeriod = inPeriod || periodContains(run.PeriodStart, run.PeriodEnd, *exception.ResolvedAt)
		}
		if !inPeriod || !s.exceptionMatchesRunWalletLocked(run, exception) {
			continue
		}
		exceptions = append(exceptions, exception)
	}
	sort.Slice(exceptions, func(i, j int) bool {
		return exceptions[i].CreatedAt.Before(exceptions[j].CreatedAt)
	})
	return exceptions
}

func (s *MemoryStore) walletSnapshotsForRunLocked(run ReconciliationRun, transactions []StablecoinTransaction, now time.Time) []WalletBalanceSnapshot {
	wallets := make([]Wallet, 0)
	for _, wallet := range s.wallets {
		if wallet.BusinessID == run.BusinessID && wallet.ArchivedAt == nil && runIncludesWallet(run, wallet.ID) {
			wallets = append(wallets, wallet)
		}
	}
	sort.Slice(wallets, func(i, j int) bool {
		return wallets[i].Address < wallets[j].Address
	})

	snapshots := make([]WalletBalanceSnapshot, 0, len(wallets))
	for _, wallet := range wallets {
		total := new(big.Rat)
		count := 0
		for _, transaction := range transactions {
			if transaction.WalletID != wallet.ID {
				continue
			}
			if amount, ok := new(big.Rat).SetString(transaction.Amount); ok {
				total.Add(total, amount)
			}
			count++
		}
		snapshots = append(snapshots, WalletBalanceSnapshot{
			ID:                    newID("wbs"),
			BusinessID:            run.BusinessID,
			ReconciliationRunID:   run.ID,
			WalletID:              wallet.ID,
			WalletAddress:         wallet.Address,
			Chain:                 wallet.Chain,
			Token:                 TokenUSDC,
			ObservedInboundAmount: formatRatAmount(total),
			TransactionCount:      count,
			CapturedAt:            now,
		})
	}
	return snapshots
}

func (s *MemoryStore) settlementRowLocked(run ReconciliationRun, request PaymentRequest, transaction StablecoinTransaction, match *TransactionMatch, exception PaymentException, hasException bool, reconciliationStatus string, createdAt time.Time) SettlementReportRow {
	row := SettlementReportRow{
		ID:                   newID("set"),
		BusinessID:           run.BusinessID,
		ReconciliationRunID:  run.ID,
		ReconciliationStatus: reconciliationStatus,
		CreatedAt:            createdAt,
	}
	if !createdAt.IsZero() {
		row.CreatedAt = createdAt
	}
	if request.ID != "" {
		row.PaymentRequestID = request.ID
		row.CustomerID = request.CustomerID
		row.InvoiceID = request.InvoiceID
		row.OrderID = request.OrderID
		row.PaymentStatus = request.Status
		row.ExpectedAmount = request.Amount
		row.Token = request.Token
		row.Chain = request.Chain
		row.WalletID = request.WalletID
		row.WalletAddress = request.DestinationAddress
	}
	if transaction.ID != "" {
		row.StablecoinTransactionID = transaction.ID
		row.Signature = transaction.Signature
		row.TransactionStatus = transaction.Status
		row.ReceivedAmount = transaction.Amount
		if row.Token == "" {
			row.Token = transaction.Token
		}
		if row.Chain == "" {
			row.Chain = transaction.Chain
		}
		if row.WalletID == "" {
			row.WalletID = transaction.WalletID
		}
		if row.WalletAddress == "" {
			row.WalletAddress = transaction.DestinationOwner
		}
	}
	if match != nil {
		row.MatchID = match.ID
		row.MatchStatus = match.Status
		row.ExpectedAmount = match.ExpectedAmount
		row.ReceivedAmount = match.ReceivedAmount
	}
	if hasException {
		row.ExceptionID = exception.ID
		row.ExceptionType = exception.Type
		row.ExceptionStatus = exception.Status
	}
	if row.CreatedAt.IsZero() {
		row.CreatedAt = time.Now().UTC()
	}
	return row
}

func (s *MemoryStore) findExceptionForReconciliationLocked(paymentRequestID, transactionID string) (PaymentException, bool) {
	for _, exception := range s.exceptions {
		if paymentRequestID != "" && exception.PaymentRequestID != paymentRequestID {
			continue
		}
		if transactionID != "" && exception.StablecoinTransactionID != transactionID {
			continue
		}
		if paymentRequestID == "" && transactionID == "" {
			continue
		}
		return exception, true
	}
	return PaymentException{}, false
}

func (s *MemoryStore) exceptionMatchesRunWalletLocked(run ReconciliationRun, exception PaymentException) bool {
	if run.WalletID == "" {
		return true
	}
	if exception.PaymentRequestID != "" {
		if request, ok := s.paymentRequests[exception.PaymentRequestID]; ok && request.WalletID == run.WalletID {
			return true
		}
	}
	if exception.StablecoinTransactionID != "" {
		if transaction, ok := s.stablecoinTransactions[exception.StablecoinTransactionID]; ok && transaction.WalletID == run.WalletID {
			return true
		}
	}
	return false
}

func (s *MemoryStore) updatePaymentRequestStatusLocked(paymentRequestID, status string, now time.Time) {
	request := s.paymentRequests[paymentRequestID]
	request.Status = status
	request.UpdatedAt = now
	s.paymentRequests[paymentRequestID] = request
}

func (s *MemoryStore) updateTransactionStatusLocked(transactionID, status string) {
	transaction := s.stablecoinTransactions[transactionID]
	transaction.Status = status
	s.stablecoinTransactions[transactionID] = transaction
}

func transactionSignatureScope(businessID, chain, signature string) string {
	return businessID + "|" + chain + "|" + signature
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

func normalizeStablecoinTransactionInput(input IngestStablecoinTransactionInput) (IngestStablecoinTransactionInput, error) {
	input.Chain = strings.ToLower(strings.TrimSpace(input.Chain))
	input.Signature = strings.TrimSpace(input.Signature)
	input.ConfirmationStatus = strings.ToLower(strings.TrimSpace(input.ConfirmationStatus))
	input.SourceAddress = strings.TrimSpace(input.SourceAddress)
	input.SourceOwner = strings.TrimSpace(input.SourceOwner)
	input.DestinationAddress = strings.TrimSpace(input.DestinationAddress)
	input.DestinationOwner = strings.TrimSpace(input.DestinationOwner)
	input.Token = strings.ToUpper(strings.TrimSpace(input.Token))
	input.Mint = strings.TrimSpace(input.Mint)
	input.Amount = strings.TrimSpace(input.Amount)
	input.AmountAtomic = strings.TrimSpace(input.AmountAtomic)

	if input.Chain != ChainSolana {
		return input, InvalidArgument("only solana transactions are supported in v1")
	}
	if input.Signature == "" {
		return input, InvalidArgument("signature is required")
	}
	if input.Slot == 0 {
		return input, InvalidArgument("slot is required")
	}
	if input.ConfirmationStatus == "" {
		return input, InvalidArgument("confirmation_status is required")
	}
	if input.ConfirmationStatus != "processed" && input.ConfirmationStatus != "confirmed" && input.ConfirmationStatus != "finalized" {
		return input, InvalidArgument("confirmation_status must be processed, confirmed, or finalized")
	}
	if err := validateAddress(input.SourceAddress); err != nil {
		return input, InvalidArgument("source_address must look like a Solana address")
	}
	if err := validateAddress(input.DestinationAddress); err != nil {
		return input, InvalidArgument("destination_address must look like a Solana address")
	}
	if input.DestinationOwner == "" {
		input.DestinationOwner = input.DestinationAddress
	}
	if err := validateAddress(input.DestinationOwner); err != nil {
		return input, InvalidArgument("destination_owner must look like a Solana address")
	}
	if input.SourceOwner != "" {
		if err := validateAddress(input.SourceOwner); err != nil {
			return input, InvalidArgument("source_owner must look like a Solana address")
		}
	}
	if input.Token != TokenUSDC {
		return input, InvalidArgument("only USDC transaction evidence is accepted in v1")
	}
	if input.Mint != SolanaUSDCMint {
		return input, InvalidArgument("transaction mint is not Solana USDC")
	}
	if input.Decimals != 6 {
		return input, InvalidArgument("USDC transaction decimals must be 6")
	}
	if input.AmountAtomic == "" {
		return input, InvalidArgument("amount_atomic is required")
	}
	if _, ok := new(big.Int).SetString(input.AmountAtomic, 10); !ok {
		return input, InvalidArgument("amount_atomic must be a base-10 integer")
	}
	if strings.HasPrefix(input.AmountAtomic, "-") || input.AmountAtomic == "0" {
		return input, InvalidArgument("amount_atomic must be greater than zero")
	}
	if err := validateAmount(input.Amount); err != nil {
		return input, err
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

func compareAmountStrings(left, right string) (int, error) {
	leftValue, ok := new(big.Rat).SetString(left)
	if !ok {
		return 0, InvalidArgument("left amount must be a decimal number")
	}
	rightValue, ok := new(big.Rat).SetString(right)
	if !ok {
		return 0, InvalidArgument("right amount must be a decimal number")
	}
	return leftValue.Cmp(rightValue), nil
}

func normalizeReconciliationRunInput(input CreateReconciliationRunInput) (CreateReconciliationRunInput, error) {
	input.WalletID = strings.TrimSpace(input.WalletID)
	input.PeriodStart = input.PeriodStart.UTC()
	input.PeriodEnd = input.PeriodEnd.UTC()

	if input.PeriodStart.IsZero() && input.PeriodEnd.IsZero() {
		input.PeriodEnd = time.Now().UTC()
		input.PeriodStart = input.PeriodEnd.Add(-24 * time.Hour)
	}
	if input.PeriodStart.IsZero() || input.PeriodEnd.IsZero() {
		return input, InvalidArgument("period_start and period_end are required")
	}
	if !input.PeriodEnd.After(input.PeriodStart) {
		return input, InvalidArgument("period_end must be after period_start")
	}
	return input, nil
}

func normalizeExportInput(input CreateExportInput) (CreateExportInput, error) {
	input.ReconciliationRunID = strings.TrimSpace(input.ReconciliationRunID)
	input.Format = strings.ToLower(strings.TrimSpace(input.Format))
	if input.ReconciliationRunID == "" {
		return input, InvalidArgument("reconciliation_run_id is required")
	}
	if input.Format == "" {
		input.Format = ExportFormatCSV
	}
	if input.Format != ExportFormatCSV && input.Format != ExportFormatJSON {
		return input, InvalidArgument("export format must be csv or json")
	}
	return input, nil
}

func normalizeAPIScopes(scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return nil, InvalidArgument("api key scopes are required")
	}
	allowed := make(map[string]bool)
	for _, scope := range allAPIScopes() {
		allowed[scope] = true
	}
	seen := make(map[string]bool)
	normalized := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.ToLower(strings.TrimSpace(scope))
		if scope == "" {
			continue
		}
		if !allowed[scope] {
			return nil, InvalidArgument("unsupported api key scope: " + scope)
		}
		if seen[scope] {
			continue
		}
		seen[scope] = true
		normalized = append(normalized, scope)
	}
	if len(normalized) == 0 {
		return nil, InvalidArgument("api key scopes are required")
	}
	return normalized, nil
}

func allAPIScopes() []string {
	return []string{
		ScopeAdmin,
		ScopeAPIKeysRead,
		ScopeAPIKeysWrite,
		ScopeUsersRead,
		ScopeUsersWrite,
		ScopeWalletsRead,
		ScopeWalletsWrite,
		ScopePaymentRequestsRead,
		ScopePaymentRequestsWrite,
		ScopeTransactionsRead,
		ScopeTransactionsWrite,
		ScopeMatchesRead,
		ScopeExceptionsRead,
		ScopeExceptionsWrite,
		ScopeReceiptsRead,
		ScopeWebhooksRead,
		ScopeWebhooksWrite,
		ScopeReconciliationRead,
		ScopeReconciliationWrite,
		ScopeExportsRead,
		ScopeExportsWrite,
		ScopeAuditLogsRead,
		ScopeAccessLogsRead,
		ScopeIncidentsRead,
		ScopeIncidentsWrite,
		ScopeSecurityPolicyRead,
		ScopeSecurityPolicyWrite,
		ScopeBetaRead,
		ScopeBetaWrite,
		ScopeUsageRead,
	}
}

func validUserRole(role string) bool {
	return role == UserRoleOwner || role == UserRoleAdmin || role == UserRoleOperator || role == UserRoleViewer
}

func defaultSecurityPolicy(businessID string, now time.Time) SecurityPolicy {
	return SecurityPolicy{
		ID:                     newID("pol"),
		BusinessID:             businessID,
		RequireScopedAPIKeys:   true,
		RateLimitPerMinute:     240,
		DataRetentionDays:      365,
		AccessLogRetentionDays: 90,
		WebhookRetentionDays:   90,
		IncidentRetentionDays:  365,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
}

func validDesignPartnerStatus(status string) bool {
	switch status {
	case DesignPartnerStatusProspect,
		DesignPartnerStatusInvited,
		DesignPartnerStatusOnboarding,
		DesignPartnerStatusActive,
		DesignPartnerStatusPaused,
		DesignPartnerStatusChurned:
		return true
	default:
		return false
	}
}

func validBetaEvidenceType(evidenceType string) bool {
	switch evidenceType {
	case BetaEvidenceTypeRealTransaction,
		BetaEvidenceTypeExceptionCase,
		BetaEvidenceTypeTestimonial,
		BetaEvidenceTypePricingCommitment,
		BetaEvidenceTypeWorkflowPain,
		BetaEvidenceTypeIntegrationRequest:
		return true
	default:
		return false
	}
}

func remainingNeeded(goal, actual int) int {
	if actual >= goal {
		return 0
	}
	return goal - actual
}

func (s *MemoryStore) designPartnersForBusinessLocked(businessID string) []DesignPartner {
	partners := make([]DesignPartner, 0)
	for _, partner := range s.designPartners {
		if partner.BusinessID == businessID {
			partners = append(partners, partner)
		}
	}
	return partners
}

func (s *MemoryStore) betaEvidenceForBusinessLocked(businessID string) []BetaEvidence {
	evidence := make([]BetaEvidence, 0)
	for _, item := range s.betaEvidence {
		if item.BusinessID == businessID {
			evidence = append(evidence, item)
		}
	}
	return evidence
}

func (s *MemoryStore) usageMetricsLocked(businessID string, now time.Time) UsageMetrics {
	metrics := UsageMetrics{BusinessID: businessID, UpdatedAt: now}
	receiptRequests := make(map[string]bool)

	for _, wallet := range s.wallets {
		if wallet.BusinessID == businessID && wallet.ArchivedAt == nil {
			metrics.ConnectedWallets++
		}
	}
	for _, request := range s.paymentRequests {
		if request.BusinessID != businessID {
			continue
		}
		metrics.PaymentRequestsCreated++
		switch request.Status {
		case PaymentStatusUnderpaid:
			metrics.UnderpaidPayments++
		case PaymentStatusOverpaid:
			metrics.OverpaidPayments++
		case PaymentStatusExpired:
			metrics.LateOrExpiredPayments++
		}
	}
	for _, transaction := range s.stablecoinTransactions {
		if transaction.BusinessID != businessID {
			continue
		}
		metrics.TransactionsDetected++
		switch transaction.Status {
		case TransactionStatusMatchedToRequest:
			metrics.TransactionsMatched++
		case TransactionStatusOrphan:
			metrics.OrphanTransactions++
		}
	}
	for _, delivery := range s.webhookDeliveries {
		if delivery.BusinessID != businessID {
			continue
		}
		switch delivery.Status {
		case WebhookStatusDelivered:
			metrics.WebhooksDelivered++
		case WebhookStatusFailed:
			metrics.WebhooksFailed++
		}
	}
	for _, exception := range s.exceptions {
		if exception.BusinessID != businessID {
			continue
		}
		switch exception.Status {
		case ExceptionStatusOpen:
			metrics.OpenExceptions++
		case ExceptionStatusResolved:
			metrics.ResolvedExceptions++
		}
	}
	for _, event := range s.receiptEvents {
		if event.BusinessID == businessID {
			receiptRequests[event.PaymentRequestID] = true
		}
	}
	metrics.ReceiptsGenerated = len(receiptRequests)
	for _, export := range s.exports {
		if export.BusinessID == businessID && export.Type == ExportTypeSettlementReport {
			metrics.SettlementReportsExported++
		}
	}
	for _, run := range s.reconciliationRuns {
		if run.BusinessID == businessID {
			metrics.ReconciliationRuns++
		}
	}
	for _, row := range s.settlementRows {
		if row.BusinessID == businessID && row.ReconciliationStatus == "reconciled" {
			metrics.ReconciledBusinessRecords++
		}
	}
	for _, user := range s.users {
		if user.BusinessID == businessID && user.Status == UserStatusActive {
			metrics.Users++
		}
	}
	for _, apiKey := range s.apiKeys {
		if apiKey.BusinessID == businessID && apiKey.RevokedAt == nil {
			metrics.ActiveAPIKeys++
		}
	}
	for _, partner := range s.designPartners {
		if partner.BusinessID != businessID {
			continue
		}
		metrics.DesignPartners++
		if partner.AgreedToTest {
			metrics.DesignPartnersAgreedToTest++
		}
		if partner.Status == DesignPartnerStatusOnboarding || partner.Status == DesignPartnerStatusActive {
			metrics.ActiveDesignPartners++
		}
		if partner.PricingCommitment {
			metrics.PricingCommitments++
		}
	}
	for _, item := range s.betaEvidence {
		if item.BusinessID != businessID {
			continue
		}
		metrics.BetaEvidenceItems++
		switch item.Type {
		case BetaEvidenceTypeTestimonial:
			metrics.Testimonials++
		case BetaEvidenceTypeRealTransaction:
			metrics.PrivateBetaTransactionsProcessed++
		case BetaEvidenceTypeExceptionCase:
			metrics.PrivateBetaExceptionCasesCollected++
		}
	}
	return metrics
}

func renderSettlementExport(report ReconciliationReport, format string) (string, string, error) {
	fileName := "settlement-" + report.ReconciliationRun.ID + "." + format
	switch format {
	case ExportFormatCSV:
		var buffer bytes.Buffer
		writer := csv.NewWriter(&buffer)
		header := []string{
			"reconciliation_run_id",
			"payment_request_id",
			"customer_id",
			"invoice_id",
			"payment_status",
			"expected_amount",
			"received_amount",
			"token",
			"chain",
			"wallet_id",
			"stablecoin_transaction_id",
			"signature",
			"transaction_status",
			"match_status",
			"exception_type",
			"exception_status",
			"reconciliation_status",
		}
		if err := writer.Write(header); err != nil {
			return "", "", err
		}
		for _, row := range report.Rows {
			record := []string{
				row.ReconciliationRunID,
				row.PaymentRequestID,
				row.CustomerID,
				row.InvoiceID,
				row.PaymentStatus,
				row.ExpectedAmount,
				row.ReceivedAmount,
				row.Token,
				row.Chain,
				row.WalletID,
				row.StablecoinTransactionID,
				row.Signature,
				row.TransactionStatus,
				row.MatchStatus,
				row.ExceptionType,
				row.ExceptionStatus,
				row.ReconciliationStatus,
			}
			if err := writer.Write(record); err != nil {
				return "", "", err
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return "", "", err
		}
		return buffer.String(), fileName, nil
	case ExportFormatJSON:
		payload, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", "", err
		}
		return string(payload), fileName, nil
	default:
		return "", "", InvalidArgument("export format must be csv or json")
	}
}

func periodContains(start, end, value time.Time) bool {
	if value.IsZero() {
		return false
	}
	value = value.UTC()
	return !value.Before(start) && value.Before(end)
}

func runIncludesWallet(run ReconciliationRun, walletID string) bool {
	return run.WalletID == "" || run.WalletID == walletID
}

func formatRatAmount(value *big.Rat) string {
	if value == nil || value.Sign() == 0 {
		return "0"
	}
	formatted := value.FloatString(6)
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")
	if formatted == "" {
		return "0"
	}
	return formatted
}

func normalizeWebhookEventTypes(eventTypes []string) ([]string, error) {
	if len(eventTypes) == 0 {
		return allReceiptEventTypes(), nil
	}

	allowed := make(map[string]bool)
	for _, eventType := range allReceiptEventTypes() {
		allowed[eventType] = true
	}

	seen := make(map[string]bool)
	normalized := make([]string, 0, len(eventTypes))
	for _, eventType := range eventTypes {
		eventType = strings.TrimSpace(eventType)
		if eventType == "" {
			continue
		}
		if !allowed[eventType] {
			return nil, InvalidArgument("unsupported webhook event type: " + eventType)
		}
		if seen[eventType] {
			continue
		}
		seen[eventType] = true
		normalized = append(normalized, eventType)
	}
	if len(normalized) == 0 {
		return nil, InvalidArgument("event_types must include at least one event type")
	}
	return normalized, nil
}

func allReceiptEventTypes() []string {
	return []string{
		ReceiptEventPaymentRequestCreated,
		ReceiptEventPaymentDetected,
		ReceiptEventTransactionVerified,
		ReceiptEventTransactionMatched,
		ReceiptEventPaymentConfirmed,
		ReceiptEventPaymentExceptioned,
		ReceiptEventExceptionResolved,
	}
}

func webhookEventMatches(eventTypes []string, eventType string) bool {
	for _, candidate := range eventTypes {
		if candidate == eventType {
			return true
		}
	}
	return false
}

func receiptEventResource(event ReceiptEvent) (string, string) {
	switch {
	case event.ExceptionID != "":
		return "exception", event.ExceptionID
	case event.TransactionMatchID != "":
		return "transaction_match", event.TransactionMatchID
	case event.StablecoinTransactionID != "":
		return "stablecoin_transaction", event.StablecoinTransactionID
	default:
		return "payment_request", event.PaymentRequestID
	}
}

func sortReceiptEventsChronologically(events []ReceiptEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		if !events[i].CreatedAt.Equal(events[j].CreatedAt) {
			return events[i].CreatedAt.Before(events[j].CreatedAt)
		}
		leftRank := receiptEventRank(events[i].Type)
		rightRank := receiptEventRank(events[j].Type)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return events[i].ID < events[j].ID
	})
}

func receiptEventRank(eventType string) int {
	switch eventType {
	case ReceiptEventPaymentRequestCreated:
		return 10
	case ReceiptEventPaymentDetected:
		return 20
	case ReceiptEventTransactionVerified:
		return 30
	case ReceiptEventTransactionMatched:
		return 40
	case ReceiptEventPaymentConfirmed:
		return 50
	case ReceiptEventPaymentExceptioned:
		return 60
	case ReceiptEventExceptionResolved:
		return 70
	default:
		return 100
	}
}

func signWebhookPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func protectSecret(secret, businessID string) string {
	key := sha256.Sum256([]byte("twins-local-secret-protection|" + businessID))
	plain := []byte(secret)
	cipher := make([]byte, len(plain))
	for i, b := range plain {
		cipher[i] = b ^ key[i%len(key)]
	}
	return "enc:v1:" + hex.EncodeToString(cipher)
}

func revealSecret(ciphertext, businessID string) string {
	const prefix = "enc:v1:"
	if !strings.HasPrefix(ciphertext, prefix) {
		return ciphertext
	}
	raw, err := hex.DecodeString(strings.TrimPrefix(ciphertext, prefix))
	if err != nil {
		return ""
	}
	key := sha256.Sum256([]byte("twins-local-secret-protection|" + businessID))
	plain := make([]byte, len(raw))
	for i, b := range raw {
		plain[i] = b ^ key[i%len(key)]
	}
	return string(plain)
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

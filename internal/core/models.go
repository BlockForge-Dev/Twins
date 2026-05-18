package core

import "time"

const (
	PaymentStatusAwaitingPayment  = "awaiting_payment"
	PaymentStatusConfirmed        = "confirmed"
	PaymentStatusUnderpaid        = "underpaid"
	PaymentStatusOverpaid         = "overpaid"
	PaymentStatusExpired          = "expired"
	PaymentStatusException        = "exception"
	PaymentStatusManuallyResolved = "manually_resolved"

	TransactionStatusPendingFinality  = "pending_finality"
	TransactionStatusConfirmedOnchain = "confirmed_onchain"
	TransactionStatusMatchedToRequest = "matched_to_request"
	TransactionStatusOrphan           = "orphan"

	MatchStatusConfirmed = "confirmed"
	MatchStatusUnderpaid = "underpaid"
	MatchStatusOverpaid  = "overpaid"
	MatchStatusExpired   = "expired"

	ExceptionStatusOpen     = "open"
	ExceptionStatusResolved = "resolved"

	ExceptionTypeUnderpaid      = "underpaid"
	ExceptionTypeOverpaid       = "overpaid"
	ExceptionTypeExpired        = "expired"
	ExceptionTypeOrphan         = "orphan"
	ExceptionTypeAmbiguousMatch = "ambiguous_match"
	ExceptionTypeWrongToken     = "wrong_token"
	ExceptionTypeWrongChain     = "wrong_chain"

	ChainSolana    = "solana"
	TokenUSDC      = "USDC"
	SolanaUSDCMint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
)

type Business struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type APIKey struct {
	ID         string     `json:"id"`
	BusinessID string     `json:"business_id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	SecretHash string     `json:"-"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

type Wallet struct {
	ID         string     `json:"id"`
	BusinessID string     `json:"business_id"`
	Label      string     `json:"label"`
	Chain      string     `json:"chain"`
	Address    string     `json:"address"`
	CreatedAt  time.Time  `json:"created_at"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
}

type PaymentRequest struct {
	ID                 string            `json:"id"`
	BusinessID         string            `json:"business_id"`
	WalletID           string            `json:"wallet_id"`
	CustomerID         string            `json:"customer_id"`
	InvoiceID          string            `json:"invoice_id"`
	OrderID            string            `json:"order_id,omitempty"`
	Amount             string            `json:"amount"`
	Token              string            `json:"token"`
	Chain              string            `json:"chain"`
	DestinationAddress string            `json:"destination_address"`
	ExpiresAt          time.Time         `json:"expires_at"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	Status             string            `json:"status"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type StablecoinTransaction struct {
	ID                 string    `json:"id"`
	BusinessID         string    `json:"business_id"`
	WalletID           string    `json:"wallet_id"`
	Chain              string    `json:"chain"`
	Signature          string    `json:"signature"`
	Slot               uint64    `json:"slot"`
	BlockTime          *int64    `json:"block_time,omitempty"`
	ConfirmationStatus string    `json:"confirmation_status"`
	SourceAddress      string    `json:"source_address"`
	SourceOwner        string    `json:"source_owner,omitempty"`
	DestinationAddress string    `json:"destination_address"`
	DestinationOwner   string    `json:"destination_owner"`
	Token              string    `json:"token"`
	Mint               string    `json:"mint"`
	Amount             string    `json:"amount"`
	AmountAtomic       string    `json:"amount_atomic"`
	Decimals           uint8     `json:"decimals"`
	Status             string    `json:"status"`
	DetectedAt         time.Time `json:"detected_at"`
	CreatedAt          time.Time `json:"created_at"`
}

type TransactionMatch struct {
	ID                      string    `json:"id"`
	BusinessID              string    `json:"business_id"`
	PaymentRequestID        string    `json:"payment_request_id"`
	StablecoinTransactionID string    `json:"stablecoin_transaction_id"`
	Status                  string    `json:"status"`
	ExpectedAmount          string    `json:"expected_amount"`
	ReceivedAmount          string    `json:"received_amount"`
	Reason                  string    `json:"reason,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
}

type PaymentException struct {
	ID                      string     `json:"id"`
	BusinessID              string     `json:"business_id"`
	PaymentRequestID        string     `json:"payment_request_id,omitempty"`
	StablecoinTransactionID string     `json:"stablecoin_transaction_id,omitempty"`
	Type                    string     `json:"type"`
	Status                  string     `json:"status"`
	Severity                string     `json:"severity"`
	Reason                  string     `json:"reason"`
	ResolutionReason        string     `json:"resolution_reason,omitempty"`
	ResolvedBy              string     `json:"resolved_by,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	ResolvedAt              *time.Time `json:"resolved_at,omitempty"`
}

type AuditLog struct {
	ID           string            `json:"id"`
	BusinessID   string            `json:"business_id"`
	ActorType    string            `json:"actor_type"`
	ActorID      string            `json:"actor_id"`
	Action       string            `json:"action"`
	ResourceType string            `json:"resource_type"`
	ResourceID   string            `json:"resource_id"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

type CreateBusinessInput struct {
	Name string `json:"name"`
}

type CreateBusinessResult struct {
	Business Business `json:"business"`
	APIKey   string   `json:"api_key"`
}

type RegisterWalletInput struct {
	Label   string `json:"label"`
	Chain   string `json:"chain"`
	Address string `json:"address"`
}

type CreatePaymentRequestInput struct {
	WalletID   string            `json:"wallet_id"`
	CustomerID string            `json:"customer_id"`
	InvoiceID  string            `json:"invoice_id"`
	OrderID    string            `json:"order_id,omitempty"`
	Amount     string            `json:"amount"`
	Token      string            `json:"token"`
	Chain      string            `json:"chain"`
	ExpiresAt  time.Time         `json:"expires_at"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type CreatePaymentRequestResult struct {
	PaymentRequest     PaymentRequest `json:"payment_request"`
	IdempotentReplayed bool           `json:"idempotent_replayed"`
}

type IngestStablecoinTransactionInput struct {
	Chain              string `json:"chain"`
	Signature          string `json:"signature"`
	Slot               uint64 `json:"slot"`
	BlockTime          *int64 `json:"block_time,omitempty"`
	ConfirmationStatus string `json:"confirmation_status"`
	SourceAddress      string `json:"source_address"`
	SourceOwner        string `json:"source_owner,omitempty"`
	DestinationAddress string `json:"destination_address"`
	DestinationOwner   string `json:"destination_owner"`
	Token              string `json:"token"`
	Mint               string `json:"mint"`
	Amount             string `json:"amount"`
	AmountAtomic       string `json:"amount_atomic"`
	Decimals           uint8  `json:"decimals"`
}

type IngestStablecoinTransactionResult struct {
	StablecoinTransaction StablecoinTransaction `json:"stablecoin_transaction"`
	TransactionMatch      *TransactionMatch     `json:"transaction_match,omitempty"`
	Exception             *PaymentException     `json:"exception,omitempty"`
	DuplicateReplayed     bool                  `json:"duplicate_replayed"`
}

type ResolveExceptionInput struct {
	Reason string `json:"reason"`
}

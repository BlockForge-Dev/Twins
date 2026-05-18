package core

import "time"

const (
	PaymentStatusAwaitingPayment = "awaiting_payment"

	ChainSolana = "solana"
	TokenUSDC   = "USDC"
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

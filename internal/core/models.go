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

	ReceiptEventPaymentRequestCreated = "payment_request.created"
	ReceiptEventPaymentDetected       = "payment.detected"
	ReceiptEventTransactionVerified   = "transaction.verified"
	ReceiptEventTransactionMatched    = "transaction.matched"
	ReceiptEventPaymentConfirmed      = "payment.confirmed"
	ReceiptEventPaymentExceptioned    = "payment.exceptioned"
	ReceiptEventExceptionResolved     = "exception.resolved"

	WebhookStatusPending   = "pending"
	WebhookStatusDelivered = "delivered"
	WebhookStatusFailed    = "failed"

	ReconciliationStatusCompleted = "completed"

	ExportStatusReady          = "ready"
	ExportFormatCSV            = "csv"
	ExportFormatJSON           = "json"
	ExportTypeSettlementReport = "settlement_report"

	UserRoleOwner    = "owner"
	UserRoleAdmin    = "admin"
	UserRoleOperator = "operator"
	UserRoleViewer   = "viewer"
	UserStatusActive = "active"

	IncidentStatusOpen     = "open"
	IncidentStatusResolved = "resolved"

	DesignPartnerStatusProspect   = "prospect"
	DesignPartnerStatusInvited    = "invited"
	DesignPartnerStatusOnboarding = "onboarding"
	DesignPartnerStatusActive     = "active"
	DesignPartnerStatusPaused     = "paused"
	DesignPartnerStatusChurned    = "churned"

	BetaEvidenceTypeRealTransaction    = "real_transaction"
	BetaEvidenceTypeExceptionCase      = "exception_case"
	BetaEvidenceTypeTestimonial        = "testimonial"
	BetaEvidenceTypePricingCommitment  = "pricing_commitment"
	BetaEvidenceTypeWorkflowPain       = "workflow_pain"
	BetaEvidenceTypeIntegrationRequest = "integration_request"

	ScopeAdmin                = "admin"
	ScopeAPIKeysRead          = "api_keys:read"
	ScopeAPIKeysWrite         = "api_keys:write"
	ScopeUsersRead            = "users:read"
	ScopeUsersWrite           = "users:write"
	ScopeWalletsRead          = "wallets:read"
	ScopeWalletsWrite         = "wallets:write"
	ScopePaymentRequestsRead  = "payment_requests:read"
	ScopePaymentRequestsWrite = "payment_requests:write"
	ScopeTransactionsRead     = "transactions:read"
	ScopeTransactionsWrite    = "transactions:write"
	ScopeMatchesRead          = "matches:read"
	ScopeExceptionsRead       = "exceptions:read"
	ScopeExceptionsWrite      = "exceptions:write"
	ScopeReceiptsRead         = "receipts:read"
	ScopeWebhooksRead         = "webhooks:read"
	ScopeWebhooksWrite        = "webhooks:write"
	ScopeReconciliationRead   = "reconciliation:read"
	ScopeReconciliationWrite  = "reconciliation:write"
	ScopeExportsRead          = "exports:read"
	ScopeExportsWrite         = "exports:write"
	ScopeAuditLogsRead        = "audit_logs:read"
	ScopeAccessLogsRead       = "access_logs:read"
	ScopeIncidentsRead        = "incidents:read"
	ScopeIncidentsWrite       = "incidents:write"
	ScopeSecurityPolicyRead   = "security_policy:read"
	ScopeSecurityPolicyWrite  = "security_policy:write"
	ScopeBetaRead             = "beta:read"
	ScopeBetaWrite            = "beta:write"
	ScopeUsageRead            = "usage:read"

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
	CreatedBy  string     `json:"created_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

type User struct {
	ID         string    `json:"id"`
	BusinessID string    `json:"business_id"`
	Email      string    `json:"email"`
	Name       string    `json:"name,omitempty"`
	Role       string    `json:"role"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
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

type ReceiptEvent struct {
	ID                      string            `json:"id"`
	BusinessID              string            `json:"business_id"`
	PaymentRequestID        string            `json:"payment_request_id"`
	StablecoinTransactionID string            `json:"stablecoin_transaction_id,omitempty"`
	TransactionMatchID      string            `json:"transaction_match_id,omitempty"`
	ExceptionID             string            `json:"exception_id,omitempty"`
	Type                    string            `json:"type"`
	Status                  string            `json:"status"`
	Description             string            `json:"description"`
	Metadata                map[string]string `json:"metadata,omitempty"`
	CreatedAt               time.Time         `json:"created_at"`
}

type Receipt struct {
	PaymentRequest PaymentRequest `json:"payment_request"`
	Events         []ReceiptEvent `json:"events"`
}

type WebhookSubscription struct {
	ID               string    `json:"id"`
	BusinessID       string    `json:"business_id"`
	URL              string    `json:"url"`
	SecretCiphertext string    `json:"-"`
	EventTypes       []string  `json:"event_types"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type WebhookDelivery struct {
	ID                      string     `json:"id"`
	BusinessID              string     `json:"business_id"`
	WebhookSubscriptionID   string     `json:"webhook_subscription_id"`
	ReceiptEventID          string     `json:"receipt_event_id"`
	EventType               string     `json:"event_type"`
	ResourceType            string     `json:"resource_type"`
	ResourceID              string     `json:"resource_id"`
	PaymentRequestID        string     `json:"payment_request_id,omitempty"`
	StablecoinTransactionID string     `json:"stablecoin_transaction_id,omitempty"`
	ExceptionID             string     `json:"exception_id,omitempty"`
	Payload                 string     `json:"payload"`
	Signature               string     `json:"signature"`
	Status                  string     `json:"status"`
	Attempts                int        `json:"attempts"`
	LastStatusCode          int        `json:"last_status_code,omitempty"`
	LastError               string     `json:"last_error,omitempty"`
	NextAttemptAt           *time.Time `json:"next_attempt_at,omitempty"`
	DeliveredAt             *time.Time `json:"delivered_at,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

type ReconciliationRun struct {
	ID                       string     `json:"id"`
	BusinessID               string     `json:"business_id"`
	Status                   string     `json:"status"`
	PeriodStart              time.Time  `json:"period_start"`
	PeriodEnd                time.Time  `json:"period_end"`
	WalletID                 string     `json:"wallet_id,omitempty"`
	TotalPaymentRequests     int        `json:"total_payment_requests"`
	ConfirmedPaymentRequests int        `json:"confirmed_payment_requests"`
	TotalTransactions        int        `json:"total_transactions"`
	MatchedTransactions      int        `json:"matched_transactions"`
	UnmatchedTransactions    int        `json:"unmatched_transactions"`
	TotalMatches             int        `json:"total_matches"`
	ExceptionCount           int        `json:"exception_count"`
	OpenExceptionCount       int        `json:"open_exception_count"`
	TotalReceivedUSDC        string     `json:"total_received_usdc"`
	CreatedAt                time.Time  `json:"created_at"`
	CompletedAt              *time.Time `json:"completed_at,omitempty"`
}

type WalletBalanceSnapshot struct {
	ID                    string    `json:"id"`
	BusinessID            string    `json:"business_id"`
	ReconciliationRunID   string    `json:"reconciliation_run_id"`
	WalletID              string    `json:"wallet_id"`
	WalletAddress         string    `json:"wallet_address"`
	Chain                 string    `json:"chain"`
	Token                 string    `json:"token"`
	ObservedInboundAmount string    `json:"observed_inbound_amount"`
	TransactionCount      int       `json:"transaction_count"`
	CapturedAt            time.Time `json:"captured_at"`
}

type SettlementReportRow struct {
	ID                      string    `json:"id"`
	BusinessID              string    `json:"business_id"`
	ReconciliationRunID     string    `json:"reconciliation_run_id"`
	PaymentRequestID        string    `json:"payment_request_id,omitempty"`
	CustomerID              string    `json:"customer_id,omitempty"`
	InvoiceID               string    `json:"invoice_id,omitempty"`
	OrderID                 string    `json:"order_id,omitempty"`
	PaymentStatus           string    `json:"payment_status,omitempty"`
	ExpectedAmount          string    `json:"expected_amount,omitempty"`
	ReceivedAmount          string    `json:"received_amount,omitempty"`
	Token                   string    `json:"token,omitempty"`
	Chain                   string    `json:"chain,omitempty"`
	WalletID                string    `json:"wallet_id,omitempty"`
	WalletAddress           string    `json:"wallet_address,omitempty"`
	StablecoinTransactionID string    `json:"stablecoin_transaction_id,omitempty"`
	Signature               string    `json:"signature,omitempty"`
	TransactionStatus       string    `json:"transaction_status,omitempty"`
	MatchID                 string    `json:"match_id,omitempty"`
	MatchStatus             string    `json:"match_status,omitempty"`
	ExceptionID             string    `json:"exception_id,omitempty"`
	ExceptionType           string    `json:"exception_type,omitempty"`
	ExceptionStatus         string    `json:"exception_status,omitempty"`
	ReconciliationStatus    string    `json:"reconciliation_status"`
	CreatedAt               time.Time `json:"created_at"`
}

type ReconciliationReport struct {
	ReconciliationRun ReconciliationRun       `json:"reconciliation_run"`
	WalletSnapshots   []WalletBalanceSnapshot `json:"wallet_snapshots"`
	Rows              []SettlementReportRow   `json:"rows"`
	Exceptions        []PaymentException      `json:"exceptions"`
}

type ExportRecord struct {
	ID                  string    `json:"id"`
	BusinessID          string    `json:"business_id"`
	ReconciliationRunID string    `json:"reconciliation_run_id"`
	Type                string    `json:"type"`
	Format              string    `json:"format"`
	Status              string    `json:"status"`
	FileName            string    `json:"file_name"`
	RowCount            int       `json:"row_count"`
	Content             string    `json:"content,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

type AccessLog struct {
	ID          string    `json:"id"`
	BusinessID  string    `json:"business_id,omitempty"`
	APIKeyID    string    `json:"api_key_id,omitempty"`
	Method      string    `json:"method"`
	Path        string    `json:"path"`
	StatusCode  int       `json:"status_code"`
	RemoteAddr  string    `json:"remote_addr"`
	UserAgent   string    `json:"user_agent,omitempty"`
	DurationMS  int64     `json:"duration_ms"`
	RateLimited bool      `json:"rate_limited"`
	AccessedAt  time.Time `json:"accessed_at"`
}

type Incident struct {
	ID                string     `json:"id"`
	BusinessID        string     `json:"business_id"`
	Title             string     `json:"title"`
	Severity          string     `json:"severity"`
	Status            string     `json:"status"`
	Description       string     `json:"description,omitempty"`
	ResolutionSummary string     `json:"resolution_summary,omitempty"`
	CreatedBy         string     `json:"created_by,omitempty"`
	ResolvedBy        string     `json:"resolved_by,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	ResolvedAt        *time.Time `json:"resolved_at,omitempty"`
}

type SecurityPolicy struct {
	ID                     string    `json:"id"`
	BusinessID             string    `json:"business_id"`
	RequireScopedAPIKeys   bool      `json:"require_scoped_api_keys"`
	RateLimitPerMinute     int       `json:"rate_limit_per_minute"`
	DataRetentionDays      int       `json:"data_retention_days"`
	AccessLogRetentionDays int       `json:"access_log_retention_days"`
	WebhookRetentionDays   int       `json:"webhook_retention_days"`
	IncidentRetentionDays  int       `json:"incident_retention_days"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type DesignPartner struct {
	ID                    string    `json:"id"`
	BusinessID            string    `json:"business_id"`
	CompanyName           string    `json:"company_name"`
	Segment               string    `json:"segment,omitempty"`
	ContactName           string    `json:"contact_name,omitempty"`
	ContactEmail          string    `json:"contact_email,omitempty"`
	UseCase               string    `json:"use_case,omitempty"`
	Status                string    `json:"status"`
	AgreedToTest          bool      `json:"agreed_to_test"`
	PricingCommitment     bool      `json:"pricing_commitment"`
	ExpectedMonthlyVolume int       `json:"expected_monthly_volume,omitempty"`
	Notes                 string    `json:"notes,omitempty"`
	CreatedBy             string    `json:"created_by,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type BetaEvidence struct {
	ID                      string    `json:"id"`
	BusinessID              string    `json:"business_id"`
	DesignPartnerID         string    `json:"design_partner_id,omitempty"`
	Type                    string    `json:"type"`
	Title                   string    `json:"title"`
	Description             string    `json:"description,omitempty"`
	PaymentRequestID        string    `json:"payment_request_id,omitempty"`
	StablecoinTransactionID string    `json:"stablecoin_transaction_id,omitempty"`
	ExceptionID             string    `json:"exception_id,omitempty"`
	Quote                   string    `json:"quote,omitempty"`
	CreatedBy               string    `json:"created_by,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
}

type UsageMetrics struct {
	BusinessID                         string    `json:"business_id"`
	ConnectedWallets                   int       `json:"connected_wallets"`
	PaymentRequestsCreated             int       `json:"payment_requests_created"`
	TransactionsDetected               int       `json:"transactions_detected"`
	TransactionsMatched                int       `json:"transactions_matched"`
	OrphanTransactions                 int       `json:"orphan_transactions"`
	UnderpaidPayments                  int       `json:"underpaid_payments"`
	OverpaidPayments                   int       `json:"overpaid_payments"`
	LateOrExpiredPayments              int       `json:"late_or_expired_payments"`
	WebhooksDelivered                  int       `json:"webhooks_delivered"`
	WebhooksFailed                     int       `json:"webhooks_failed"`
	OpenExceptions                     int       `json:"open_exceptions"`
	ResolvedExceptions                 int       `json:"resolved_exceptions"`
	ReceiptsGenerated                  int       `json:"receipts_generated"`
	SettlementReportsExported          int       `json:"settlement_reports_exported"`
	ReconciliationRuns                 int       `json:"reconciliation_runs"`
	ReconciledBusinessRecords          int       `json:"reconciled_business_records"`
	Users                              int       `json:"users"`
	ActiveAPIKeys                      int       `json:"active_api_keys"`
	DesignPartners                     int       `json:"design_partners"`
	DesignPartnersAgreedToTest         int       `json:"design_partners_agreed_to_test"`
	ActiveDesignPartners               int       `json:"active_design_partners"`
	PricingCommitments                 int       `json:"pricing_commitments"`
	BetaEvidenceItems                  int       `json:"beta_evidence_items"`
	Testimonials                       int       `json:"testimonials"`
	PrivateBetaTransactionsProcessed   int       `json:"private_beta_transactions_processed"`
	PrivateBetaExceptionCasesCollected int       `json:"private_beta_exception_cases_collected"`
	UpdatedAt                          time.Time `json:"updated_at"`
}

type PrivateBetaReport struct {
	BusinessID                         string          `json:"business_id"`
	Usage                              UsageMetrics    `json:"usage"`
	DesignPartners                     []DesignPartner `json:"design_partners"`
	Evidence                           []BetaEvidence  `json:"evidence"`
	DesignPartnersOnboarded            int             `json:"design_partners_onboarded"`
	DesignPartnersAgreedToTest         int             `json:"design_partners_agreed_to_test"`
	PartnersWithRealTransactions       int             `json:"partners_with_real_transactions"`
	PricingCommitments                 int             `json:"pricing_commitments"`
	ExceptionCasesCollected            int             `json:"exception_cases_collected"`
	TestimonialsCollected              int             `json:"testimonials_collected"`
	ReconciledBusinessRecords          int             `json:"reconciled_business_records"`
	ReadyForPrivateBetaEvidence        bool            `json:"ready_for_private_beta_evidence"`
	RemainingDesignPartnersNeeded      int             `json:"remaining_design_partners_needed"`
	RemainingTransactionPartnersNeeded int             `json:"remaining_transaction_partners_needed"`
	RemainingPricingCommitmentsNeeded  int             `json:"remaining_pricing_commitments_needed"`
	GeneratedAt                        time.Time       `json:"generated_at"`
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
	Name       string `json:"name"`
	OwnerEmail string `json:"owner_email,omitempty"`
}

type CreateBusinessResult struct {
	Business Business `json:"business"`
	APIKey   string   `json:"api_key"`
	Owner    User     `json:"owner"`
}

type CreateAPIKeyInput struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type CreateAPIKeyResult struct {
	APIKey APIKey `json:"api_key"`
	Secret string `json:"secret"`
}

type CreateUserInput struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
	Role  string `json:"role"`
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

type CreateWebhookSubscriptionInput struct {
	URL        string   `json:"url"`
	Secret     string   `json:"secret,omitempty"`
	EventTypes []string `json:"event_types,omitempty"`
}

type CreateWebhookSubscriptionResult struct {
	WebhookSubscription WebhookSubscription `json:"webhook_subscription"`
	SigningSecret       string              `json:"signing_secret,omitempty"`
}

type CreateReconciliationRunInput struct {
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	WalletID    string    `json:"wallet_id,omitempty"`
}

type CreateExportInput struct {
	ReconciliationRunID string `json:"reconciliation_run_id"`
	Format              string `json:"format"`
}

type RecordAccessLogInput struct {
	BusinessID  string
	APIKeyID    string
	Method      string
	Path        string
	StatusCode  int
	RemoteAddr  string
	UserAgent   string
	DurationMS  int64
	RateLimited bool
}

type CreateIncidentInput struct {
	Title       string `json:"title"`
	Severity    string `json:"severity"`
	Description string `json:"description,omitempty"`
}

type ResolveIncidentInput struct {
	Summary string `json:"summary"`
}

type UpdateSecurityPolicyInput struct {
	RequireScopedAPIKeys   *bool `json:"require_scoped_api_keys,omitempty"`
	RateLimitPerMinute     *int  `json:"rate_limit_per_minute,omitempty"`
	DataRetentionDays      *int  `json:"data_retention_days,omitempty"`
	AccessLogRetentionDays *int  `json:"access_log_retention_days,omitempty"`
	WebhookRetentionDays   *int  `json:"webhook_retention_days,omitempty"`
	IncidentRetentionDays  *int  `json:"incident_retention_days,omitempty"`
}

type CreateDesignPartnerInput struct {
	CompanyName           string `json:"company_name"`
	Segment               string `json:"segment,omitempty"`
	ContactName           string `json:"contact_name,omitempty"`
	ContactEmail          string `json:"contact_email,omitempty"`
	UseCase               string `json:"use_case,omitempty"`
	Status                string `json:"status,omitempty"`
	AgreedToTest          bool   `json:"agreed_to_test"`
	PricingCommitment     bool   `json:"pricing_commitment"`
	ExpectedMonthlyVolume int    `json:"expected_monthly_volume,omitempty"`
	Notes                 string `json:"notes,omitempty"`
}

type UpdateDesignPartnerInput struct {
	Status                *string `json:"status,omitempty"`
	AgreedToTest          *bool   `json:"agreed_to_test,omitempty"`
	PricingCommitment     *bool   `json:"pricing_commitment,omitempty"`
	ExpectedMonthlyVolume *int    `json:"expected_monthly_volume,omitempty"`
	Notes                 *string `json:"notes,omitempty"`
}

type CreateBetaEvidenceInput struct {
	DesignPartnerID         string `json:"design_partner_id,omitempty"`
	Type                    string `json:"type"`
	Title                   string `json:"title"`
	Description             string `json:"description,omitempty"`
	PaymentRequestID        string `json:"payment_request_id,omitempty"`
	StablecoinTransactionID string `json:"stablecoin_transaction_id,omitempty"`
	ExceptionID             string `json:"exception_id,omitempty"`
	Quote                   string `json:"quote,omitempty"`
}

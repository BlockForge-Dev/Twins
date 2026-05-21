package api

import (
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"twins/internal/core"
)

type Server struct {
	store                 *core.MemoryStore
	mux                   *http.ServeMux
	rateMu                sync.Mutex
	rateWindows           map[string]rateWindow
	rateLimit             int
	businessCreationToken string
}

type ServerOptions struct {
	BusinessCreationToken string
}

type rateWindow struct {
	Count   int
	ResetAt time.Time
}

func NewServer(store *core.MemoryStore) http.Handler {
	return NewServerWithOptions(store, ServerOptions{})
}

func NewServerWithOptions(store *core.MemoryStore, options ServerOptions) http.Handler {
	server := &Server{
		store:                 store,
		mux:                   http.NewServeMux(),
		rateWindows:           make(map[string]rateWindow),
		rateLimit:             240,
		businessCreationToken: strings.TrimSpace(options.BusinessCreationToken),
	}
	server.routes()
	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
	recorder.Header().Set("X-Content-Type-Options", "nosniff")

	rateLimited := false
	if shouldRateLimit(r.URL.Path) {
		allowed, retryAfter := s.allowRequest(r)
		if !allowed {
			rateLimited = true
			recorder.Header().Set("Retry-After", retryAfter)
			writeJSON(recorder, http.StatusTooManyRequests, map[string]any{
				"error": core.RateLimited("rate limit exceeded"),
			})
			s.recordAccess(r, recorder.statusCode, time.Since(start), rateLimited)
			return
		}
	}

	s.mux.ServeHTTP(recorder, r)
	s.recordAccess(r, recorder.statusCode, time.Since(start), rateLimited)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/", s.handleRoot)
	s.mux.HandleFunc("/receipts/", s.handlePublicReceipt)
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/readyz", s.handleReady)
	s.mux.HandleFunc("/dashboard", s.handleDashboard)
	s.mux.HandleFunc("/v1/businesses", s.handleBusinesses)
	s.mux.HandleFunc("/v1/api-keys", s.handleAPIKeys)
	s.mux.HandleFunc("/v1/api-keys/", s.handleAPIKeyByID)
	s.mux.HandleFunc("/v1/users", s.handleUsers)
	s.mux.HandleFunc("/v1/wallets", s.handleWallets)
	s.mux.HandleFunc("/v1/payment-requests", s.handlePaymentRequests)
	s.mux.HandleFunc("/v1/payment-requests/", s.handlePaymentRequestByID)
	s.mux.HandleFunc("/v1/stablecoin-transactions", s.handleStablecoinTransactions)
	s.mux.HandleFunc("/v1/transaction-matches", s.handleTransactionMatches)
	s.mux.HandleFunc("/v1/exceptions", s.handleExceptions)
	s.mux.HandleFunc("/v1/exceptions/", s.handleExceptionByID)
	s.mux.HandleFunc("/v1/receipt-events", s.handleReceiptEvents)
	s.mux.HandleFunc("/v1/webhook-subscriptions", s.handleWebhookSubscriptions)
	s.mux.HandleFunc("/v1/webhook-deliveries", s.handleWebhookDeliveries)
	s.mux.HandleFunc("/v1/webhook-deliveries/", s.handleWebhookDeliveryByID)
	s.mux.HandleFunc("/v1/reconciliation-runs", s.handleReconciliationRuns)
	s.mux.HandleFunc("/v1/reconciliation-runs/", s.handleReconciliationRunByID)
	s.mux.HandleFunc("/v1/exports", s.handleExports)
	s.mux.HandleFunc("/v1/exports/", s.handleExportByID)
	s.mux.HandleFunc("/v1/audit-logs", s.handleAuditLogs)
	s.mux.HandleFunc("/v1/access-logs", s.handleAccessLogs)
	s.mux.HandleFunc("/v1/incidents", s.handleIncidents)
	s.mux.HandleFunc("/v1/incidents/", s.handleIncidentByID)
	s.mux.HandleFunc("/v1/security-policy", s.handleSecurityPolicy)
	s.mux.HandleFunc("/v1/design-partners", s.handleDesignPartners)
	s.mux.HandleFunc("/v1/design-partners/", s.handleDesignPartnerByID)
	s.mux.HandleFunc("/v1/beta-evidence", s.handleBetaEvidence)
	s.mux.HandleFunc("/v1/usage-metrics", s.handleUsageMetrics)
	s.mux.HandleFunc("/v1/private-beta-report", s.handlePrivateBetaReport)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, core.NotFound("route not found"))
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	persistent, _, err := s.store.PersistenceStatus()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":     "not_ready",
			"persistent": persistent,
			"error":      err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ready",
		"persistent": persistent,
	})
}

func (s *Server) handlePublicReceipt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/receipts/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, core.NotFound("receipt not found"))
		return
	}

	receipt, err := s.store.GetPublicReceipt(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]core.Receipt{"receipt": receipt})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := dashboardTemplate.Execute(w, nil); err != nil {
		http.Error(w, "dashboard render failed", http.StatusInternalServerError)
	}
}

func (s *Server) handleBusinesses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	if s.businessCreationToken != "" && r.Header.Get("X-Twins-Setup-Token") != s.businessCreationToken {
		writeError(w, core.Unauthorized("valid setup token is required to create businesses"))
		return
	}

	var input core.CreateBusinessInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, core.InvalidArgument(err.Error()))
		return
	}

	result, err := s.store.CreateBusiness(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		business, _, ok := s.authenticate(w, r, core.ScopeAPIKeysRead)
		if !ok {
			return
		}
		keys, err := s.store.ListAPIKeys(r.Context(), business.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_keys": keys})
	case http.MethodPost:
		business, apiKey, ok := s.authenticate(w, r, core.ScopeAPIKeysWrite)
		if !ok {
			return
		}
		var input core.CreateAPIKeyInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, core.InvalidArgument(err.Error()))
			return
		}
		result, err := s.store.CreateAPIKey(r.Context(), business.ID, apiKey.ID, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleAPIKeyByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	business, apiKey, ok := s.authenticate(w, r, core.ScopeAPIKeysWrite)
	if !ok {
		return
	}

	remaining := strings.TrimPrefix(r.URL.Path, "/v1/api-keys/")
	parts := strings.Split(remaining, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "revoke" {
		writeError(w, core.NotFound("api key route not found"))
		return
	}

	revoked, err := s.store.RevokeAPIKey(r.Context(), business.ID, apiKey.ID, parts[0])
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]core.APIKey{"api_key": revoked})
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		business, _, ok := s.authenticate(w, r, core.ScopeUsersRead)
		if !ok {
			return
		}
		users, err := s.store.ListUsers(r.Context(), business.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": users})
	case http.MethodPost:
		business, apiKey, ok := s.authenticate(w, r, core.ScopeUsersWrite)
		if !ok {
			return
		}
		var input core.CreateUserInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, core.InvalidArgument(err.Error()))
			return
		}
		user, err := s.store.CreateUser(r.Context(), business.ID, apiKey.ID, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]core.User{"user": user})
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleWallets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		business, _, ok := s.authenticate(w, r, core.ScopeWalletsRead)
		if !ok {
			return
		}
		wallets, err := s.store.ListWallets(r.Context(), business.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"wallets": wallets})
	case http.MethodPost:
		business, apiKey, ok := s.authenticate(w, r, core.ScopeWalletsWrite)
		if !ok {
			return
		}
		var input core.RegisterWalletInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, core.InvalidArgument(err.Error()))
			return
		}
		wallet, err := s.store.RegisterWallet(r.Context(), business.ID, apiKey.ID, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]core.Wallet{"wallet": wallet})
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handlePaymentRequests(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		business, _, ok := s.authenticate(w, r, core.ScopePaymentRequestsRead)
		if !ok {
			return
		}
		requests, err := s.store.ListPaymentRequests(r.Context(), business.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"payment_requests": requests})
	case http.MethodPost:
		business, apiKey, ok := s.authenticate(w, r, core.ScopePaymentRequestsWrite)
		if !ok {
			return
		}
		var input core.CreatePaymentRequestInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, core.InvalidArgument(err.Error()))
			return
		}
		result, err := s.store.CreatePaymentRequest(r.Context(), business.ID, apiKey.ID, r.Header.Get("Idempotency-Key"), input)
		if err != nil {
			writeError(w, err)
			return
		}
		if result.IdempotentReplayed {
			w.Header().Set("Idempotent-Replayed", "true")
			writeJSON(w, http.StatusOK, result)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handlePaymentRequestByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	business, _, ok := s.authenticate(w, r, core.ScopePaymentRequestsRead)
	if !ok {
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/v1/payment-requests/")
	if strings.HasSuffix(id, "/receipt") {
		paymentRequestID := strings.TrimSuffix(id, "/receipt")
		if paymentRequestID == "" || strings.Contains(paymentRequestID, "/") {
			writeError(w, core.NotFound("receipt not found"))
			return
		}
		receipt, err := s.store.GetReceipt(r.Context(), business.ID, paymentRequestID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]core.Receipt{"receipt": receipt})
		return
	}
	if id == "" || strings.Contains(id, "/") {
		writeError(w, core.NotFound("payment request not found"))
		return
	}

	paymentRequest, err := s.store.GetPaymentRequest(r.Context(), business.ID, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]core.PaymentRequest{"payment_request": paymentRequest})
}

func (s *Server) handleStablecoinTransactions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		business, _, ok := s.authenticate(w, r, core.ScopeTransactionsRead)
		if !ok {
			return
		}
		transactions, err := s.store.ListStablecoinTransactions(r.Context(), business.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"stablecoin_transactions": transactions})
	case http.MethodPost:
		business, apiKey, ok := s.authenticate(w, r, core.ScopeTransactionsWrite)
		if !ok {
			return
		}
		var input core.IngestStablecoinTransactionInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, core.InvalidArgument(err.Error()))
			return
		}
		result, err := s.store.IngestStablecoinTransaction(r.Context(), business.ID, apiKey.ID, input)
		if err != nil {
			writeError(w, err)
			return
		}
		if result.DuplicateReplayed {
			w.Header().Set("Duplicate-Replayed", "true")
			writeJSON(w, http.StatusOK, result)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleTransactionMatches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	business, _, ok := s.authenticate(w, r, core.ScopeMatchesRead)
	if !ok {
		return
	}

	matches, err := s.store.ListTransactionMatches(r.Context(), business.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transaction_matches": matches})
}

func (s *Server) handleExceptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	business, _, ok := s.authenticate(w, r, core.ScopeExceptionsRead)
	if !ok {
		return
	}

	exceptions, err := s.store.ListExceptions(r.Context(), business.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"exceptions": exceptions})
}

func (s *Server) handleExceptionByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	business, apiKey, ok := s.authenticate(w, r, core.ScopeExceptionsWrite)
	if !ok {
		return
	}

	remaining := strings.TrimPrefix(r.URL.Path, "/v1/exceptions/")
	parts := strings.Split(remaining, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "resolve" {
		writeError(w, core.NotFound("exception route not found"))
		return
	}

	var input core.ResolveExceptionInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, core.InvalidArgument(err.Error()))
		return
	}

	exception, err := s.store.ResolveException(r.Context(), business.ID, apiKey.ID, parts[0], input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]core.PaymentException{"exception": exception})
}

func (s *Server) handleReceiptEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	business, _, ok := s.authenticate(w, r, core.ScopeReceiptsRead)
	if !ok {
		return
	}

	events, err := s.store.ListReceiptEvents(r.Context(), business.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"receipt_events": events})
}

func (s *Server) handleWebhookSubscriptions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		business, _, ok := s.authenticate(w, r, core.ScopeWebhooksRead)
		if !ok {
			return
		}
		subscriptions, err := s.store.ListWebhookSubscriptions(r.Context(), business.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"webhook_subscriptions": subscriptions})
	case http.MethodPost:
		business, apiKey, ok := s.authenticate(w, r, core.ScopeWebhooksWrite)
		if !ok {
			return
		}
		var input core.CreateWebhookSubscriptionInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, core.InvalidArgument(err.Error()))
			return
		}
		result, err := s.store.CreateWebhookSubscription(r.Context(), business.ID, apiKey.ID, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	business, _, ok := s.authenticate(w, r, core.ScopeWebhooksRead)
	if !ok {
		return
	}

	deliveries, err := s.store.ListWebhookDeliveries(r.Context(), business.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhook_deliveries": deliveries})
}

func (s *Server) handleWebhookDeliveryByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	business, apiKey, ok := s.authenticate(w, r, core.ScopeWebhooksWrite)
	if !ok {
		return
	}

	remaining := strings.TrimPrefix(r.URL.Path, "/v1/webhook-deliveries/")
	parts := strings.Split(remaining, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "replay" {
		writeError(w, core.NotFound("webhook delivery route not found"))
		return
	}

	delivery, err := s.store.ReplayWebhookDelivery(r.Context(), business.ID, apiKey.ID, parts[0])
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]core.WebhookDelivery{"webhook_delivery": delivery})
}

func (s *Server) handleReconciliationRuns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		business, _, ok := s.authenticate(w, r, core.ScopeReconciliationRead)
		if !ok {
			return
		}
		runs, err := s.store.ListReconciliationRuns(r.Context(), business.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"reconciliation_runs": runs})
	case http.MethodPost:
		business, apiKey, ok := s.authenticate(w, r, core.ScopeReconciliationWrite)
		if !ok {
			return
		}
		var input core.CreateReconciliationRunInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, core.InvalidArgument(err.Error()))
			return
		}
		report, err := s.store.CreateReconciliationRun(r.Context(), business.ID, apiKey.ID, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]core.ReconciliationReport{"reconciliation_report": report})
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleReconciliationRunByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	business, _, ok := s.authenticate(w, r, core.ScopeReconciliationRead)
	if !ok {
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/v1/reconciliation-runs/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, core.NotFound("reconciliation run not found"))
		return
	}

	report, err := s.store.GetReconciliationReport(r.Context(), business.ID, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]core.ReconciliationReport{"reconciliation_report": report})
}

func (s *Server) handleExports(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		business, _, ok := s.authenticate(w, r, core.ScopeExportsRead)
		if !ok {
			return
		}
		exports, err := s.store.ListExports(r.Context(), business.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"exports": exports})
	case http.MethodPost:
		business, apiKey, ok := s.authenticate(w, r, core.ScopeExportsWrite)
		if !ok {
			return
		}
		var input core.CreateExportInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, core.InvalidArgument(err.Error()))
			return
		}
		export, err := s.store.CreateExport(r.Context(), business.ID, apiKey.ID, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]core.ExportRecord{"export": export})
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleExportByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	business, _, ok := s.authenticate(w, r, core.ScopeExportsRead)
	if !ok {
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/v1/exports/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, core.NotFound("export not found"))
		return
	}

	export, err := s.store.GetExport(r.Context(), business.ID, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]core.ExportRecord{"export": export})
}

func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	business, _, ok := s.authenticate(w, r, core.ScopeAuditLogsRead)
	if !ok {
		return
	}

	logs, err := s.store.ListAuditLogs(r.Context(), business.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit_logs": logs})
}

func (s *Server) handleAccessLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	business, _, ok := s.authenticate(w, r, core.ScopeAccessLogsRead)
	if !ok {
		return
	}

	logs, err := s.store.ListAccessLogs(r.Context(), business.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access_logs": logs})
}

func (s *Server) handleIncidents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		business, _, ok := s.authenticate(w, r, core.ScopeIncidentsRead)
		if !ok {
			return
		}
		incidents, err := s.store.ListIncidents(r.Context(), business.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"incidents": incidents})
	case http.MethodPost:
		business, apiKey, ok := s.authenticate(w, r, core.ScopeIncidentsWrite)
		if !ok {
			return
		}
		var input core.CreateIncidentInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, core.InvalidArgument(err.Error()))
			return
		}
		incident, err := s.store.CreateIncident(r.Context(), business.ID, apiKey.ID, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]core.Incident{"incident": incident})
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleIncidentByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	business, apiKey, ok := s.authenticate(w, r, core.ScopeIncidentsWrite)
	if !ok {
		return
	}

	remaining := strings.TrimPrefix(r.URL.Path, "/v1/incidents/")
	parts := strings.Split(remaining, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "resolve" {
		writeError(w, core.NotFound("incident route not found"))
		return
	}

	var input core.ResolveIncidentInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, core.InvalidArgument(err.Error()))
		return
	}
	incident, err := s.store.ResolveIncident(r.Context(), business.ID, apiKey.ID, parts[0], input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]core.Incident{"incident": incident})
}

func (s *Server) handleSecurityPolicy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		business, _, ok := s.authenticate(w, r, core.ScopeSecurityPolicyRead)
		if !ok {
			return
		}
		policy, err := s.store.GetSecurityPolicy(r.Context(), business.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]core.SecurityPolicy{"security_policy": policy})
	case http.MethodPatch:
		business, apiKey, ok := s.authenticate(w, r, core.ScopeSecurityPolicyWrite)
		if !ok {
			return
		}
		var input core.UpdateSecurityPolicyInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, core.InvalidArgument(err.Error()))
			return
		}
		policy, err := s.store.UpdateSecurityPolicy(r.Context(), business.ID, apiKey.ID, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]core.SecurityPolicy{"security_policy": policy})
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPatch)
	}
}

func (s *Server) handleDesignPartners(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		business, _, ok := s.authenticate(w, r, core.ScopeBetaRead)
		if !ok {
			return
		}
		partners, err := s.store.ListDesignPartners(r.Context(), business.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"design_partners": partners})
	case http.MethodPost:
		business, apiKey, ok := s.authenticate(w, r, core.ScopeBetaWrite)
		if !ok {
			return
		}
		var input core.CreateDesignPartnerInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, core.InvalidArgument(err.Error()))
			return
		}
		partner, err := s.store.CreateDesignPartner(r.Context(), business.ID, apiKey.ID, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]core.DesignPartner{"design_partner": partner})
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleDesignPartnerByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeMethodNotAllowed(w, http.MethodPatch)
		return
	}
	business, apiKey, ok := s.authenticate(w, r, core.ScopeBetaWrite)
	if !ok {
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/v1/design-partners/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, core.NotFound("design partner not found"))
		return
	}
	var input core.UpdateDesignPartnerInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, core.InvalidArgument(err.Error()))
		return
	}
	partner, err := s.store.UpdateDesignPartner(r.Context(), business.ID, apiKey.ID, id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]core.DesignPartner{"design_partner": partner})
}

func (s *Server) handleBetaEvidence(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		business, _, ok := s.authenticate(w, r, core.ScopeBetaRead)
		if !ok {
			return
		}
		evidence, err := s.store.ListBetaEvidence(r.Context(), business.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"beta_evidence": evidence})
	case http.MethodPost:
		business, apiKey, ok := s.authenticate(w, r, core.ScopeBetaWrite)
		if !ok {
			return
		}
		var input core.CreateBetaEvidenceInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, core.InvalidArgument(err.Error()))
			return
		}
		evidence, err := s.store.CreateBetaEvidence(r.Context(), business.ID, apiKey.ID, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]core.BetaEvidence{"beta_evidence": evidence})
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleUsageMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	business, _, ok := s.authenticate(w, r, core.ScopeUsageRead)
	if !ok {
		return
	}
	metrics, err := s.store.GetUsageMetrics(r.Context(), business.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]core.UsageMetrics{"usage_metrics": metrics})
}

func (s *Server) handlePrivateBetaReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	business, _, ok := s.authenticate(w, r, core.ScopeBetaRead)
	if !ok {
		return
	}
	report, err := s.store.GetPrivateBetaReport(r.Context(), business.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]core.PrivateBetaReport{"private_beta_report": report})
}

func (s *Server) authenticate(w http.ResponseWriter, r *http.Request, requiredScopes ...string) (core.Business, core.APIKey, bool) {
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	if raw == "" {
		writeError(w, core.Unauthorized("Authorization header is required"))
		return core.Business{}, core.APIKey{}, false
	}
	if !strings.HasPrefix(strings.ToLower(raw), "bearer ") {
		writeError(w, core.Unauthorized("Authorization header must use Bearer authentication"))
		return core.Business{}, core.APIKey{}, false
	}

	token := strings.TrimSpace(raw[len("Bearer "):])
	business, apiKey, err := s.store.AuthenticateAPIKey(r.Context(), token)
	if err != nil {
		writeError(w, err)
		return core.Business{}, core.APIKey{}, false
	}
	for _, scope := range requiredScopes {
		if !apiKeyHasScope(apiKey, scope) {
			writeError(w, core.Forbidden("API key missing required scope: "+scope))
			return core.Business{}, core.APIKey{}, false
		}
	}
	return business, apiKey, true
}

func readJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error) {
	var appErr *core.AppError
	if !errors.As(err, &appErr) {
		appErr = &core.AppError{Code: "internal", Message: "internal server error"}
	}

	status := http.StatusInternalServerError
	switch appErr.Code {
	case core.CodeInvalidArgument:
		status = http.StatusBadRequest
	case core.CodeUnauthorized:
		status = http.StatusUnauthorized
	case core.CodeForbidden:
		status = http.StatusForbidden
	case core.CodeNotFound:
		status = http.StatusNotFound
	case core.CodeConflict:
		status = http.StatusConflict
	case core.CodeRateLimited:
		status = http.StatusTooManyRequests
	}

	writeJSON(w, status, map[string]any{"error": appErr})
}

func writeMethodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
		"error": map[string]string{
			"code":    "method_not_allowed",
			"message": "method not allowed",
		},
	})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func shouldRateLimit(path string) bool {
	return strings.HasPrefix(path, "/v1/")
}

func (s *Server) allowRequest(r *http.Request) (bool, string) {
	now := time.Now().UTC()
	key := rateLimitKey(r)
	limit := s.rateLimit
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(raw), "bearer ") {
		token := strings.TrimSpace(raw[len("Bearer "):])
		if business, _, err := s.store.AuthenticateAPIKey(r.Context(), token); err == nil {
			if policy, err := s.store.GetSecurityPolicy(r.Context(), business.ID); err == nil && policy.RateLimitPerMinute > 0 {
				limit = policy.RateLimitPerMinute
			}
		}
	}

	s.rateMu.Lock()
	defer s.rateMu.Unlock()

	window := s.rateWindows[key]
	if window.ResetAt.IsZero() || !now.Before(window.ResetAt) {
		window = rateWindow{ResetAt: now.Add(time.Minute)}
	}
	window.Count++
	s.rateWindows[key] = window
	if window.Count > limit {
		retryAfter := int(time.Until(window.ResetAt).Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}
		return false, strconv.Itoa(retryAfter)
	}
	return true, ""
}

func rateLimitKey(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth != "" {
		return auth
	}
	return r.RemoteAddr
}

func (s *Server) recordAccess(r *http.Request, statusCode int, duration time.Duration, rateLimited bool) {
	if !strings.HasPrefix(r.URL.Path, "/v1/") {
		return
	}

	businessID := ""
	apiKeyID := ""
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(raw), "bearer ") {
		token := strings.TrimSpace(raw[len("Bearer "):])
		if business, apiKey, err := s.store.AuthenticateAPIKey(r.Context(), token); err == nil {
			businessID = business.ID
			apiKeyID = apiKey.ID
		}
	}

	s.store.RecordAccessLog(r.Context(), core.RecordAccessLogInput{
		BusinessID:  businessID,
		APIKeyID:    apiKeyID,
		Method:      r.Method,
		Path:        r.URL.Path,
		StatusCode:  statusCode,
		RemoteAddr:  r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		DurationMS:  duration.Milliseconds(),
		RateLimited: rateLimited,
	})
}

func apiKeyHasScope(apiKey core.APIKey, requiredScope string) bool {
	for _, scope := range apiKey.Scopes {
		if scope == core.ScopeAdmin || scope == requiredScope {
			return true
		}
	}
	return false
}

var dashboardTemplate = template.Must(template.New("dashboard").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Twins Dashboard</title>
  <style>
    :root {
      color-scheme: light;
      --ink: #18201c;
      --muted: #66736b;
      --line: #d8ded9;
      --panel: #ffffff;
      --bg: #f4f7f2;
      --accent: #0f7a5f;
      --accent-2: #294f8a;
      --warn: #b45309;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: var(--bg);
      color: var(--ink);
    }
    header {
      border-bottom: 1px solid var(--line);
      background: #ffffff;
      padding: 18px 24px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;
    }
    main {
      width: min(1180px, calc(100vw - 32px));
      margin: 24px auto;
    }
    h1 {
      font-size: 22px;
      margin: 0;
      letter-spacing: 0;
    }
    .muted { color: var(--muted); }
    .toolbar {
      display: grid;
      grid-template-columns: minmax(220px, 1fr) auto auto;
      gap: 10px;
      align-items: center;
      margin-bottom: 18px;
    }
    input {
      width: 100%;
      min-height: 40px;
      border: 1px solid var(--line);
      border-radius: 6px;
      padding: 0 12px;
      font: inherit;
      background: #ffffff;
      color: var(--ink);
    }
    button {
      min-height: 40px;
      border: 1px solid var(--accent);
      border-radius: 6px;
      background: var(--accent);
      color: #ffffff;
      padding: 0 14px;
      font: inherit;
      cursor: pointer;
    }
    button.secondary {
      background: #ffffff;
      color: var(--accent-2);
      border-color: var(--line);
    }
    section {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      overflow: hidden;
      margin-bottom: 18px;
    }
    .section-head {
      padding: 14px 16px;
      border-bottom: 1px solid var(--line);
      display: flex;
      justify-content: space-between;
      gap: 12px;
      align-items: center;
    }
    h2 {
      font-size: 15px;
      margin: 0;
      letter-spacing: 0;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      table-layout: fixed;
      font-size: 14px;
    }
    th, td {
      border-bottom: 1px solid var(--line);
      padding: 12px 14px;
      text-align: left;
      vertical-align: top;
      overflow-wrap: anywhere;
    }
    th {
      color: var(--muted);
      font-weight: 600;
      background: #fbfcfa;
    }
    tr:last-child td { border-bottom: 0; }
    .status {
      display: inline-block;
      border: 1px solid #b6d6c9;
      background: #e9f6f0;
      color: #0f5d49;
      border-radius: 999px;
      padding: 3px 8px;
      font-size: 12px;
      white-space: nowrap;
    }
    .empty, .error {
      padding: 28px 16px;
      color: var(--muted);
    }
    .error { color: var(--warn); }
    code { font-family: ui-monospace, SFMono-Regular, Consolas, "Liberation Mono", monospace; }
    @media (max-width: 720px) {
      header { align-items: flex-start; flex-direction: column; }
      .toolbar { grid-template-columns: 1fr; }
      table { min-width: 760px; }
      .table-wrap { overflow-x: auto; }
    }
  </style>
</head>
<body>
  <header>
    <div>
      <h1>Twins</h1>
      <div class="muted">Payment requests, chain evidence, receipts, webhooks, reconciliation, security controls, and private beta evidence</div>
    </div>
    <div class="muted" id="wallet-count"></div>
  </header>
  <main>
    <div class="toolbar">
      <input id="api-key" type="password" autocomplete="off" placeholder="API key">
      <button id="refresh">Refresh</button>
      <button class="secondary" id="clear">Clear</button>
    </div>
    <section>
      <div class="section-head">
        <h2>Payment Request List</h2>
        <span class="muted" id="updated-at"></span>
      </div>
      <div class="table-wrap" id="content">
        <div class="empty">Enter an API key to load payment requests.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Stablecoin Transaction Evidence</h2>
        <span class="muted" id="transaction-count"></span>
      </div>
      <div class="table-wrap" id="transaction-content">
        <div class="empty">Enter an API key to load transaction evidence.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Transaction Matches</h2>
        <span class="muted" id="match-count"></span>
      </div>
      <div class="table-wrap" id="match-content">
        <div class="empty">Enter an API key to load matches.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Receipt Timeline</h2>
        <span class="muted" id="receipt-count"></span>
      </div>
      <div class="table-wrap" id="receipt-content">
        <div class="empty">Enter an API key to load receipt events.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Webhook Delivery Logs</h2>
        <span class="muted" id="webhook-count"></span>
      </div>
      <div class="table-wrap" id="webhook-content">
        <div class="empty">Enter an API key to load webhook deliveries.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Reconciliation Runs</h2>
        <span class="muted" id="reconciliation-count"></span>
      </div>
      <div class="table-wrap" id="reconciliation-content">
        <div class="empty">Enter an API key to load reconciliation runs.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Wallet Snapshots</h2>
        <span class="muted" id="wallet-snapshot-count"></span>
      </div>
      <div class="table-wrap" id="wallet-snapshot-content">
        <div class="empty">Run reconciliation to load wallet snapshots.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Settlement Report</h2>
        <span class="muted" id="settlement-count"></span>
      </div>
      <div class="table-wrap" id="settlement-content">
        <div class="empty">Run reconciliation to load settlement rows.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Exports</h2>
        <span class="muted" id="export-count"></span>
      </div>
      <div class="table-wrap" id="export-content">
        <div class="empty">Enter an API key to load exports.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Exceptions</h2>
        <span class="muted" id="exception-count"></span>
      </div>
      <div class="table-wrap" id="exception-content">
        <div class="empty">Enter an API key to load exceptions.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Security Policy</h2>
        <span class="muted" id="security-policy-count"></span>
      </div>
      <div class="table-wrap" id="security-policy-content">
        <div class="empty">Enter an API key to load security policy.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>API Keys</h2>
        <span class="muted" id="api-key-count"></span>
      </div>
      <div class="table-wrap" id="api-key-content">
        <div class="empty">Enter an API key to load API keys.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Users</h2>
        <span class="muted" id="user-count"></span>
      </div>
      <div class="table-wrap" id="user-content">
        <div class="empty">Enter an API key to load users.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Incidents</h2>
        <span class="muted" id="incident-count"></span>
      </div>
      <div class="table-wrap" id="incident-content">
        <div class="empty">Enter an API key to load incidents.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Access Logs</h2>
        <span class="muted" id="access-log-count"></span>
      </div>
      <div class="table-wrap" id="access-log-content">
        <div class="empty">Enter an API key to load access logs.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Usage Metrics</h2>
        <span class="muted" id="usage-metric-count"></span>
      </div>
      <div class="table-wrap" id="usage-metric-content">
        <div class="empty">Enter an API key to load usage metrics.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Design Partners</h2>
        <span class="muted" id="design-partner-count"></span>
      </div>
      <div class="table-wrap" id="design-partner-content">
        <div class="empty">Enter an API key to load design partners.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Beta Evidence</h2>
        <span class="muted" id="beta-evidence-count"></span>
      </div>
      <div class="table-wrap" id="beta-evidence-content">
        <div class="empty">Enter an API key to load beta evidence.</div>
      </div>
    </section>
    <section>
      <div class="section-head">
        <h2>Private Beta Readiness</h2>
        <span class="muted" id="private-beta-count"></span>
      </div>
      <div class="table-wrap" id="private-beta-content">
        <div class="empty">Enter an API key to load private beta readiness.</div>
      </div>
    </section>
  </main>
  <script>
    const keyInput = document.querySelector("#api-key");
    const refreshButton = document.querySelector("#refresh");
    const clearButton = document.querySelector("#clear");
    const content = document.querySelector("#content");
    const transactionContent = document.querySelector("#transaction-content");
    const matchContent = document.querySelector("#match-content");
    const receiptContent = document.querySelector("#receipt-content");
    const webhookContent = document.querySelector("#webhook-content");
    const reconciliationContent = document.querySelector("#reconciliation-content");
    const walletSnapshotContent = document.querySelector("#wallet-snapshot-content");
    const settlementContent = document.querySelector("#settlement-content");
    const exportContent = document.querySelector("#export-content");
    const exceptionContent = document.querySelector("#exception-content");
    const securityPolicyContent = document.querySelector("#security-policy-content");
    const apiKeyContent = document.querySelector("#api-key-content");
    const userContent = document.querySelector("#user-content");
    const incidentContent = document.querySelector("#incident-content");
    const accessLogContent = document.querySelector("#access-log-content");
    const usageMetricContent = document.querySelector("#usage-metric-content");
    const designPartnerContent = document.querySelector("#design-partner-content");
    const betaEvidenceContent = document.querySelector("#beta-evidence-content");
    const privateBetaContent = document.querySelector("#private-beta-content");
    const updatedAt = document.querySelector("#updated-at");
    const transactionCount = document.querySelector("#transaction-count");
    const matchCount = document.querySelector("#match-count");
    const receiptCount = document.querySelector("#receipt-count");
    const webhookCount = document.querySelector("#webhook-count");
    const reconciliationCount = document.querySelector("#reconciliation-count");
    const walletSnapshotCount = document.querySelector("#wallet-snapshot-count");
    const settlementCount = document.querySelector("#settlement-count");
    const exportCount = document.querySelector("#export-count");
    const exceptionCount = document.querySelector("#exception-count");
    const securityPolicyCount = document.querySelector("#security-policy-count");
    const apiKeyCount = document.querySelector("#api-key-count");
    const userCount = document.querySelector("#user-count");
    const incidentCount = document.querySelector("#incident-count");
    const accessLogCount = document.querySelector("#access-log-count");
    const usageMetricCount = document.querySelector("#usage-metric-count");
    const designPartnerCount = document.querySelector("#design-partner-count");
    const betaEvidenceCount = document.querySelector("#beta-evidence-count");
    const privateBetaCount = document.querySelector("#private-beta-count");
    const walletCount = document.querySelector("#wallet-count");

    keyInput.value = localStorage.getItem("twins_api_key") || "";

    function esc(value) {
      return String(value ?? "").replace(/[&<>"']/g, function (char) {
        return {"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[char];
      });
    }

    async function api(path) {
      const key = keyInput.value.trim();
      if (!key) throw new Error("API key is required");
      localStorage.setItem("twins_api_key", key);
      const response = await fetch(path, {
        headers: { "Authorization": "Bearer " + key }
      });
      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error?.message || "Request failed");
      }
      return data;
    }

    function renderRows(rows) {
      if (!rows.length) {
        content.innerHTML = '<div class="empty">No payment requests yet.</div>';
        return;
      }
      content.innerHTML = '<table><thead><tr><th>ID</th><th>Status</th><th>Amount</th><th>Customer</th><th>Invoice</th><th>Destination</th><th>Expires</th></tr></thead><tbody>' +
        rows.map(row => '<tr>' +
          '<td><code>' + esc(row.id) + '</code></td>' +
          '<td><span class="status">' + esc(row.status) + '</span></td>' +
          '<td>' + esc(row.amount) + ' ' + esc(row.token) + '</td>' +
          '<td>' + esc(row.customer_id) + '</td>' +
          '<td>' + esc(row.invoice_id) + '</td>' +
          '<td><code>' + esc(row.destination_address) + '</code></td>' +
          '<td>' + esc(new Date(row.expires_at).toLocaleString()) + '</td>' +
        '</tr>').join("") +
        '</tbody></table>';
    }

    function renderTransactions(rows) {
      transactionCount.textContent = rows.length + " transaction" + (rows.length === 1 ? "" : "s");
      if (!rows.length) {
        transactionContent.innerHTML = '<div class="empty">No stablecoin transaction evidence yet.</div>';
        return;
      }
      transactionContent.innerHTML = '<table><thead><tr><th>Signature</th><th>Status</th><th>Amount</th><th>Finality</th><th>Destination Owner</th><th>Slot</th></tr></thead><tbody>' +
        rows.map(row => '<tr>' +
          '<td><code>' + esc(row.signature) + '</code></td>' +
          '<td><span class="status">' + esc(row.status) + '</span></td>' +
          '<td>' + esc(row.amount) + ' ' + esc(row.token) + '</td>' +
          '<td>' + esc(row.confirmation_status) + '</td>' +
          '<td><code>' + esc(row.destination_owner) + '</code></td>' +
          '<td>' + esc(row.slot) + '</td>' +
        '</tr>').join("") +
        '</tbody></table>';
    }

    function renderMatches(rows) {
      matchCount.textContent = rows.length + " match" + (rows.length === 1 ? "" : "es");
      if (!rows.length) {
        matchContent.innerHTML = '<div class="empty">No transaction matches yet.</div>';
        return;
      }
      matchContent.innerHTML = '<table><thead><tr><th>ID</th><th>Status</th><th>Request</th><th>Transaction</th><th>Expected</th><th>Received</th><th>Reason</th></tr></thead><tbody>' +
        rows.map(row => '<tr>' +
          '<td><code>' + esc(row.id) + '</code></td>' +
          '<td><span class="status">' + esc(row.status) + '</span></td>' +
          '<td><code>' + esc(row.payment_request_id) + '</code></td>' +
          '<td><code>' + esc(row.stablecoin_transaction_id) + '</code></td>' +
          '<td>' + esc(row.expected_amount) + '</td>' +
          '<td>' + esc(row.received_amount) + '</td>' +
          '<td>' + esc(row.reason) + '</td>' +
        '</tr>').join("") +
        '</tbody></table>';
    }

    function renderReceiptEvents(rows) {
      receiptCount.textContent = rows.length + " event" + (rows.length === 1 ? "" : "s");
      if (!rows.length) {
        receiptContent.innerHTML = '<div class="empty">No receipt events yet.</div>';
        return;
      }
      receiptContent.innerHTML = '<table><thead><tr><th>Event</th><th>Status</th><th>Request</th><th>Transaction</th><th>Exception</th><th>Description</th><th>Created</th></tr></thead><tbody>' +
        rows.map(row => '<tr>' +
          '<td>' + esc(row.type) + '</td>' +
          '<td><span class="status">' + esc(row.status) + '</span></td>' +
          '<td><code>' + esc(row.payment_request_id) + '</code></td>' +
          '<td><code>' + esc(row.stablecoin_transaction_id || "") + '</code></td>' +
          '<td><code>' + esc(row.exception_id || "") + '</code></td>' +
          '<td>' + esc(row.description) + '</td>' +
          '<td>' + esc(new Date(row.created_at).toLocaleString()) + '</td>' +
        '</tr>').join("") +
        '</tbody></table>';
    }

    function renderWebhookDeliveries(rows) {
      webhookCount.textContent = rows.length + " " + (rows.length === 1 ? "delivery" : "deliveries");
      if (!rows.length) {
        webhookContent.innerHTML = '<div class="empty">No webhook deliveries yet.</div>';
        return;
      }
      webhookContent.innerHTML = '<table><thead><tr><th>ID</th><th>Status</th><th>Event</th><th>Resource</th><th>Attempts</th><th>HTTP</th><th>Error</th></tr></thead><tbody>' +
        rows.map(row => '<tr>' +
          '<td><code>' + esc(row.id) + '</code></td>' +
          '<td><span class="status">' + esc(row.status) + '</span></td>' +
          '<td>' + esc(row.event_type) + '</td>' +
          '<td><code>' + esc(row.resource_type) + ':' + esc(row.resource_id) + '</code></td>' +
          '<td>' + esc(row.attempts) + '</td>' +
          '<td>' + esc(row.last_status_code || "") + '</td>' +
          '<td>' + esc(row.last_error || "") + '</td>' +
        '</tr>').join("") +
        '</tbody></table>';
    }

    function renderReconciliationRuns(rows) {
      reconciliationCount.textContent = rows.length + " run" + (rows.length === 1 ? "" : "s");
      if (!rows.length) {
        reconciliationContent.innerHTML = '<div class="empty">No reconciliation runs yet.</div>';
        return;
      }
      reconciliationContent.innerHTML = '<table><thead><tr><th>ID</th><th>Status</th><th>Period</th><th>Requests</th><th>Transactions</th><th>Matched</th><th>Exceptions</th><th>Total USDC</th></tr></thead><tbody>' +
        rows.map(row => '<tr>' +
          '<td><code>' + esc(row.id) + '</code></td>' +
          '<td><span class="status">' + esc(row.status) + '</span></td>' +
          '<td>' + esc(new Date(row.period_start).toLocaleString()) + '<br>' + esc(new Date(row.period_end).toLocaleString()) + '</td>' +
          '<td>' + esc(row.total_payment_requests) + '</td>' +
          '<td>' + esc(row.total_transactions) + '</td>' +
          '<td>' + esc(row.matched_transactions) + '</td>' +
          '<td>' + esc(row.exception_count) + '</td>' +
          '<td>' + esc(row.total_received_usdc) + ' USDC</td>' +
        '</tr>').join("") +
        '</tbody></table>';
    }

    function renderWalletSnapshots(rows) {
      walletSnapshotCount.textContent = rows.length + " snapshot" + (rows.length === 1 ? "" : "s");
      if (!rows.length) {
        walletSnapshotContent.innerHTML = '<div class="empty">No wallet snapshots yet.</div>';
        return;
      }
      walletSnapshotContent.innerHTML = '<table><thead><tr><th>Wallet</th><th>Chain</th><th>Token</th><th>Observed Inbound</th><th>Transactions</th><th>Captured</th></tr></thead><tbody>' +
        rows.map(row => '<tr>' +
          '<td><code>' + esc(row.wallet_address) + '</code></td>' +
          '<td>' + esc(row.chain) + '</td>' +
          '<td>' + esc(row.token) + '</td>' +
          '<td>' + esc(row.observed_inbound_amount) + '</td>' +
          '<td>' + esc(row.transaction_count) + '</td>' +
          '<td>' + esc(new Date(row.captured_at).toLocaleString()) + '</td>' +
        '</tr>').join("") +
        '</tbody></table>';
    }

    function renderSettlementRows(rows) {
      settlementCount.textContent = rows.length + " row" + (rows.length === 1 ? "" : "s");
      if (!rows.length) {
        settlementContent.innerHTML = '<div class="empty">No settlement rows yet.</div>';
        return;
      }
      settlementContent.innerHTML = '<table><thead><tr><th>Status</th><th>Request</th><th>Invoice</th><th>Expected</th><th>Received</th><th>Transaction</th><th>Match</th><th>Exception</th></tr></thead><tbody>' +
        rows.map(row => '<tr>' +
          '<td><span class="status">' + esc(row.reconciliation_status) + '</span></td>' +
          '<td><code>' + esc(row.payment_request_id || "") + '</code></td>' +
          '<td>' + esc(row.invoice_id || "") + '</td>' +
          '<td>' + esc(row.expected_amount || "") + '</td>' +
          '<td>' + esc(row.received_amount || "") + '</td>' +
          '<td><code>' + esc(row.stablecoin_transaction_id || "") + '</code></td>' +
          '<td>' + esc(row.match_status || "") + '</td>' +
          '<td>' + esc(row.exception_type || "") + '</td>' +
        '</tr>').join("") +
        '</tbody></table>';
    }

    function renderExports(rows) {
      exportCount.textContent = rows.length + " export" + (rows.length === 1 ? "" : "s");
      if (!rows.length) {
        exportContent.innerHTML = '<div class="empty">No exports yet.</div>';
        return;
      }
      exportContent.innerHTML = '<table><thead><tr><th>ID</th><th>Status</th><th>Format</th><th>File</th><th>Rows</th><th>Run</th><th>Created</th></tr></thead><tbody>' +
        rows.map(row => '<tr>' +
          '<td><code>' + esc(row.id) + '</code></td>' +
          '<td><span class="status">' + esc(row.status) + '</span></td>' +
          '<td>' + esc(row.format) + '</td>' +
          '<td>' + esc(row.file_name) + '</td>' +
          '<td>' + esc(row.row_count) + '</td>' +
          '<td><code>' + esc(row.reconciliation_run_id) + '</code></td>' +
          '<td>' + esc(new Date(row.created_at).toLocaleString()) + '</td>' +
        '</tr>').join("") +
        '</tbody></table>';
    }

    function renderExceptions(rows) {
      exceptionCount.textContent = rows.length + " exception" + (rows.length === 1 ? "" : "s");
      if (!rows.length) {
        exceptionContent.innerHTML = '<div class="empty">No open or resolved exceptions yet.</div>';
        return;
      }
      exceptionContent.innerHTML = '<table><thead><tr><th>ID</th><th>Type</th><th>Status</th><th>Severity</th><th>Request</th><th>Transaction</th><th>Reason</th></tr></thead><tbody>' +
        rows.map(row => '<tr>' +
          '<td><code>' + esc(row.id) + '</code></td>' +
          '<td>' + esc(row.type) + '</td>' +
          '<td><span class="status">' + esc(row.status) + '</span></td>' +
          '<td>' + esc(row.severity) + '</td>' +
          '<td><code>' + esc(row.payment_request_id || "") + '</code></td>' +
          '<td><code>' + esc(row.stablecoin_transaction_id || "") + '</code></td>' +
          '<td>' + esc(row.reason) + '</td>' +
        '</tr>').join("") +
        '</tbody></table>';
    }

    function renderSecurityPolicy(policy) {
      if (!policy || !policy.id) {
        securityPolicyCount.textContent = "";
        securityPolicyContent.innerHTML = '<div class="empty">No security policy yet.</div>';
        return;
      }
      securityPolicyCount.textContent = "rate " + esc(policy.rate_limit_per_minute) + "/min";
      securityPolicyContent.innerHTML = '<table><thead><tr><th>Scoped Keys</th><th>Rate Limit</th><th>Data Retention</th><th>Access Logs</th><th>Webhooks</th><th>Incidents</th><th>Updated</th></tr></thead><tbody><tr>' +
        '<td>' + esc(policy.require_scoped_api_keys ? "required" : "optional") + '</td>' +
        '<td>' + esc(policy.rate_limit_per_minute) + '/min</td>' +
        '<td>' + esc(policy.data_retention_days) + ' days</td>' +
        '<td>' + esc(policy.access_log_retention_days) + ' days</td>' +
        '<td>' + esc(policy.webhook_retention_days) + ' days</td>' +
        '<td>' + esc(policy.incident_retention_days) + ' days</td>' +
        '<td>' + esc(new Date(policy.updated_at).toLocaleString()) + '</td>' +
        '</tr></tbody></table>';
    }

    function renderAPIKeys(rows) {
      apiKeyCount.textContent = rows.length + " key" + (rows.length === 1 ? "" : "s");
      if (!rows.length) {
        apiKeyContent.innerHTML = '<div class="empty">No API keys yet.</div>';
        return;
      }
      apiKeyContent.innerHTML = '<table><thead><tr><th>ID</th><th>Name</th><th>Prefix</th><th>Scopes</th><th>Status</th><th>Created</th></tr></thead><tbody>' +
        rows.map(row => '<tr>' +
          '<td><code>' + esc(row.id) + '</code></td>' +
          '<td>' + esc(row.name) + '</td>' +
          '<td><code>' + esc(row.prefix) + '</code></td>' +
          '<td>' + esc((row.scopes || []).join(", ")) + '</td>' +
          '<td><span class="status">' + esc(row.revoked_at ? "revoked" : "active") + '</span></td>' +
          '<td>' + esc(new Date(row.created_at).toLocaleString()) + '</td>' +
        '</tr>').join("") +
        '</tbody></table>';
    }

    function renderUsers(rows) {
      userCount.textContent = rows.length + " user" + (rows.length === 1 ? "" : "s");
      if (!rows.length) {
        userContent.innerHTML = '<div class="empty">No users yet.</div>';
        return;
      }
      userContent.innerHTML = '<table><thead><tr><th>Email</th><th>Name</th><th>Role</th><th>Status</th><th>Created</th></tr></thead><tbody>' +
        rows.map(row => '<tr>' +
          '<td>' + esc(row.email) + '</td>' +
          '<td>' + esc(row.name || "") + '</td>' +
          '<td><span class="status">' + esc(row.role) + '</span></td>' +
          '<td>' + esc(row.status) + '</td>' +
          '<td>' + esc(new Date(row.created_at).toLocaleString()) + '</td>' +
        '</tr>').join("") +
        '</tbody></table>';
    }

    function renderIncidents(rows) {
      incidentCount.textContent = rows.length + " incident" + (rows.length === 1 ? "" : "s");
      if (!rows.length) {
        incidentContent.innerHTML = '<div class="empty">No incidents yet.</div>';
        return;
      }
      incidentContent.innerHTML = '<table><thead><tr><th>ID</th><th>Status</th><th>Severity</th><th>Title</th><th>Resolution</th><th>Created</th></tr></thead><tbody>' +
        rows.map(row => '<tr>' +
          '<td><code>' + esc(row.id) + '</code></td>' +
          '<td><span class="status">' + esc(row.status) + '</span></td>' +
          '<td>' + esc(row.severity) + '</td>' +
          '<td>' + esc(row.title) + '</td>' +
          '<td>' + esc(row.resolution_summary || "") + '</td>' +
          '<td>' + esc(new Date(row.created_at).toLocaleString()) + '</td>' +
        '</tr>').join("") +
        '</tbody></table>';
    }

    function renderAccessLogs(rows) {
      accessLogCount.textContent = rows.length + " log" + (rows.length === 1 ? "" : "s");
      if (!rows.length) {
        accessLogContent.innerHTML = '<div class="empty">No access logs yet.</div>';
        return;
      }
      accessLogContent.innerHTML = '<table><thead><tr><th>Method</th><th>Path</th><th>Status</th><th>Key</th><th>Duration</th><th>Rate Limited</th><th>Accessed</th></tr></thead><tbody>' +
        rows.slice(0, 25).map(row => '<tr>' +
          '<td>' + esc(row.method) + '</td>' +
          '<td><code>' + esc(row.path) + '</code></td>' +
          '<td>' + esc(row.status_code) + '</td>' +
          '<td><code>' + esc(row.api_key_id || "") + '</code></td>' +
          '<td>' + esc(row.duration_ms) + 'ms</td>' +
          '<td>' + esc(row.rate_limited ? "yes" : "no") + '</td>' +
          '<td>' + esc(new Date(row.accessed_at).toLocaleString()) + '</td>' +
        '</tr>').join("") +
        '</tbody></table>';
    }

    function renderUsageMetrics(metrics) {
      if (!metrics || !metrics.business_id) {
        usageMetricCount.textContent = "";
        usageMetricContent.innerHTML = '<div class="empty">No usage metrics yet.</div>';
        return;
      }
      usageMetricCount.textContent = esc(metrics.reconciled_business_records || 0) + " reconciled records";
      usageMetricContent.innerHTML = '<table><thead><tr><th>Requests</th><th>Transactions</th><th>Matched</th><th>Receipts</th><th>Exports</th><th>Partners</th><th>Evidence</th><th>Pricing</th></tr></thead><tbody><tr>' +
        '<td>' + esc(metrics.payment_requests_created) + '</td>' +
        '<td>' + esc(metrics.transactions_detected) + '</td>' +
        '<td>' + esc(metrics.transactions_matched) + '</td>' +
        '<td>' + esc(metrics.receipts_generated) + '</td>' +
        '<td>' + esc(metrics.settlement_reports_exported) + '</td>' +
        '<td>' + esc(metrics.design_partners) + '</td>' +
        '<td>' + esc(metrics.beta_evidence_items) + '</td>' +
        '<td>' + esc(metrics.pricing_commitments) + '</td>' +
        '</tr></tbody></table>';
    }

    function renderDesignPartners(rows) {
      designPartnerCount.textContent = rows.length + " partner" + (rows.length === 1 ? "" : "s");
      if (!rows.length) {
        designPartnerContent.innerHTML = '<div class="empty">No design partners yet.</div>';
        return;
      }
      designPartnerContent.innerHTML = '<table><thead><tr><th>Company</th><th>Status</th><th>Segment</th><th>Contact</th><th>Agreed</th><th>Pricing</th><th>Expected Volume</th></tr></thead><tbody>' +
        rows.map(row => '<tr>' +
          '<td>' + esc(row.company_name) + '</td>' +
          '<td><span class="status">' + esc(row.status) + '</span></td>' +
          '<td>' + esc(row.segment || "") + '</td>' +
          '<td>' + esc(row.contact_name || "") + '<br>' + esc(row.contact_email || "") + '</td>' +
          '<td>' + esc(row.agreed_to_test ? "yes" : "no") + '</td>' +
          '<td>' + esc(row.pricing_commitment ? "yes" : "no") + '</td>' +
          '<td>' + esc(row.expected_monthly_volume || 0) + '</td>' +
        '</tr>').join("") +
        '</tbody></table>';
    }

    function renderBetaEvidence(rows) {
      betaEvidenceCount.textContent = rows.length + " item" + (rows.length === 1 ? "" : "s");
      if (!rows.length) {
        betaEvidenceContent.innerHTML = '<div class="empty">No beta evidence yet.</div>';
        return;
      }
      betaEvidenceContent.innerHTML = '<table><thead><tr><th>Type</th><th>Title</th><th>Partner</th><th>Request</th><th>Transaction</th><th>Quote</th><th>Created</th></tr></thead><tbody>' +
        rows.map(row => '<tr>' +
          '<td><span class="status">' + esc(row.type) + '</span></td>' +
          '<td>' + esc(row.title) + '</td>' +
          '<td><code>' + esc(row.design_partner_id || "") + '</code></td>' +
          '<td><code>' + esc(row.payment_request_id || "") + '</code></td>' +
          '<td><code>' + esc(row.stablecoin_transaction_id || "") + '</code></td>' +
          '<td>' + esc(row.quote || "") + '</td>' +
          '<td>' + esc(new Date(row.created_at).toLocaleString()) + '</td>' +
        '</tr>').join("") +
        '</tbody></table>';
    }

    function renderPrivateBetaReport(report) {
      if (!report || !report.business_id) {
        privateBetaCount.textContent = "";
        privateBetaContent.innerHTML = '<div class="empty">No private beta report yet.</div>';
        return;
      }
      privateBetaCount.textContent = report.ready_for_private_beta_evidence ? "evidence target met" : "evidence target open";
      privateBetaContent.innerHTML = '<table><thead><tr><th>Onboarded</th><th>Agreed</th><th>Tx Partners</th><th>Pricing</th><th>Exception Cases</th><th>Testimonials</th><th>Remaining Partners</th><th>Ready</th></tr></thead><tbody><tr>' +
        '<td>' + esc(report.design_partners_onboarded) + '</td>' +
        '<td>' + esc(report.design_partners_agreed_to_test) + '</td>' +
        '<td>' + esc(report.partners_with_real_transactions) + '</td>' +
        '<td>' + esc(report.pricing_commitments) + '</td>' +
        '<td>' + esc(report.exception_cases_collected) + '</td>' +
        '<td>' + esc(report.testimonials_collected) + '</td>' +
        '<td>' + esc(report.remaining_design_partners_needed) + '</td>' +
        '<td><span class="status">' + esc(report.ready_for_private_beta_evidence ? "yes" : "not yet") + '</span></td>' +
        '</tr></tbody></table>';
    }

    async function refresh() {
      content.innerHTML = '<div class="empty">Loading...</div>';
      transactionContent.innerHTML = '<div class="empty">Loading...</div>';
      matchContent.innerHTML = '<div class="empty">Loading...</div>';
      receiptContent.innerHTML = '<div class="empty">Loading...</div>';
      webhookContent.innerHTML = '<div class="empty">Loading...</div>';
      reconciliationContent.innerHTML = '<div class="empty">Loading...</div>';
      walletSnapshotContent.innerHTML = '<div class="empty">Loading...</div>';
      settlementContent.innerHTML = '<div class="empty">Loading...</div>';
      exportContent.innerHTML = '<div class="empty">Loading...</div>';
      exceptionContent.innerHTML = '<div class="empty">Loading...</div>';
      securityPolicyContent.innerHTML = '<div class="empty">Loading...</div>';
      apiKeyContent.innerHTML = '<div class="empty">Loading...</div>';
      userContent.innerHTML = '<div class="empty">Loading...</div>';
      incidentContent.innerHTML = '<div class="empty">Loading...</div>';
      accessLogContent.innerHTML = '<div class="empty">Loading...</div>';
      usageMetricContent.innerHTML = '<div class="empty">Loading...</div>';
      designPartnerContent.innerHTML = '<div class="empty">Loading...</div>';
      betaEvidenceContent.innerHTML = '<div class="empty">Loading...</div>';
      privateBetaContent.innerHTML = '<div class="empty">Loading...</div>';
      walletCount.textContent = "";
      transactionCount.textContent = "";
      matchCount.textContent = "";
      receiptCount.textContent = "";
      webhookCount.textContent = "";
      reconciliationCount.textContent = "";
      walletSnapshotCount.textContent = "";
      settlementCount.textContent = "";
      exportCount.textContent = "";
      exceptionCount.textContent = "";
      securityPolicyCount.textContent = "";
      apiKeyCount.textContent = "";
      userCount.textContent = "";
      incidentCount.textContent = "";
      accessLogCount.textContent = "";
      usageMetricCount.textContent = "";
      designPartnerCount.textContent = "";
      betaEvidenceCount.textContent = "";
      privateBetaCount.textContent = "";
      try {
        const [requestData, walletData, transactionData, matchData, receiptData, webhookData, reconciliationData, exportData, exceptionData, policyData, apiKeyData, userData, incidentData, accessLogData, usageData, designPartnerData, betaEvidenceData, privateBetaData] = await Promise.all([
          api("/v1/payment-requests"),
          api("/v1/wallets"),
          api("/v1/stablecoin-transactions"),
          api("/v1/transaction-matches"),
          api("/v1/receipt-events"),
          api("/v1/webhook-deliveries"),
          api("/v1/reconciliation-runs"),
          api("/v1/exports"),
          api("/v1/exceptions"),
          api("/v1/security-policy"),
          api("/v1/api-keys"),
          api("/v1/users"),
          api("/v1/incidents"),
          api("/v1/access-logs"),
          api("/v1/usage-metrics"),
          api("/v1/design-partners"),
          api("/v1/beta-evidence"),
          api("/v1/private-beta-report")
        ]);
        renderRows(requestData.payment_requests || []);
        renderTransactions(transactionData.stablecoin_transactions || []);
        renderMatches(matchData.transaction_matches || []);
        renderReceiptEvents(receiptData.receipt_events || []);
        renderWebhookDeliveries(webhookData.webhook_deliveries || []);
        const runs = reconciliationData.reconciliation_runs || [];
        renderReconciliationRuns(runs);
        renderExports(exportData.exports || []);
        if (runs.length) {
          const latestReport = await api("/v1/reconciliation-runs/" + encodeURIComponent(runs[0].id));
          const report = latestReport.reconciliation_report || {};
          renderWalletSnapshots(report.wallet_snapshots || []);
          renderSettlementRows(report.rows || []);
        } else {
          renderWalletSnapshots([]);
          renderSettlementRows([]);
        }
        renderExceptions(exceptionData.exceptions || []);
        renderSecurityPolicy(policyData.security_policy || {});
        renderAPIKeys(apiKeyData.api_keys || []);
        renderUsers(userData.users || []);
        renderIncidents(incidentData.incidents || []);
        renderAccessLogs(accessLogData.access_logs || []);
        renderUsageMetrics(usageData.usage_metrics || {});
        renderDesignPartners(designPartnerData.design_partners || []);
        renderBetaEvidence(betaEvidenceData.beta_evidence || []);
        renderPrivateBetaReport(privateBetaData.private_beta_report || {});
        const wallets = walletData.wallets || [];
        walletCount.textContent = wallets.length + " wallet" + (wallets.length === 1 ? "" : "s");
        updatedAt.textContent = "Updated " + new Date().toLocaleTimeString();
      } catch (err) {
        content.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        transactionContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        matchContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        receiptContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        webhookContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        reconciliationContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        walletSnapshotContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        settlementContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        exportContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        exceptionContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        securityPolicyContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        apiKeyContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        userContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        incidentContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        accessLogContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        usageMetricContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        designPartnerContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        betaEvidenceContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        privateBetaContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
      }
    }

    refreshButton.addEventListener("click", refresh);
    clearButton.addEventListener("click", function () {
      localStorage.removeItem("twins_api_key");
      keyInput.value = "";
      walletCount.textContent = "";
      updatedAt.textContent = "";
      transactionCount.textContent = "";
      matchCount.textContent = "";
      receiptCount.textContent = "";
      webhookCount.textContent = "";
      reconciliationCount.textContent = "";
      walletSnapshotCount.textContent = "";
      settlementCount.textContent = "";
      exportCount.textContent = "";
      exceptionCount.textContent = "";
      securityPolicyCount.textContent = "";
      apiKeyCount.textContent = "";
      userCount.textContent = "";
      incidentCount.textContent = "";
      accessLogCount.textContent = "";
      usageMetricCount.textContent = "";
      designPartnerCount.textContent = "";
      betaEvidenceCount.textContent = "";
      privateBetaCount.textContent = "";
      content.innerHTML = '<div class="empty">Enter an API key to load payment requests.</div>';
      transactionContent.innerHTML = '<div class="empty">Enter an API key to load transaction evidence.</div>';
      matchContent.innerHTML = '<div class="empty">Enter an API key to load matches.</div>';
      receiptContent.innerHTML = '<div class="empty">Enter an API key to load receipt events.</div>';
      webhookContent.innerHTML = '<div class="empty">Enter an API key to load webhook deliveries.</div>';
      reconciliationContent.innerHTML = '<div class="empty">Enter an API key to load reconciliation runs.</div>';
      walletSnapshotContent.innerHTML = '<div class="empty">Run reconciliation to load wallet snapshots.</div>';
      settlementContent.innerHTML = '<div class="empty">Run reconciliation to load settlement rows.</div>';
      exportContent.innerHTML = '<div class="empty">Enter an API key to load exports.</div>';
      exceptionContent.innerHTML = '<div class="empty">Enter an API key to load exceptions.</div>';
      securityPolicyContent.innerHTML = '<div class="empty">Enter an API key to load security policy.</div>';
      apiKeyContent.innerHTML = '<div class="empty">Enter an API key to load API keys.</div>';
      userContent.innerHTML = '<div class="empty">Enter an API key to load users.</div>';
      incidentContent.innerHTML = '<div class="empty">Enter an API key to load incidents.</div>';
      accessLogContent.innerHTML = '<div class="empty">Enter an API key to load access logs.</div>';
      usageMetricContent.innerHTML = '<div class="empty">Enter an API key to load usage metrics.</div>';
      designPartnerContent.innerHTML = '<div class="empty">Enter an API key to load design partners.</div>';
      betaEvidenceContent.innerHTML = '<div class="empty">Enter an API key to load beta evidence.</div>';
      privateBetaContent.innerHTML = '<div class="empty">Enter an API key to load private beta readiness.</div>';
    });
    if (keyInput.value) refresh();
  </script>
</body>
</html>`))

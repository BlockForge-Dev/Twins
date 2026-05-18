package api

import (
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net/http"
	"strings"

	"twins/internal/core"
)

type Server struct {
	store *core.MemoryStore
	mux   *http.ServeMux
}

func NewServer(store *core.MemoryStore) http.Handler {
	server := &Server{
		store: store,
		mux:   http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/", s.handleRoot)
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/dashboard", s.handleDashboard)
	s.mux.HandleFunc("/v1/businesses", s.handleBusinesses)
	s.mux.HandleFunc("/v1/wallets", s.handleWallets)
	s.mux.HandleFunc("/v1/payment-requests", s.handlePaymentRequests)
	s.mux.HandleFunc("/v1/payment-requests/", s.handlePaymentRequestByID)
	s.mux.HandleFunc("/v1/stablecoin-transactions", s.handleStablecoinTransactions)
	s.mux.HandleFunc("/v1/transaction-matches", s.handleTransactionMatches)
	s.mux.HandleFunc("/v1/exceptions", s.handleExceptions)
	s.mux.HandleFunc("/v1/exceptions/", s.handleExceptionByID)
	s.mux.HandleFunc("/v1/audit-logs", s.handleAuditLogs)
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

func (s *Server) handleWallets(w http.ResponseWriter, r *http.Request) {
	business, apiKey, ok := s.authenticate(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		wallets, err := s.store.ListWallets(r.Context(), business.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"wallets": wallets})
	case http.MethodPost:
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
	business, apiKey, ok := s.authenticate(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		requests, err := s.store.ListPaymentRequests(r.Context(), business.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"payment_requests": requests})
	case http.MethodPost:
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
	business, _, ok := s.authenticate(w, r)
	if !ok {
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/v1/payment-requests/")
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
	business, apiKey, ok := s.authenticate(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		transactions, err := s.store.ListStablecoinTransactions(r.Context(), business.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"stablecoin_transactions": transactions})
	case http.MethodPost:
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
	business, _, ok := s.authenticate(w, r)
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
	business, _, ok := s.authenticate(w, r)
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
	business, apiKey, ok := s.authenticate(w, r)
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

func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	business, _, ok := s.authenticate(w, r)
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

func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (core.Business, core.APIKey, bool) {
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
	case core.CodeNotFound:
		status = http.StatusNotFound
	case core.CodeConflict:
		status = http.StatusConflict
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
      <div class="muted">Payment requests and chain evidence</div>
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
        <h2>Exceptions</h2>
        <span class="muted" id="exception-count"></span>
      </div>
      <div class="table-wrap" id="exception-content">
        <div class="empty">Enter an API key to load exceptions.</div>
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
    const exceptionContent = document.querySelector("#exception-content");
    const updatedAt = document.querySelector("#updated-at");
    const transactionCount = document.querySelector("#transaction-count");
    const matchCount = document.querySelector("#match-count");
    const exceptionCount = document.querySelector("#exception-count");
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

    async function refresh() {
      content.innerHTML = '<div class="empty">Loading...</div>';
      transactionContent.innerHTML = '<div class="empty">Loading...</div>';
      matchContent.innerHTML = '<div class="empty">Loading...</div>';
      exceptionContent.innerHTML = '<div class="empty">Loading...</div>';
      walletCount.textContent = "";
      transactionCount.textContent = "";
      matchCount.textContent = "";
      exceptionCount.textContent = "";
      try {
        const [requestData, walletData, transactionData, matchData, exceptionData] = await Promise.all([
          api("/v1/payment-requests"),
          api("/v1/wallets"),
          api("/v1/stablecoin-transactions"),
          api("/v1/transaction-matches"),
          api("/v1/exceptions")
        ]);
        renderRows(requestData.payment_requests || []);
        renderTransactions(transactionData.stablecoin_transactions || []);
        renderMatches(matchData.transaction_matches || []);
        renderExceptions(exceptionData.exceptions || []);
        const wallets = walletData.wallets || [];
        walletCount.textContent = wallets.length + " wallet" + (wallets.length === 1 ? "" : "s");
        updatedAt.textContent = "Updated " + new Date().toLocaleTimeString();
      } catch (err) {
        content.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        transactionContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        matchContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
        exceptionContent.innerHTML = '<div class="error">' + esc(err.message) + '</div>';
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
      exceptionCount.textContent = "";
      content.innerHTML = '<div class="empty">Enter an API key to load payment requests.</div>';
      transactionContent.innerHTML = '<div class="empty">Enter an API key to load transaction evidence.</div>';
      matchContent.innerHTML = '<div class="empty">Enter an API key to load matches.</div>';
      exceptionContent.innerHTML = '<div class="empty">Enter an API key to load exceptions.</div>';
    });
    if (keyInput.value) refresh();
  </script>
</body>
</html>`))

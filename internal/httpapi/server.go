package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/DEV-GHILDIYAL/Shipra/internal/identity"
	"github.com/DEV-GHILDIYAL/Shipra/internal/storage"
)

type Server struct {
	DB      *sql.DB
	BaseURL string
	Logger  *slog.Logger
	mu      sync.Mutex
	login   map[string]loginLimit
}

type loginLimit struct {
	count int
	since time.Time
}

func New(db *sql.DB, baseURL string, logger *slog.Logger) *Server {
	return &Server{DB: db, BaseURL: baseURL, Logger: logger, login: make(map[string]loginLimit)}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if s.DB.PingContext(ctx) != nil || storage.RequireVersion(ctx, s.DB) != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/v1/session", s.loginSession)
	mux.HandleFunc("DELETE /api/v1/session", s.logoutSession)
	mux.HandleFunc("GET /api/v1/me", s.me)
	mux.HandleFunc("POST /api/v1/tokens", s.createToken)
	mux.HandleFunc("DELETE /api/v1/tokens/{id}", s.revokeToken)
	mux.HandleFunc("GET /api/v1/projects", s.listProjects)
	mux.HandleFunc("POST /api/v1/projects", s.createProject)
	mux.HandleFunc("GET /api/v1/projects/{id}/repositories", s.listRepositories)
	mux.HandleFunc("POST /api/v1/projects/{id}/repositories", s.createRepository)
	mux.HandleFunc("POST /api/v1/users", s.createUser)
	mux.HandleFunc("PATCH /api/v1/users/{id}/role", s.changeRole)
	mux.HandleFunc("GET /api/v1/audit-events", s.listAudit)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		id, _ := identity.RandomID()
		w.Header().Set("X-Request-ID", id)
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id))
		mux.ServeHTTP(w, r)
	})
}

type requestIDKey struct{}

func requestID(r *http.Request) string {
	id, _ := r.Context().Value(requestIDKey{}).(string)
	return id
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	respond(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "request_id": requestID(r)}})
}
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		fail(w, r, 415, "content_type", "application/json required")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		fail(w, r, 400, "invalid_json", "Invalid JSON request")
		return false
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		fail(w, r, 400, "invalid_json", "Only one JSON value is allowed")
		return false
	}
	return true
}

func (s *Server) originOK(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	a, errA := url.Parse(origin)
	b, errB := url.Parse(s.BaseURL)
	return errA == nil && errB == nil && a.Scheme == b.Scheme && strings.EqualFold(a.Host, b.Host) && a.Path == "" && a.RawQuery == ""
}

func (s *Server) principal(w http.ResponseWriter, r *http.Request, action string) (identity.Principal, bool) {
	var p identity.Principal
	var err error
	bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if bearer != r.Header.Get("Authorization") && bearer != "" {
		p, err = identity.Token(r.Context(), s.DB, bearer)
	} else {
		cookie, cookieErr := r.Cookie("shipra_session")
		if cookieErr != nil {
			fail(w, r, 401, "unauthenticated", "Authentication required")
			return p, false
		}
		p, err = identity.Session(r.Context(), s.DB, cookie.Value)
		if err == nil && r.Method != http.MethodGet && r.Method != http.MethodHead {
			if !s.originOK(r) || !identity.CSRFMatches(p, r.Header.Get("X-CSRF-Token")) {
				fail(w, r, 403, "csrf", "Invalid request origin or CSRF token")
				return p, false
			}
		}
	}
	if err != nil {
		fail(w, r, 401, "unauthenticated", "Authentication required")
		return p, false
	}
	if !identity.Allows(p.Role, p.Scope, action) {
		fail(w, r, 403, "forbidden", "Insufficient permission")
		return p, false
	}
	return p, true
}

func (s *Server) allowLogin(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if len(s.login) > 10000 {
		s.login = make(map[string]loginLimit)
	}
	entry := s.login[host]
	if now.Sub(entry.since) >= time.Minute {
		entry = loginLimit{since: now}
	}
	entry.count++
	s.login[host] = entry
	return entry.count <= 5
}

func (s *Server) loginSession(w http.ResponseWriter, r *http.Request) {
	if !s.originOK(r) {
		fail(w, r, 403, "origin", "Invalid origin")
		return
	}
	if !s.allowLogin(r.RemoteAddr) {
		fail(w, r, 429, "rate_limited", "Try again later")
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	p, err := identity.AuthenticatePassword(r.Context(), s.DB, input.Username, input.Password)
	if err != nil {
		fail(w, r, 401, "invalid_credentials", "Invalid credentials")
		return
	}
	secret, csrf, err := identity.CreateSession(r.Context(), s.DB, p, requestID(r))
	if err != nil {
		s.internal(w, r, err)
		return
	}
	base, _ := url.Parse(s.BaseURL)
	http.SetCookie(w, &http.Cookie{Name: "shipra_session", Value: secret, Path: "/api/v1", HttpOnly: true, Secure: base.Scheme == "https", SameSite: http.SameSiteStrictMode, MaxAge: int(identity.SessionExpiresAt().Seconds())})
	respond(w, 200, map[string]any{"user": p, "csrf_token": csrf})
}

func (s *Server) logoutSession(w http.ResponseWriter, r *http.Request) {
	p, ok := s.principal(w, r, "read")
	if !ok {
		return
	}
	if p.SessionID == "" {
		fail(w, r, 400, "session_required", "A browser session is required")
		return
	}
	if err := identity.RevokeSession(r.Context(), s.DB, p, requestID(r)); err != nil {
		s.internal(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "shipra_session", Path: "/api/v1", MaxAge: -1, HttpOnly: true})
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	p, ok := s.principal(w, r, "read")
	if ok {
		respond(w, 200, p)
	}
}

func (s *Server) createToken(w http.ResponseWriter, r *http.Request) {
	p, ok := s.principal(w, r, "read")
	if !ok {
		return
	}
	var input struct {
		Scope string `json:"scope"`
	}
	if !decode(w, r, &input) {
		return
	}
	id, secret, err := identity.CreateToken(r.Context(), s.DB, p, input.Scope, requestID(r))
	if errors.Is(err, identity.ErrDenied) {
		fail(w, r, 403, "forbidden", "Scope exceeds current permission")
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	respond(w, 201, map[string]string{"id": id, "token": secret})
}
func (s *Server) revokeToken(w http.ResponseWriter, r *http.Request) {
	p, ok := s.principal(w, r, "read")
	if !ok {
		return
	}
	err := identity.RevokeToken(r.Context(), s.DB, p, r.PathValue("id"), requestID(r))
	if errors.Is(err, identity.ErrDenied) {
		fail(w, r, 404, "not_found", "Token not found")
		return
	}
	if err != nil {
		s.internal(w, r, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) internal(w http.ResponseWriter, r *http.Request, err error) {
	s.Logger.Error("request failed", "request_id", requestID(r), "error", err)
	fail(w, r, 500, "internal", "Request failed")
}

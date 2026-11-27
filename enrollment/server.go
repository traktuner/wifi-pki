package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type server struct {
	cfg       config
	store     *store
	provider  *oidc.Provider
	verifier  *oidc.IDTokenVerifier
	oauth     oauth2.Config
	limits    *ipLimiter
	downloads *downloadStore
}

type claims struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Nonce         string `json:"nonce"`
	AZP           string `json:"azp"`
	RealmAccess   struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

type ipLimiter struct {
	mu     sync.Mutex
	events map[string][]time.Time
}

type pendingDownload struct {
	Profile  []byte
	Filename string
	Subject  string
	Name     string
	Expires  time.Time
}

type downloadStore struct {
	mu    sync.Mutex
	items map[string]pendingDownload
}

func newDownloadStore() *downloadStore {
	return &downloadStore{items: make(map[string]pendingDownload)}
}

func (d *downloadStore) put(id string, item pendingDownload) {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now().UTC()
	for key, existing := range d.items {
		if !existing.Expires.After(now) {
			delete(d.items, key)
		}
	}
	d.items[id] = item
}

func (d *downloadStore) get(id, subject string, consume bool) (pendingDownload, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	item, ok := d.items[id]
	if !ok || item.Subject != subject || !item.Expires.After(time.Now().UTC()) {
		if ok && !item.Expires.After(time.Now().UTC()) {
			delete(d.items, id)
		}
		return pendingDownload{}, false
	}
	if consume {
		delete(d.items, id)
	}
	return item, true
}

func (l *ipLimiter) allow(key string, max int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-window)
	values := l.events[key][:0]
	for _, t := range l.events[key] {
		if t.After(cutoff) {
			values = append(values, t)
		}
	}
	if len(values) >= max {
		l.events[key] = values
		return false
	}
	l.events[key] = append(values, now)
	return true
}

func newServer(ctx context.Context, c config, db *store) (*server, error) {
	provider, err := oidc.NewProvider(ctx, c.OIDCIssuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	oauth := oauth2.Config{
		ClientID: c.OIDCClientID, ClientSecret: string(c.OIDCClientSecret),
		Endpoint: provider.Endpoint(), RedirectURL: c.OIDCRedirectURL,
		Scopes: []string{oidc.ScopeOpenID, "profile", "email"},
	}
	return &server{
		cfg: c, store: db, provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: c.OIDCClientID}),
		oauth:    oauth, limits: &ipLimiter{events: make(map[string][]time.Time)},
		downloads: newDownloadStore(),
	}, nil
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/assets/app.css", stylesheet)
	mux.HandleFunc("/assets/favicon.svg", brandingIcon)
	mux.HandleFunc("/", s.home)
	mux.HandleFunc("/login", s.login)
	mux.HandleFunc("/oauth/callback", s.callback)
	mux.HandleFunc("/enroll", s.enroll)
	mux.HandleFunc("/profile-ready", s.profileReady)
	mux.HandleFunc("/download", s.download)
	mux.HandleFunc("/device/cancel", s.cancelDevice)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if err := s.store.db.Ping(); err != nil {
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return securityHeaders(mux)
}

func (s *server) webhookRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/stepca/scep-challenge", s.webhook)
	return securityHeaders(mux)
}

func (s *server) session(r *http.Request) (session, error) {
	cookie, err := r.Cookie("__Host-wifi-session")
	if err != nil {
		return session{}, err
	}
	id, ok := verifyOpaque(s.cfg.SessionSecret, cookie.Value)
	if !ok {
		return session{}, errors.New("invalid session cookie")
	}
	return s.store.getSession(r.Context(), id, time.Now().UTC())
}

func (s *server) setCookie(w http.ResponseWriter, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/",
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: maxAge})
}

func (s *server) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	sess, err := s.session(r)
	data := homePageData{SSID: s.cfg.SSID, OnboardingSSID: s.cfg.OnboardingSSID}
	if err == nil {
		data.Authenticated, data.Email, data.CSRF = true, sess.Email, sess.CSRF
		data.Devices, err = s.store.listDevices(r.Context(), sess.Sub)
		if err != nil {
			slog.Error("list devices", "error", err)
			data.Devices = nil
			data.Notice = "Die Geräteliste ist vorübergehend nicht verfügbar."
		}
		if r.URL.Query().Get("cancelled") == "1" {
			data.Notice = "Die offene Geräteberechtigung wurde entfernt."
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pageTemplate.Execute(w, data); err != nil {
		slog.Error("render home", "error", err)
	}
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	state, _ := randomToken(32)
	nonce, _ := randomToken(32)
	verifier, _ := randomToken(32)
	id, _ := randomToken(32)
	if state == "" || nonce == "" || verifier == "" || id == "" {
		http.Error(w, "temporary failure", http.StatusServiceUnavailable)
		return
	}
	if err := s.store.putOAuth(r.Context(), id, tokenHash(s.cfg.SessionSecret, state),
		nonce, verifier, time.Now().UTC().Add(10*time.Minute)); err != nil {
		http.Error(w, "temporary failure", http.StatusServiceUnavailable)
		return
	}
	challenge := sha256.Sum256([]byte(verifier))
	s.setCookie(w, "__Host-wifi-oidc", signOpaque(s.cfg.SessionSecret, id), 600)
	url := s.oauth.AuthCodeURL(state,
		oidc.Nonce(nonce),
		oauth2.SetAuthURLParam("code_challenge", base64.RawURLEncoding.EncodeToString(challenge[:])),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"))
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *server) callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Query().Get("error") != "" {
		http.Error(w, "OIDC login failed", http.StatusUnauthorized)
		return
	}
	cookie, err := r.Cookie("__Host-wifi-oidc")
	if err != nil {
		http.Error(w, "invalid login state", http.StatusBadRequest)
		return
	}
	id, ok := verifyOpaque(s.cfg.SessionSecret, cookie.Value)
	if !ok {
		http.Error(w, "invalid login state", http.StatusBadRequest)
		return
	}
	stateHash, nonce, verifier, err := s.store.consumeOAuth(r.Context(), id, time.Now().UTC())
	if err != nil || !secureEqual(stateHash, tokenHash(s.cfg.SessionSecret, r.URL.Query().Get("state"))) {
		http.Error(w, "invalid login state", http.StatusBadRequest)
		return
	}
	token, err := s.oauth.Exchange(r.Context(), r.URL.Query().Get("code"),
		oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		slog.Warn("oidc exchange rejected")
		http.Error(w, "OIDC login failed", http.StatusUnauthorized)
		return
	}
	rawID, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "OIDC login failed", http.StatusUnauthorized)
		return
	}
	idToken, err := s.verifier.Verify(r.Context(), rawID)
	if err != nil {
		slog.Warn("oidc token rejected")
		http.Error(w, "OIDC login failed", http.StatusUnauthorized)
		return
	}
	var cl claims
	if err := idToken.Claims(&cl); err != nil ||
		!authorizedClaims(cl, nonce, s.cfg.OIDCClientID, s.cfg.OIDCRequiredRole) {
		slog.Warn("oidc authorization rejected")
		http.Error(w, "not authorized for WiFi enrollment", http.StatusForbidden)
		return
	}
	if !cl.EmailVerified {
		cl.Email = ""
	}
	sid, _ := randomToken(32)
	csrf, _ := randomToken(32)
	sess := session{ID: sid, Sub: cl.Subject, Email: cl.Email, CSRF: csrf, Expires: time.Now().UTC().Add(s.cfg.SessionTTL)}
	if sid == "" || csrf == "" || s.store.createSession(r.Context(), sess) != nil {
		http.Error(w, "temporary failure", http.StatusServiceUnavailable)
		return
	}
	s.setCookie(w, "__Host-wifi-oidc", "", -1)
	s.setCookie(w, "__Host-wifi-session", signOpaque(s.cfg.SessionSecret, sid), int(s.cfg.SessionTTL.Seconds()))
	slog.Info("oidc login accepted", "subject", auditSubject(cl.Subject))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) enroll(w http.ResponseWriter, r *http.Request) {
	if err := requirePost(r); err != nil {
		http.Error(w, err.Error(), http.StatusMethodNotAllowed)
		return
	}
	sess, err := s.session(r)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil || !secureEqual([]byte(sess.CSRF), []byte(r.Form.Get("csrf"))) {
		http.Error(w, "invalid request", http.StatusForbidden)
		return
	}
	name := strings.TrimSpace(r.Form.Get("device_name"))
	if !validDeviceName(name) {
		http.Error(w, "invalid device name", http.StatusBadRequest)
		return
	}
	if !s.limits.allow("enroll:"+auditSubject(sess.Sub), 10, time.Hour) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	if err := s.stepCAHealthy(r.Context()); err != nil {
		slog.Error("step-ca unavailable during enrollment")
		http.Error(w, "certificate authority unavailable", http.StatusServiceUnavailable)
		return
	}
	challenge, _ := randomToken(32)
	deviceUUID, _ := uuidV4()
	enrollmentID, _ := uuidV4()
	if challenge == "" || deviceUUID == "" || enrollmentID == "" {
		http.Error(w, "temporary failure", http.StatusServiceUnavailable)
		return
	}
	shortSub := fmt.Sprintf("%x", sha256.Sum256([]byte(sess.Sub)))[:12]
	identity := "wifi-" + shortSub + "-" + deviceUUID
	now := time.Now().UTC()
	e := enrollment{ID: enrollmentID, Sub: sess.Sub, Email: sess.Email, DeviceName: name,
		DeviceUUID: deviceUUID, IdentityCN: identity,
		ChallengeHash:    tokenHash(s.cfg.SessionSecret, challenge),
		ChallengeExpires: now.Add(s.cfg.ChallengeTTL), Expires: now.Add(1825 * 24 * time.Hour)}
	if err := s.store.createEnrollment(r.Context(), e, s.cfg.MaxEnrollmentsPerDay); err != nil {
		slog.Warn("enrollment rejected", "subject", auditSubject(sess.Sub))
		http.Error(w, "enrollment unavailable or limit reached", http.StatusTooManyRequests)
		return
	}
	unsigned, err := generateProfile(s.cfg, profileInput{
		DisplayName: "OnCloud WiFi – " + name, IdentityCN: identity,
		DeviceUUID: deviceUUID, Email: sess.Email, Challenge: challenge})
	if err != nil {
		http.Error(w, "profile generation failed", http.StatusInternalServerError)
		return
	}
	profile, err := signProfile(s.cfg, unsigned)
	if err != nil {
		slog.Error("profile signing failed", "error", err)
		http.Error(w, "profile signing failed", http.StatusServiceUnavailable)
		return
	}
	downloadID, err := randomToken(32)
	if err != nil {
		http.Error(w, "temporary failure", http.StatusServiceUnavailable)
		return
	}
	s.downloads.put(downloadID, pendingDownload{
		Profile: profile, Filename: safeFilename(name), Subject: sess.Sub, Name: name,
		Expires: time.Now().UTC().Add(5 * time.Minute),
	})
	s.setCookie(w, "__Host-wifi-download", signOpaque(s.cfg.SessionSecret, downloadID), 300)
	slog.Info("enrollment profile prepared", "subject", auditSubject(sess.Sub), "device", deviceUUID)
	http.Redirect(w, r, "/profile-ready", http.StatusSeeOther)
}

func (s *server) profileReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess, err := s.session(r)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	cookie, err := r.Cookie("__Host-wifi-download")
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	id, ok := verifyOpaque(s.cfg.SessionSecret, cookie.Value)
	item, available := s.downloads.get(id, sess.Sub, false)
	if !ok || !available {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := readyTemplate.Execute(w, readyPageData{
		Name: item.Name, SSID: s.cfg.SSID, OnboardingSSID: s.cfg.OnboardingSSID,
	}); err != nil {
		slog.Error("render profile ready", "error", err)
	}
}

func (s *server) download(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess, err := s.session(r)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	cookie, err := r.Cookie("__Host-wifi-download")
	if err != nil {
		http.Error(w, "profile download expired", http.StatusGone)
		return
	}
	id, ok := verifyOpaque(s.cfg.SessionSecret, cookie.Value)
	if !ok {
		http.Error(w, "profile download expired", http.StatusGone)
		return
	}
	item, ok := s.downloads.get(id, sess.Sub, true)
	if !ok {
		http.Error(w, "profile download expired", http.StatusGone)
		return
	}
	s.setCookie(w, "__Host-wifi-download", "", -1)
	w.Header().Set("Content-Type", "application/x-apple-aspen-config")
	w.Header().Set("Content-Disposition", `attachment; filename="`+item.Filename+`"`)
	w.Header().Set("Cache-Control", "no-store, private, max-age=0")
	w.Header().Set("Content-Length", fmt.Sprint(len(item.Profile)))
	_, _ = w.Write(item.Profile)
	slog.Info("enrollment profile downloaded", "subject", auditSubject(sess.Sub))
}

func (s *server) cancelDevice(w http.ResponseWriter, r *http.Request) {
	if err := requirePost(r); err != nil {
		http.Error(w, err.Error(), http.StatusMethodNotAllowed)
		return
	}
	sess, err := s.session(r)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil ||
		!secureEqual([]byte(sess.CSRF), []byte(r.Form.Get("csrf"))) {
		http.Error(w, "invalid request", http.StatusForbidden)
		return
	}
	deviceUUID := strings.TrimSpace(r.Form.Get("device_uuid"))
	if err := s.store.cancelEnrollment(r.Context(), sess.Sub, deviceUUID); err != nil {
		http.Error(w, "device authorization cannot be removed", http.StatusConflict)
		return
	}
	slog.Info("pending device authorization removed",
		"subject", auditSubject(sess.Sub), "device", deviceUUID)
	http.Redirect(w, r, "/?cancelled=1", http.StatusSeeOther)
}

type webhookRequest struct {
	SCEPChallenge string    `json:"scepChallenge"`
	TransactionID string    `json:"scepTransactionID"`
	Timestamp     time.Time `json:"timestamp"`
}

func (s *server) webhook(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost || !s.limits.allow("webhook:"+remoteIP(r), 120, time.Minute) {
		json.NewEncoder(w).Encode(map[string]bool{"allow": false})
		return
	}
	raw := http.MaxBytesReader(w, r.Body, 16<<10)
	body, err := readAll(raw)
	if err != nil ||
		!secureEqual([]byte(r.Header.Get("X-Smallstep-Webhook-ID")), []byte(s.cfg.SCEPWebhookID)) ||
		!verifyWebhookSignature(s.cfg.WebhookSecret, body, r.Header.Get("X-Smallstep-Signature")) {
		slog.Warn("scep webhook authentication rejected")
		json.NewEncoder(w).Encode(map[string]bool{"allow": false})
		return
	}
	var request webhookRequest
	if json.Unmarshal(body, &request) != nil || request.SCEPChallenge == "" ||
		request.TransactionID == "" || len(request.TransactionID) > 256 ||
		request.Timestamp.IsZero() || time.Since(request.Timestamp).Abs() > 2*time.Minute {
		json.NewEncoder(w).Encode(map[string]bool{"allow": false})
		return
	}
	e, allowed, err := s.store.consumeChallenge(r.Context(),
		tokenHash(s.cfg.SessionSecret, request.SCEPChallenge), request.TransactionID,
		time.Now().UTC(), s.cfg.RetryWindow)
	if err != nil {
		slog.Error("scep webhook database failure")
		allowed = false
	}
	response := map[string]any{"allow": allowed}
	if allowed {
		data := map[string]string{
			"commonName": e.IdentityCN,
			"uriSAN":     "urn:oncloud:wifi-device:" + e.DeviceUUID,
		}
		if e.Email != "" {
			data["emailSAN"] = e.Email
		}
		response["data"] = data
		slog.Info("scep challenge accepted", "subject", auditSubject(e.Sub), "device", e.DeviceUUID)
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (s *server) stepCAHealthy(ctx context.Context) error {
	rootPEM, err := os.ReadFile(s.cfg.RootCAPath)
	if err != nil {
		return err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(rootPEM) {
		return errors.New("invalid step-ca root")
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: s.cfg.StepCAServerName,
		}},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.StepCAInternalURL+"/health", nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("step-ca health status %d", response.StatusCode)
	}
	return nil
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
func authorizedClaims(cl claims, nonce, clientID, requiredRole string) bool {
	return cl.Subject != "" && cl.Nonce == nonce &&
		(cl.AZP == "" || cl.AZP == clientID) &&
		contains(cl.RealmAccess.Roles, requiredRole)
}
func auditSubject(sub string) string {
	sum := sha256.Sum256([]byte(sub))
	return fmt.Sprintf("%x", sum[:8])
}
func remoteIP(r *http.Request) string {
	// The endpoint is internal-only. Do not trust forwarding headers here.
	value := r.RemoteAddr
	if pos := strings.LastIndexByte(value, ':'); pos > 0 {
		value = value[:pos]
	}
	return value
}

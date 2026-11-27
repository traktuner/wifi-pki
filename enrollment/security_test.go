package main

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

func TestChallengeGenerationAndHashing(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	a, err := randomToken(32)
	if err != nil || len(a) < 43 {
		t.Fatalf("randomToken: %q %v", a, err)
	}
	b, _ := randomToken(32)
	if a == b {
		t.Fatal("two challenges matched")
	}
	if secureEqual(tokenHash(secret, a), tokenHash(secret, b)) {
		t.Fatal("different challenges produced equal hashes")
	}
}

func TestWebhookSignature(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	encoded := []byte(base64.StdEncoding.EncodeToString(secret))
	decoded, err := decodeWebhookSigningSecret(encoded)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"scepChallenge":"redacted","scepTransactionID":"tx-1"}`)
	mac := hmac.New(sha256.New, decoded)
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))
	if !verifyWebhookSignature(decoded, body, sig) {
		t.Fatal("valid signature rejected")
	}
	if verifyWebhookSignature(decoded, append(body, 'x'), sig) {
		t.Fatal("tampered body accepted")
	}
	if _, err := decodeWebhookSigningSecret(secret); err == nil {
		t.Fatal("raw non-Base64 webhook secret accepted")
	}
}

func TestDeviceNameValidation(t *testing.T) {
	for _, good := range []string{"Thomas iPhone 17 Pro", "Cleo iPad", "MacBook-Pro (2026)"} {
		if !validDeviceName(good) {
			t.Errorf("valid name rejected: %q", good)
		}
	}
	for _, bad := range []string{"", "x", "<script>", " leading", "two  spaces", string(make([]byte, 65))} {
		if validDeviceName(bad) {
			t.Errorf("invalid name accepted: %q", bad)
		}
	}
}

func TestAuthorizationClaims(t *testing.T) {
	var c claims
	c.Subject, c.Nonce, c.AZP = "stable-sub", "nonce", "wifi-enrollment"
	c.RealmAccess.Roles = []string{"wifi-enroll"}
	if !authorizedClaims(c, "nonce", "wifi-enrollment", "wifi-enroll") {
		t.Fatal("authorized user rejected")
	}
	c.RealmAccess.Roles = nil
	if authorizedClaims(c, "nonce", "wifi-enrollment", "wifi-enroll") {
		t.Fatal("unauthorized role accepted")
	}
	c.RealmAccess.Roles = []string{"wifi-enroll"}
	c.AZP = "other-client"
	if authorizedClaims(c, "nonce", "wifi-enrollment", "wifi-enroll") {
		t.Fatal("invalid authorized party accepted")
	}
	c.AZP = "wifi-enrollment"
	if authorizedClaims(c, "wrong-nonce", "wifi-enrollment", "wifi-enroll") {
		t.Fatal("invalid nonce accepted")
	}
}

func TestWebhookInvalidIDFailsClosed(t *testing.T) {
	s := &server{
		cfg: config{SCEPWebhookID: "expected", WebhookSecret: []byte("01234567890123456789012345678901"),
			SessionSecret: []byte("abcdefghijklmnopqrstuvwxyz012345")},
		store: testStore(t), limits: &ipLimiter{events: make(map[string][]time.Time)},
	}
	body, _ := json.Marshal(webhookRequest{SCEPChallenge: "unknown", TransactionID: "tx",
		Timestamp: time.Now().UTC()})
	mac := hmac.New(sha256.New, s.cfg.WebhookSecret)
	mac.Write(body)
	req := httptest.NewRequest(http.MethodPost, "/internal/stepca/scep-challenge", strings.NewReader(string(body)))
	req.Header.Set("X-Smallstep-Webhook-ID", "wrong")
	req.Header.Set("X-Smallstep-Signature", hex.EncodeToString(mac.Sum(nil)))
	rec := httptest.NewRecorder()
	s.webhook(rec, req)
	if !strings.Contains(rec.Body.String(), `"allow":false`) {
		t.Fatalf("invalid webhook ID did not fail closed: %s", rec.Body.String())
	}
}

func TestOIDCVerifierRejectsIssuerAudienceAndExpiry(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes())
		n := base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes())
		json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "test", "use": "sig", "alg": "RS256", "n": n, "e": e,
		}}})
	}))
	defer jwks.Close()
	issuer := "https://id.example.org/realms/oncloud"
	verifier := oidc.NewVerifier(issuer, oidc.NewRemoteKeySet(context.Background(), jwks.URL),
		&oidc.Config{ClientID: "wifi-enrollment"})
	valid := map[string]any{"iss": issuer, "sub": "sub", "aud": "wifi-enrollment",
		"iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Minute).Unix()}
	if _, err := verifier.Verify(context.Background(), signedJWT(t, key, valid)); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	for name, mutate := range map[string]func(map[string]any){
		"issuer":   func(c map[string]any) { c["iss"] = "https://evil.invalid" },
		"audience": func(c map[string]any) { c["aud"] = "other-client" },
		"expiry":   func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() },
	} {
		t.Run(name, func(t *testing.T) {
			claims := make(map[string]any, len(valid))
			for k, v := range valid {
				claims[k] = v
			}
			mutate(claims)
			if _, err := verifier.Verify(context.Background(), signedJWT(t, key, claims)); err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
}

func signedJWT(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "test", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload)
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)
}

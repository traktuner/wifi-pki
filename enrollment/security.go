package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

var deviceNameRE = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N} ._'()&+\-]{1,63}$`)

func randomToken(n int) (string, error) {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func tokenHash(secret []byte, value string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(value))
	return mac.Sum(nil)
}

func secureEqual(a, b []byte) bool {
	if len(a) != len(b) {
		// Keep the mismatch path from becoming a useful prefix oracle.
		dummy := make([]byte, len(a))
		subtle.ConstantTimeCompare(a, dummy)
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}

func verifyWebhookSignature(secret, raw []byte, supplied string) bool {
	mac := hmac.New(sha256.New, secret)
	mac.Write(raw)
	expected := mac.Sum(nil)
	supplied = strings.TrimSpace(supplied)
	if decoded, err := hex.DecodeString(supplied); err == nil {
		return secureEqual(expected, decoded)
	}
	if decoded, err := base64.StdEncoding.DecodeString(supplied); err == nil {
		return secureEqual(expected, decoded)
	}
	return false
}

func signOpaque(secret []byte, value string) string {
	sig := tokenHash(secret, value)
	return value + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func verifyOpaque(secret []byte, signed string) (string, bool) {
	pos := strings.LastIndexByte(signed, '.')
	if pos <= 0 {
		return "", false
	}
	value := signed[:pos]
	got, err := base64.RawURLEncoding.DecodeString(signed[pos+1:])
	return value, err == nil && secureEqual(tokenHash(secret, value), got)
}

func validDeviceName(value string) bool {
	return len([]rune(value)) >= 2 && len([]rune(value)) <= 64 &&
		deviceNameRE.MatchString(value) && !strings.Contains(value, "  ")
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; img-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cache-Control", "no-store, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		next.ServeHTTP(w, r)
	})
}

func requirePost(r *http.Request) error {
	if r.Method != http.MethodPost {
		return errors.New("method not allowed")
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		return errors.New("invalid content type")
	}
	return nil
}

func safeFilename(name string) string {
	name = strings.ToLower(name)
	name = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "device"
	}
	return fmt.Sprintf("oncloud-wifi-%s.mobileconfig", name)
}

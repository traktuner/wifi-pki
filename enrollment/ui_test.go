package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPortalTemplatesUseCSPCompatibleStylesAndEscapeContent(t *testing.T) {
	var rendered bytes.Buffer
	data := homePageData{
		Authenticated:  true,
		Email:          `<img src=x onerror=alert(1)>`,
		SSID:           `ExampleNet"><script>alert(1)</script>`,
		OnboardingSSID: `ExampleNet-Setup"><script>alert(2)</script>`,
		CSRF:           `csrf-token`,
		Devices: []deviceRow{{
			Name: "Thomas <iPhone>", UUID: "00000000-0000-4000-8000-000000000001",
			StatusLabel: "Aktiv", StatusClass: "status-good",
			Expires: "27.07.2031", ExpiresISO: "2031-07-27T08:44:06Z",
		}},
	}
	if err := pageTemplate.Execute(&rendered, data); err != nil {
		t.Fatal(err)
	}
	body := rendered.String()
	for _, forbidden := range []string{
		"<style", "style=", "<script>alert(1)</script>",
		"<script>alert(2)</script>", "<img src=x",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("unsafe or CSP-incompatible portal HTML contains %q", forbidden)
		}
	}
	for _, required := range []string{
		`href="/assets/app.css"`, `href="/assets/favicon.svg"`, "&lt;img src=x onerror=alert(1)&gt;",
		"Thomas &lt;iPhone&gt;", `class="status status-good"`,
		`value="csrf-token"`, `placeholder="z. B. iPhone"`,
		"ExampleNet-Setup&#34;&gt;&lt;script&gt;alert(2)&lt;/script&gt;", `class="oncloud-mark"`,
		`class="oncloud-container"`, `class="oncloud-module"`,
	} {
		if !strings.Contains(body, required) {
			t.Errorf("portal HTML missing %q", required)
		}
	}
	if strings.Contains(body, "Hardware‑MAC nur für") {
		t.Fatal("SSID-specific hardware-MAC hint unexpectedly present")
	}
}

func TestReadyTemplateContainsInstallAndPrivacyGuidance(t *testing.T) {
	var rendered bytes.Buffer
	if err := readyTemplate.Execute(&rendered, readyPageData{
		Name: "Firestarter", SSID: "ExampleNet", OnboardingSSID: "ExampleNet-Setup",
	}); err != nil {
		t.Fatal(err)
	}
	body := rendered.String()
	for _, required := range []string{
		`href="/download"`, "Profil geladen", "Hardware‑MAC", "ExampleNet",
		"ExampleNet-Setup", "Bleib während Download und Installation",
		"Einmaliger Download", `href="/assets/app.css"`, `href="/assets/favicon.svg"`,
		`class="oncloud-mark"`,
	} {
		if !strings.Contains(body, required) {
			t.Errorf("ready HTML missing %q", required)
		}
	}
}

func TestStylesheetAndSecurityHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/app.css", nil)
	rec := httptest.NewRecorder()
	securityHeaders(http.HandlerFunc(stylesheet)).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("stylesheet status=%d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/css; charset=utf-8" {
		t.Fatalf("stylesheet content type=%q", got)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "style-src 'self'") || strings.Contains(csp, "'unsafe-inline'") {
		t.Fatalf("unsafe stylesheet CSP: %q", csp)
	}
	if !strings.Contains(rec.Body.String(), "@media (prefers-reduced-motion: reduce)") {
		t.Fatal("stylesheet lacks reduced-motion support")
	}
	for _, required := range []string{
		".brand-mark .oncloud-container { fill: #f7f8fa; stroke: none; }",
		".brand-mark .oncloud-module { fill: #817aa8; stroke: none; }",
	} {
		if !strings.Contains(rec.Body.String(), required) {
			t.Errorf("stylesheet missing OnCloud brand rule %q", required)
		}
	}
}

func TestBrandingIcon(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/favicon.svg", nil)
	rec := httptest.NewRecorder()
	securityHeaders(http.HandlerFunc(brandingIcon)).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("branding icon status=%d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Fatalf("branding icon content type=%q", got)
	}
	for _, required := range []string{"#17191d", "#f7f8fa", "#817aa8"} {
		if !strings.Contains(rec.Body.String(), required) {
			t.Errorf("branding icon missing color %q", required)
		}
	}
}

func TestDeviceStatusPresentation(t *testing.T) {
	for status, expected := range map[string]string{
		"issued": "Aktiv", "pending": "Profil bereit",
		"requested": "Wird ausgestellt", "revoked": "Gesperrt",
		"unexpected": "Unbekannt",
	} {
		label, class := deviceStatusPresentation(status)
		if label != expected || !strings.HasPrefix(class, "status-") {
			t.Errorf("%s => %q %q", status, label, class)
		}
	}
}

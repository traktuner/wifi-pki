package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"encoding/xml"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTestCert(t *testing.T, path, cn string) {
	t.Helper()
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	template := &x509.Certificate{SerialNumber: bigOne(), Subject: pkix.Name{CommonName: cn},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestProfileCMSSigningRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "signing.crt")
	key := filepath.Join(dir, "signing.key")
	passwordFile := filepath.Join(dir, "password")
	password := "test-only-signing-password"
	if err := os.WriteFile(passwordFile, []byte(password+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("openssl", "req", "-x509", "-newkey", "rsa:2048",
		"-keyout", key, "-out", cert, "-days", "1", "-sha256",
		"-subj", "/CN=OnCloud WiFi Test Profile Signing",
		"-passout", "pass:"+password)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create signing identity: %v: %s", err, output)
	}
	unsigned := []byte(`<?xml version="1.0"?><plist version="1.0"><dict/></plist>`)
	signed, err := signProfile(config{
		SigningCert: cert, SigningKey: key, SigningPasswordFile: passwordFile,
	}, unsigned)
	if err != nil {
		t.Fatal(err)
	}
	signedPath := filepath.Join(dir, "profile.mobileconfig")
	verifiedPath := filepath.Join(dir, "verified.plist")
	if err := os.WriteFile(signedPath, signed, 0600); err != nil {
		t.Fatal(err)
	}
	verify := exec.Command("openssl", "cms", "-verify", "-binary", "-inform", "DER",
		"-in", signedPath, "-noverify", "-out", verifiedPath)
	if output, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("verify signed profile: %v: %s", err, output)
	}
	verified, err := os.ReadFile(verifiedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(verified, unsigned) {
		t.Fatal("CMS verified content differs from the unsigned profile")
	}
}

func bigOne() *big.Int { return big.NewInt(1) }

func TestProfileIsSafeValidXML(t *testing.T) {
	dir := t.TempDir()
	root, intermediate := filepath.Join(dir, "root.crt"), filepath.Join(dir, "intermediate.crt")
	writeTestCert(t, root, "Root")
	writeTestCert(t, intermediate, "Intermediate")
	c := config{RootCAPath: root, IntermediateCA: intermediate, SCEPURL: "https://ca.example.org/scep/wifi-device",
		SCEPProvisionerName: "wifi-device", SSID: "Test & WiFi", OnboardingSSID: "Test Setup",
		EncryptionType:   "WPA3",
		RadiusServerName: "radius.example.org", Hidden: false}
	p, err := generateProfile(c, profileInput{DisplayName: "OnCloud WiFi – Phone",
		IdentityCN: "wifi-0123456789ab-00000000-0000-4000-8000-000000000001",
		DeviceUUID: "00000000-0000-4000-8000-000000000001",
		Email:      "verified@example.test", Challenge: "secret-challenge"})
	if err != nil {
		t.Fatal(err)
	}
	text := string(p)
	var plist struct {
		XMLName xml.Name `xml:"plist"`
	}
	if err := xml.Unmarshal(p, &plist); err != nil {
		t.Fatalf("generated profile is not valid XML: %v", err)
	}
	for _, required := range []string{
		"<integer>13</integer>", "<string>radius.example.org</string>",
		"<integer>2048</integer>", "<integer>5</integer>",
		"<key>KeyIsExtractable</key><false/>", "Test &amp; WiFi",
		"<key>uniformResourceIdentifier</key>",
		"<key>DisableAssociationMACRandomization</key><true/>",
		"<key>SSID_STR</key><string>Test Setup</string>",
		"<key>AutoJoin</key><false/>",
		"<key>CaptiveBypass</key><false/>",
		"<key>EncryptionType</key><string>None</string>",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("missing %q", required)
		}
	}
	if strings.Contains(text, "<key>Password</key>") {
		t.Fatal("password payload unexpectedly present")
	}
	if strings.Contains(text, "<key>URI</key>") {
		t.Fatal("obsolete or invented URI SAN key unexpectedly present")
	}
	if strings.Count(text, "<string>com.apple.wifi.managed</string>") != 2 {
		t.Fatal("profile must contain exactly target and onboarding Wi-Fi payloads")
	}
	target := strings.Index(text, "<key>SSID_STR</key><string>Test &amp; WiFi</string>")
	onboarding := strings.Index(text, "<key>SSID_STR</key><string>Test Setup</string>")
	if target == -1 || onboarding == -1 || onboarding < target {
		t.Fatal("onboarding auto-join disable payload must follow the target Wi-Fi payload")
	}
}

func TestProfileRejectsUnsafeOnboardingSSID(t *testing.T) {
	dir := t.TempDir()
	root, intermediate := filepath.Join(dir, "root.crt"), filepath.Join(dir, "intermediate.crt")
	writeTestCert(t, root, "Root")
	writeTestCert(t, intermediate, "Intermediate")
	base := config{
		RootCAPath: root, IntermediateCA: intermediate,
		SCEPURL: "https://ca.example.org/scep/wifi-device", SCEPProvisionerName: "wifi-device",
		SSID: "ExampleNet-EAPTLS", EncryptionType: "WPA3", RadiusServerName: "radius.example.org",
	}
	in := profileInput{
		DisplayName: "OnCloud WiFi – Phone",
		IdentityCN:  "wifi-0123456789ab-00000000-0000-4000-8000-000000000001",
		DeviceUUID:  "00000000-0000-4000-8000-000000000001",
		Challenge:   "secret-challenge",
	}
	if _, err := generateProfile(base, in); err == nil {
		t.Fatal("missing onboarding SSID unexpectedly accepted")
	}
	base.OnboardingSSID = base.SSID
	if _, err := generateProfile(base, in); err == nil {
		t.Fatal("target SSID reused as onboarding SSID unexpectedly accepted")
	}
}

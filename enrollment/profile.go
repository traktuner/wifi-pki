package main

import (
	"bytes"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type profileInput struct {
	DisplayName, IdentityCN, DeviceUUID, Email, Challenge string
}

func uuidV4() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}

func certDER(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if block, _ := pem.Decode(raw); block != nil {
		raw = block.Bytes
	}
	if _, err := x509.ParseCertificate(raw); err != nil {
		return nil, fmt.Errorf("parse certificate %s: %w", path, err)
	}
	return raw, nil
}

func x(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func generateProfile(c config, in profileInput) ([]byte, error) {
	if strings.TrimSpace(c.OnboardingSSID) == "" {
		return nil, errors.New("onboarding SSID is required")
	}
	if c.OnboardingSSID == c.SSID {
		return nil, errors.New("onboarding SSID must differ from target SSID")
	}
	rootDER, err := certDER(c.RootCAPath)
	if err != nil {
		return nil, err
	}
	intermediateDER, err := certDER(c.IntermediateCA)
	if err != nil {
		return nil, err
	}
	profileUUID, _ := uuidV4()
	rootUUID, _ := uuidV4()
	intermediateUUID, _ := uuidV4()
	scepUUID, _ := uuidV4()
	wifiUUID, _ := uuidV4()
	onboardingWifiUUID, _ := uuidV4()
	if profileUUID == "" || rootUUID == "" || intermediateUUID == "" || scepUUID == "" || wifiUUID == "" || onboardingWifiUUID == "" {
		return nil, errors.New("uuid generation failed")
	}
	emailSAN := ""
	if in.Email != "" {
		emailSAN = "<key>rfc822Name</key><string>" + x(in.Email) + "</string>"
	}
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>PayloadType</key><string>Configuration</string>
<key>PayloadVersion</key><integer>1</integer>
<key>PayloadIdentifier</key><string>at.oncloud.wifi.profile.%s</string>
<key>PayloadUUID</key><string>%s</string>
<key>PayloadDisplayName</key><string>%s</string>
<key>PayloadDescription</key><string>Individual OnCloud EAP-TLS identity. The private key is generated on this device.</string>
<key>PayloadRemovalDisallowed</key><false/>
<key>PayloadContent</key><array>
<dict><key>PayloadType</key><string>com.apple.security.root</string>
<key>PayloadVersion</key><integer>1</integer><key>PayloadIdentifier</key><string>at.oncloud.wifi.root.%s</string>
<key>PayloadUUID</key><string>%s</string><key>PayloadDisplayName</key><string>OnCloud WiFi Root CA</string>
<key>PayloadContent</key><data>%s</data></dict>
<dict><key>PayloadType</key><string>com.apple.security.pem</string>
<key>PayloadVersion</key><integer>1</integer><key>PayloadIdentifier</key><string>at.oncloud.wifi.issuing.%s</string>
<key>PayloadUUID</key><string>%s</string><key>PayloadDisplayName</key><string>OnCloud WiFi Issuing CA</string>
<key>PayloadContent</key><data>%s</data></dict>
<dict><key>PayloadType</key><string>com.apple.security.scep</string>
<key>PayloadVersion</key><integer>1</integer><key>PayloadIdentifier</key><string>at.oncloud.wifi.scep.%s</string>
<key>PayloadUUID</key><string>%s</string><key>PayloadDisplayName</key><string>OnCloud WiFi Device Identity</string>
<key>PayloadContent</key><dict>
<key>URL</key><string>%s</string><key>Name</key><string>%s</string>
<key>Challenge</key><string>%s</string>
<key>Subject</key><array><array><array><string>CN</string><string>%s</string></array></array></array>
<key>SubjectAltName</key><dict><key>uniformResourceIdentifier</key><string>urn:oncloud:wifi-device:%s</string>%s</dict>
<key>Key Type</key><string>RSA</string><key>Keysize</key><integer>2048</integer>
<key>Key Usage</key><integer>5</integer><key>KeyIsExtractable</key><false/>
<key>Retries</key><integer>3</integer><key>RetryDelay</key><integer>10</integer>
</dict></dict>
<dict><key>PayloadType</key><string>com.apple.wifi.managed</string>
<key>PayloadVersion</key><integer>1</integer><key>PayloadIdentifier</key><string>at.oncloud.wifi.network.%s</string>
<key>PayloadUUID</key><string>%s</string><key>PayloadDisplayName</key><string>%s</string>
<key>SSID_STR</key><string>%s</string><key>HIDDEN_NETWORK</key><%s/>
<key>AutoJoin</key><true/><key>DisableAssociationMACRandomization</key><true/>
<key>EncryptionType</key><string>%s</string>
<key>PayloadCertificateUUID</key><string>%s</string>
<key>EAPClientConfiguration</key><dict>
<key>AcceptEAPTypes</key><array><integer>13</integer></array>
<key>UserName</key><string>%s</string>
<key>TLSCertificateIsRequired</key><true/>
<key>TLSTrustedServerNames</key><array><string>%s</string></array>
<key>PayloadCertificateAnchorUUID</key><array><string>%s</string></array>
<key>TLSMinimumVersion</key><string>1.2</string><key>TLSMaximumVersion</key><string>1.2</string>
</dict></dict>
<dict><key>PayloadType</key><string>com.apple.wifi.managed</string>
<key>PayloadVersion</key><integer>1</integer><key>PayloadIdentifier</key><string>at.oncloud.wifi.onboarding.%s</string>
<key>PayloadUUID</key><string>%s</string><key>PayloadDisplayName</key><string>OnCloud WiFi Setup – Auto-Join aus</string>
<key>SSID_STR</key><string>%s</string><key>HIDDEN_NETWORK</key><false/>
<key>AutoJoin</key><false/><key>CaptiveBypass</key><false/>
<key>EncryptionType</key><string>None</string>
</dict></array></dict></plist>`,
		profileUUID, profileUUID, x(in.DisplayName),
		rootUUID, rootUUID, base64.StdEncoding.EncodeToString(rootDER),
		intermediateUUID, intermediateUUID, base64.StdEncoding.EncodeToString(intermediateDER),
		scepUUID, scepUUID, x(c.SCEPURL), x(c.SCEPProvisionerName), x(in.Challenge),
		x(in.IdentityCN), x(in.DeviceUUID), emailSAN,
		wifiUUID, wifiUUID, x(in.DisplayName), x(c.SSID),
		map[bool]string{true: "true", false: "false"}[c.Hidden], x(c.EncryptionType),
		scepUUID, x(in.IdentityCN), x(c.RadiusServerName), rootUUID,
		onboardingWifiUUID, onboardingWifiUUID, x(c.OnboardingSSID))
	return []byte(payload), nil
}

func signProfile(c config, unsigned []byte) ([]byte, error) {
	if c.SigningCert == "" || c.SigningKey == "" || c.SigningPasswordFile == "" {
		if c.AllowUnsigned {
			return unsigned, nil
		}
		return nil, errors.New("profile signing material is required")
	}
	dir, err := os.MkdirTemp("/tmp", "mobileconfig-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	input, output := filepath.Join(dir, "profile.plist"), filepath.Join(dir, "profile.mobileconfig")
	if err := os.WriteFile(input, unsigned, 0600); err != nil {
		return nil, err
	}
	cmd := exec.Command("openssl", "cms", "-sign", "-binary", "-nodetach",
		"-outform", "DER", "-in", input, "-out", output,
		"-signer", c.SigningCert, "-inkey", c.SigningKey,
		"-passin", "file:"+c.SigningPasswordFile)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("CMS signing failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return os.ReadFile(output)
}

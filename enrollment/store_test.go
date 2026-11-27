package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) *store {
	t.Helper()
	s, err := openStore(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.db.Close() })
	return s
}

func TestChallengeTransactionBindingAndReplay(t *testing.T) {
	s := testStore(t)
	now := time.Now().UTC()
	hash := []byte("32-byte-challenge-hash-material!")
	e := enrollment{
		ID: "enrollment-1", Sub: "sub", Email: "verified@example.test",
		DeviceName: "Test iPhone", DeviceUUID: "00000000-0000-4000-8000-000000000001",
		IdentityCN:    "wifi-0123456789ab-00000000-0000-4000-8000-000000000001",
		ChallengeHash: hash, ChallengeExpires: now.Add(10 * time.Minute),
		Expires: now.Add(1825 * 24 * time.Hour),
	}
	if err := s.createEnrollment(context.Background(), e, 5); err != nil {
		t.Fatal(err)
	}
	if _, allowed, err := s.consumeChallenge(context.Background(), hash, "tx-1", now, 2*time.Minute); err != nil || !allowed {
		t.Fatalf("first use rejected: allowed=%v err=%v", allowed, err)
	}
	if _, allowed, err := s.consumeChallenge(context.Background(), hash, "tx-1", now.Add(time.Minute), 2*time.Minute); err != nil || !allowed {
		t.Fatalf("same transaction retry rejected: allowed=%v err=%v", allowed, err)
	}
	if _, allowed, _ := s.consumeChallenge(context.Background(), hash, "tx-2", now.Add(time.Minute), 2*time.Minute); allowed {
		t.Fatal("different transaction replay accepted")
	}
	if _, allowed, _ := s.consumeChallenge(context.Background(), hash, "tx-1", now.Add(3*time.Minute), 2*time.Minute); allowed {
		t.Fatal("late retry accepted")
	}
}

func TestExpiredChallengeRejected(t *testing.T) {
	s := testStore(t)
	now := time.Now().UTC()
	hash := []byte("expired-hash")
	e := enrollment{ID: "expired", Sub: "sub", DeviceName: "Old Phone",
		DeviceUUID:    "00000000-0000-4000-8000-000000000002",
		IdentityCN:    "wifi-0123456789ab-00000000-0000-4000-8000-000000000002",
		ChallengeHash: hash, ChallengeExpires: now.Add(-time.Second), Expires: now.Add(time.Hour)}
	if err := s.createEnrollment(context.Background(), e, 5); err != nil {
		t.Fatal(err)
	}
	if _, allowed, _ := s.consumeChallenge(context.Background(), hash, "tx", now, time.Minute); allowed {
		t.Fatal("expired challenge accepted")
	}
}

func TestCancelEnrollmentInvalidatesChallengeAndChecksOwner(t *testing.T) {
	s := testStore(t)
	now := time.Now().UTC()
	hash := []byte("cancelled-challenge-hash")
	e := enrollment{ID: "cancel", Sub: "owner-sub", DeviceName: "Old iPhone",
		DeviceUUID:    "00000000-0000-4000-8000-000000000004",
		IdentityCN:    "wifi-0123456789ab-00000000-0000-4000-8000-000000000004",
		ChallengeHash: hash, ChallengeExpires: now.Add(10 * time.Minute),
		Expires: now.Add(1825 * 24 * time.Hour)}
	if err := s.createEnrollment(context.Background(), e, 5); err != nil {
		t.Fatal(err)
	}
	if err := s.cancelEnrollment(context.Background(), "other-sub", e.DeviceUUID); err == nil {
		t.Fatal("different user cancelled enrollment")
	}
	if err := s.cancelEnrollment(context.Background(), e.Sub, e.DeviceUUID); err != nil {
		t.Fatal(err)
	}
	if _, allowed, err := s.consumeChallenge(context.Background(), hash, "tx", now, time.Minute); err != nil || allowed {
		t.Fatalf("cancelled challenge accepted: allowed=%v err=%v", allowed, err)
	}
	devices, err := s.listDevices(context.Background(), e.Sub)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 0 {
		t.Fatalf("cancelled enrollment remained visible: %+v", devices)
	}
	replacement := enrollment{
		ID: "replacement", Sub: e.Sub, DeviceName: "New iPhone",
		DeviceUUID:       "00000000-0000-4000-8000-000000000005",
		IdentityCN:       "wifi-0123456789ab-00000000-0000-4000-8000-000000000005",
		ChallengeHash:    []byte("replacement-challenge-hash"),
		ChallengeExpires: now.Add(10 * time.Minute),
		Expires:          now.Add(1825 * 24 * time.Hour),
	}
	if err := s.createEnrollment(context.Background(), replacement, 1); err != nil {
		t.Fatalf("cancelled enrollment still consumed active-device quota: %v", err)
	}
}

func TestBindAndMarkRevoked(t *testing.T) {
	s := testStore(t)
	now := time.Now().UTC()
	e := enrollment{ID: "revoke", Sub: "sub", DeviceName: "Lost Phone",
		DeviceUUID:    "00000000-0000-4000-8000-000000000003",
		IdentityCN:    "wifi-0123456789ab-00000000-0000-4000-8000-000000000003",
		ChallengeHash: []byte("revoke-hash"), ChallengeExpires: now.Add(time.Minute),
		Expires: now.Add(1825 * 24 * time.Hour)}
	if err := s.createEnrollment(context.Background(), e, 5); err != nil {
		t.Fatal(err)
	}
	if err := s.bindSerial(context.Background(), e.DeviceUUID, "123456789"); err != nil {
		t.Fatal(err)
	}
	if err := s.markRevoked(context.Background(), "123456789"); err != nil {
		t.Fatal(err)
	}
	var status string
	var revokedAt *int64
	if err := s.db.QueryRow(`SELECT status,revoked_at FROM enrollments WHERE id=?`, e.ID).
		Scan(&status, &revokedAt); err != nil {
		t.Fatal(err)
	}
	if status != "revoked" || revokedAt == nil {
		t.Fatalf("revocation metadata not recorded: status=%q revoked=%v", status, revokedAt)
	}
}

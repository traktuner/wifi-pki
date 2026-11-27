package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProfileDownloadIsAuthenticatedAndOneTime(t *testing.T) {
	db := testStore(t)
	secret := []byte("abcdefghijklmnopqrstuvwxyz012345")
	sess := session{
		ID: "session-id", Sub: "owner-sub", Email: "owner@example.test", CSRF: "csrf",
		Expires: time.Now().UTC().Add(time.Minute),
	}
	if err := db.createSession(t.Context(), sess); err != nil {
		t.Fatal(err)
	}
	s := &server{cfg: config{SessionSecret: secret}, store: db, downloads: newDownloadStore()}
	id := "download-id"
	profile := []byte("signed-mobileconfig")
	s.downloads.put(id, pendingDownload{
		Profile: profile, Filename: "oncloud-wifi-phone.mobileconfig",
		Subject: sess.Sub, Name: "Phone", Expires: time.Now().UTC().Add(time.Minute),
	})

	request := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/download", nil)
		req.AddCookie(&http.Cookie{Name: "__Host-wifi-session", Value: signOpaque(secret, sess.ID)})
		req.AddCookie(&http.Cookie{Name: "__Host-wifi-download", Value: signOpaque(secret, id)})
		rec := httptest.NewRecorder()
		s.download(rec, req)
		return rec
	}

	first := request()
	if first.Code != http.StatusOK || !bytes.Equal(first.Body.Bytes(), profile) {
		t.Fatalf("first download failed: status=%d body=%q", first.Code, first.Body.Bytes())
	}
	if first.Header().Get("Content-Type") != "application/x-apple-aspen-config" ||
		first.Header().Get("Cache-Control") != "no-store, private, max-age=0" {
		t.Fatalf("unsafe download headers: %v", first.Header())
	}
	if second := request(); second.Code != http.StatusGone {
		t.Fatalf("download was reusable: status=%d", second.Code)
	}
}

func TestProfileDownloadRejectsDifferentUser(t *testing.T) {
	s := &server{downloads: newDownloadStore()}
	s.downloads.put("id", pendingDownload{
		Profile: []byte("profile"), Subject: "owner", Expires: time.Now().UTC().Add(time.Minute),
	})
	if _, ok := s.downloads.get("id", "other-user", true); ok {
		t.Fatal("different user obtained profile")
	}
	if _, ok := s.downloads.get("id", "owner", true); !ok {
		t.Fatal("owner profile was consumed by rejected request")
	}
}

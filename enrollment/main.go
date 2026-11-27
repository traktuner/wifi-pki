package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		healthcheck()
		return
	}
	cfg, err := loadConfig()
	if err != nil {
		slog.Error("configuration rejected", "error", err)
		os.Exit(1)
	}
	db, err := openStore(cfg.DBPath)
	if err != nil {
		slog.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	defer db.db.Close()
	if len(os.Args) > 1 && os.Args[1] == "admin" {
		admin(db)
		return
	}
	srv, err := newServer(context.Background(), cfg, db)
	if err != nil {
		slog.Error("OIDC initialization failed", "error", err)
		os.Exit(1)
	}
	httpServer := &http.Server{
		Addr: cfg.ListenAddr, Handler: srv.routes(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 16 << 10,
	}
	webhookServer := &http.Server{
		Addr: cfg.WebhookListenAddr, Handler: srv.webhookRoutes(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second,
		MaxHeaderBytes: 16 << 10,
	}
	errs := make(chan error, 2)
	go func() {
		slog.Info("wifi enrollment listening", "address", cfg.ListenAddr)
		errs <- httpServer.ListenAndServe()
	}()
	go func() {
		slog.Info("internal SCEP webhook listening", "address", cfg.WebhookListenAddr)
		errs <- webhookServer.ListenAndServeTLS(cfg.WebhookTLSCert, cfg.WebhookTLSKey)
	}()
	if err := <-errs; err != nil && err != http.ErrServerClosed {
		slog.Error("http server failed", "error", err)
		os.Exit(1)
	}
}

func healthcheck() {
	if len(os.Args) != 3 {
		os.Exit(2)
	}
	client := http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(os.Args[2])
	if err != nil {
		os.Exit(1)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		os.Exit(1)
	}
}

func admin(db *store) {
	if len(os.Args) == 3 && os.Args[2] == "list" {
		if err := db.listAdmin(context.Background()); err != nil {
			slog.Error("admin list failed", "error", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 5 && os.Args[2] == "bind-serial" {
		if err := db.bindSerial(context.Background(), os.Args[3], os.Args[4]); err != nil {
			slog.Error("bind serial failed", "error", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 4 && os.Args[2] == "mark-revoked" {
		if err := db.markRevoked(context.Background(), os.Args[3]); err != nil {
			slog.Error("mark revoked failed", "error", err)
			os.Exit(1)
		}
		return
	}
	fmt.Fprintln(os.Stderr, "usage: wifi-enrollment admin list | admin bind-serial DEVICE_UUID SERIAL | admin mark-revoked SERIAL")
	os.Exit(2)
}

func readAll(reader io.Reader) ([]byte, error) {
	return io.ReadAll(reader)
}

func trimSecret(value string) string { return strings.TrimSpace(value) }

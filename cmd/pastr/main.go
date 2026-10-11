// Command pastr serves a self-hosted, spoiler-safe pastebin.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/santhoshjanan/pastr/internal/auth"
	"github.com/santhoshjanan/pastr/internal/config"
	"github.com/santhoshjanan/pastr/internal/store"
	"github.com/santhoshjanan/pastr/internal/web"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	if err := run(log); err != nil {
		log.Error("pastr stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o700); err != nil {
		return err
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := bootstrapAdmin(ctx, cfg, st, time.Now()); err != nil {
		return err
	}
	srv, err := web.New(cfg, st, log)
	if err != nil {
		return err
	}
	go func() {
		tick := time.NewTicker(time.Hour)
		defer tick.Stop()
		for {
			sweep(ctx, st, time.Now(), log)
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()

	httpSrv := &http.Server{
		Addr: cfg.Addr, Handler: srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second,
		MaxHeaderBytes: 16 << 10,
	}
	errc := make(chan error, 1)
	go func() { errc <- httpSrv.ListenAndServe() }()
	log.Info("pastr listening", "addr", cfg.Addr, "base_url", cfg.BaseURL)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// bootstrapAdmin creates the first account from the environment when no users exist.
// Once any user exists the admin variables are ignored.
func bootstrapAdmin(ctx context.Context, cfg config.Config, st *store.Store, now time.Time) error {
	n, err := st.CountUsers(ctx)
	if err != nil || n > 0 {
		return err
	}
	if cfg.AdminUsername == "" {
		return errors.New("no users exist yet: set PASTR_ADMIN_USERNAME and PASTR_ADMIN_PASSWORD to create the first admin")
	}
	username, err := auth.ValidateUsername(cfg.AdminUsername)
	if err != nil {
		return fmt.Errorf("PASTR_ADMIN_USERNAME: %w", err)
	}
	if err := auth.ValidatePassword(cfg.AdminPassword); err != nil {
		return fmt.Errorf("PASTR_ADMIN_PASSWORD: %w", err)
	}
	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}
	_, err = st.CreateUser(ctx, username, hash, true, now)
	return err
}

func sweep(ctx context.Context, st *store.Store, now time.Time, log *slog.Logger) {
	pastes, err := st.DeleteExpiredPastes(ctx, now)
	if err != nil {
		log.Error("sweep pastes", "err", err)
	}
	sessions, err := st.DeleteExpiredSessions(ctx, now)
	if err != nil {
		log.Error("sweep sessions", "err", err)
	}
	if pastes+sessions > 0 {
		log.Info("swept", "pastes", pastes, "sessions", sessions)
	}
}

// healthURL turns the listen address into one the health check can dial:
// loopback when pastr listens on every interface, the given host otherwise.
func healthURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host, port = "", "8080"
	}
	if ip, perr := netip.ParseAddr(host); host == "" || (perr == nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/healthz"
}

// healthcheck is the container's HEALTHCHECK command; the image has no shell or curl.
func healthcheck() int {
	addr := os.Getenv("PASTR_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(healthURL(addr))
	if err != nil {
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

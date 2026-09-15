package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/MikeO7/kinosail-dashboard/internal/auth"
	"github.com/MikeO7/kinosail-dashboard/internal/dashboard"
	"github.com/MikeO7/kinosail-dashboard/internal/database"
	"github.com/MikeO7/kinosail-dashboard/internal/server"
	"github.com/MikeO7/kinosail-dashboard/internal/supporter"
)

var version = "dev"

type application struct {
	store     *database.Store
	board     *dashboard.Service
	auth      *auth.Manager
	prober    *dashboard.Prober
	supporter *supporter.Service
}

type runtimeConfig struct {
	listen                      string
	dataDir                     string
	probeInterval               time.Duration
	publicHosts                 []string
	trustedHosts                []string
	publicURL                   string
	secureCookies               bool
	supporterURL, activationURL string
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	command, err := requestedCommand(os.Args[1:])
	if err != nil {
		fail(err)
	}
	if command == "version" {
		fmt.Println(version)
		return
	}
	configured, err := loadRuntimeConfig()
	if err != nil {
		fail(err)
	}
	if command == "healthcheck" {
		if err := healthcheck(ctx, configured.listen); err != nil {
			fail(err)
		}
		return
	}
	app, err := openApplication(ctx, configured)
	if err != nil {
		fail(err)
	}
	defer app.store.Close()
	if err := serve(ctx, app, configured); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fail(err)
	}
}

func openApplication(ctx context.Context, configured runtimeConfig) (*application, error) {
	store, err := database.Open(ctx, configured.dataDir)
	if err != nil {
		return nil, err
	}
	board, err := dashboard.NewService(ctx, store)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	authentication, err := auth.NewManager(ctx, store)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	supporterService, err := supporter.New(ctx, store, supporter.Config{ActivationURL: configured.activationURL, SupportURL: configured.supporterURL})
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	return &application{store: store, board: board, auth: authentication, prober: dashboard.NewProber(board, configured.probeInterval, configured.publicHosts), supporter: supporterService}, nil
}

func serve(ctx context.Context, app *application, configured runtimeConfig) error { //nolint:contextcheck // Handler construction does not start context-bound work.
	//nolint:contextcheck // Handler construction does not start context-bound work.
	httpServer := newHTTPServer(configured.listen, server.New(server.Config{SecureCookies: configured.secureCookies, TrustedHosts: configured.trustedHosts, PublicURL: configured.publicURL}, app.board, app.auth, app.prober, app.supporter))
	go app.prober.Run(ctx)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	slog.Info("Kinosail Dashboard ready", "listen", httpServer.Addr)
	return httpServer.ListenAndServe()
}

func healthcheck(ctx context.Context, listen string) error {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return errors.New("listen address is invalid")
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	requestCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/healthz", nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned %d", response.StatusCode)
	}
	return nil
}

func requestedCommand(args []string) (string, error) {
	if len(args) > 1 {
		return "", errors.New("usage: kinosail-dashboard [healthcheck|version]")
	}
	command := "serve"
	if len(args) == 1 {
		command = args[0]
	}
	if command != "serve" && command != "healthcheck" && command != "version" {
		return "", fmt.Errorf("unknown command %q", command)
	}
	return command, nil
}

func loadRuntimeConfig() (runtimeConfig, error) {
	configured := runtimeConfig{listen: valueOrDefault("KINOSAIL_DASHBOARD_LISTEN", "127.0.0.1:38400"), dataDir: valueOrDefault("KINOSAIL_DASHBOARD_DATA_DIR", "./data"), probeInterval: 30 * time.Second}
	err := validateRuntimePaths(configured.listen, configured.dataDir)
	if err != nil {
		return runtimeConfig{}, err
	}
	configured.secureCookies, err = strconv.ParseBool(valueOrDefault("KINOSAIL_DASHBOARD_SECURE_COOKIES", "false"))
	if err != nil {
		return runtimeConfig{}, errors.New("KINOSAIL_DASHBOARD_SECURE_COOKIES must be true or false")
	}
	configured.publicURL, err = parsePublicURL(os.Getenv("KINOSAIL_DASHBOARD_PUBLIC_URL"), configured.listen)
	if err != nil {
		return runtimeConfig{}, err
	}
	if strings.HasPrefix(configured.publicURL, "https://") != configured.secureCookies {
		return runtimeConfig{}, errors.New("secure cookies must match the public URL scheme")
	}
	configured.probeInterval, err = parseProbeInterval(os.Getenv("KINOSAIL_DASHBOARD_PROBE_INTERVAL"))
	if err != nil {
		return runtimeConfig{}, err
	}
	configured.publicHosts, err = parsePublicHosts(os.Getenv("KINOSAIL_DASHBOARD_PUBLIC_PROBE_HOSTS"))
	if err != nil {
		return runtimeConfig{}, err
	}
	configured.trustedHosts, err = parseTrustedHosts(os.Getenv("KINOSAIL_DASHBOARD_TRUSTED_HOSTS"))
	if err != nil {
		return runtimeConfig{}, err
	}
	configured.activationURL = strings.TrimSpace(os.Getenv("KINOSAIL_SUPPORTER_ACTIVATION_URL"))
	configured.supporterURL = strings.TrimSpace(os.Getenv("KINOSAIL_SUPPORTER_URL"))
	if len(configured.activationURL) > 2048 || len(configured.supporterURL) > 2048 {
		return runtimeConfig{}, errors.New("supporter URL is invalid")
	}
	return configured, nil
}

func parsePublicURL(value, listen string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultPublicURL(listen)
	}
	if len(value) > 2048 {
		return "", errors.New("public URL is invalid")
	}
	parsed, err := url.Parse(value)
	if err != nil || !validPublicURL(parsed) {
		return "", errors.New("public URL is invalid")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func defaultPublicURL(listen string) (string, error) {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", errors.New("listen address is invalid")
	}
	address := net.ParseIP(host)
	if host == "" || host == "0.0.0.0" || host == "::" || address != nil && address.IsLoopback() {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port), nil
}

func validPublicURL(parsed *url.URL) bool {
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && (parsed.Path == "" || parsed.Path == "/")
}

func parseTrustedHosts(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	if len(parts) > 50 {
		return nil, errors.New("trusted host list supports at most 50 hosts")
	}
	hosts := make([]string, 0, len(parts))
	for _, part := range parts {
		host := strings.ToLower(strings.TrimSuffix(strings.Trim(strings.TrimSpace(part), "[]"), "."))
		if net.ParseIP(host) == nil && !validHostname(host) {
			return nil, errors.New("trusted host list contains an invalid host")
		}
		hosts = append(hosts, host)
	}
	return hosts, nil
}

func parsePublicHosts(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	if len(parts) > 50 {
		return nil, errors.New("public probe host list supports at most 50 hosts")
	}
	hosts := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, part := range parts {
		host := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(part), "."))
		if net.ParseIP(host) != nil || !validHostname(host) {
			return nil, errors.New("public probe host list contains an invalid host")
		}
		if !seen[host] {
			seen[host] = true
			hosts = append(hosts, host)
		}
	}
	return hosts, nil
}

func valueOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
func fail(err error) { slog.Error("Kinosail Dashboard stopped", "error", err); os.Exit(1) }

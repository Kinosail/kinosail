package server

import (
	"context"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/webassets"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/library"
	sharedmetadata "github.com/MikeO7/kinosail/packages/metadata"
	"github.com/MikeO7/kinosail/packages/remoteaccess"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
	"github.com/MikeO7/kinosail/packages/workload"
)

type Config struct {
	Lifecycle           context.Context
	MediaDir            string
	DataDir             string
	CacheDir            string
	FFmpeg              string
	FFprobe             string
	FPCalc              string
	RequireAuth         bool
	AuthURL             string
	ScanInterval        time.Duration
	Subtitles           SubtitleConfig
	Metadata            MetadataConfig
	TMDBToken           string
	TMDBURL             string
	TMDBImageURL        string
	OIDC                OIDCConfig
	SAML                SAMLConfig
	MCP                 MCPConfig
	ProxyToken          string
	Notifications       NotificationConfig
	BackupDir           string
	BackupKey           string
	BackupInterval      time.Duration
	BackupRetention     int
	MaintenanceInterval time.Duration
	Configuration       configuration.Snapshot
	InternetAccess      *remoteaccess.Manager
	TrustedHTTPS        *trustedhttps.Manager
	Supporter           SupporterConfig
	SubtitleApp         bool

	SubtitleOperationTime SubtitleOperationTime
}

func New(config Config) http.Handler {
	return newApplication(config)
}

func newApplication(config Config) http.Handler { //nolint:funlen,cyclop,gocognit // The composition root validates defaults and wires every adapter to shared application services.
	lifecycle, managedLifecycle := prepareApplicationConfig(&config)
	startupReady := false
	if managedLifecycle {
		var cancel context.CancelFunc
		config.Lifecycle, cancel = context.WithCancel(config.Lifecycle)
		lifecycle = config.Lifecycle
		defer func() {
			if !startupReady {
				cancel()
			}
		}()
	}
	stateDB, err := database.OpenContext(config.Lifecycle, config.DataDir, managedLifecycle)
	if err != nil {
		return unavailableApplication(config, "application state is unavailable")
	}
	beforeDatabaseClose := closeApplicationDatabase(config.Lifecycle, stateDB, managedLifecycle)
	settings, updates, unavailable := initializeApplicationSettings(config, stateDB, managedLifecycle)
	if unavailable != "" {
		return unavailableApplication(config, unavailable)
	}
	supporter := newSupporterProgram(settings, config.Supporter)
	workloads := workload.New(workload.HeavyCapacity())
	metadata := newMetadataStore(config.Metadata, config.DataDir, config.CacheDir, stateDB)
	if metadata.err != nil {
		return unavailableApplication(config, "application state is unavailable")
	}
	probe := newMediaProbe(config.FFprobe)
	probe.ffmpeg, probe.cacheDir = config.FFmpeg, config.CacheDir
	legacyMetadata := sharedmetadata.NewTMDB(sharedmetadata.TMDBConfig{Token: config.TMDBToken, URL: config.TMDBURL, ImageURL: config.TMDBImageURL, CacheDir: config.CacheDir})
	index := &libraryIndex{Index: catalog.NewIndex(config.Lifecycle, settings.roots(), config.CacheDir, config.ScanInterval, func(ctx context.Context) (func(), error) {
		return workloads.Acquire(ctx, workload.Background)
	})}
	index.SetDecorator(func(items []library.Item) []library.Item {
		return metadata.apply(legacyMetadata.Enrich(config.Lifecycle, probe.decorate(config.Lifecycle, items)))
	})
	index.SetFrequency(settings.scanFrequency())
	if managedLifecycle {
		index.Schedule(config.Lifecycle)
		index.Watch(config.Lifecycle)
	}
	auth := newAuthentication(config.Lifecycle, config.DataDir, config.RequireAuth, config.AuthURL, settings, config.Notifications, stateDB, config.Configuration.Duration("logging.audit_retention"), config.Configuration.Duration("logging.playback_retention"))
	startAccessFlush(config.Lifecycle, managedLifecycle, auth.profiles, beforeDatabaseClose)
	if config.InternetAccess != nil && config.InternetAccess.Status().Mode == "https" {
		auth.passkeys.redirectPages = false
	}
	if !auth.audit.Healthy() {
		return unavailableApplication(config, "activity journal is unavailable")
	}
	disableInternetForBrokenProfiles(config, auth)
	agentConnections := newMCPConnections(config.AuthURL, config.DataDir, auth.profiles, stateDB)
	agentConnections.Configure(config.MCP)
	auth.audit.SetTitle(func(id string) string {
		item, _ := index.Find(id)
		return item.Title
	})
	auth.audit.SetSnapshot(settings.auditSnapshot)
	management := newOwnerAccess(config, auth)
	subtitles := newSubtitleProvider(config.Subtitles, config.CacheDir, config.DataDir, index, settings, config.FFmpeg, stateDB)
	settings.subtitleProvider = subtitles
	backups := newBackupManager(lifecycle, config.DataDir, config.BackupDir, config.BackupKey, config.BackupInterval, config.BackupRetention, workloads)
	subtitleManager := newSubtitleManager(index, settings, subtitles, probe, backups)
	subtitleManager.operations = newSubtitleOperations(config)
	subtitles.admission = subtitleManager.operations.admission
	subtitles.sync.admission = subtitles.admission
	if managedLifecycle && config.SubtitleApp {
		subtitleManager.schedule(config.Lifecycle)
	}
	maintenance := newMaintenanceManager(lifecycle, backups, metadata, config.MaintenanceInterval)
	events := wireLiveEvents(index)
	subtitles.ledger.setPublisher(func() { events.Publish("", "subtitles.updated", "/api/v1/subtitle-library") })
	mux := http.NewServeMux()
	mux.HandleFunc("GET /static/htmx.min.js", serveScript(htmx))
	mux.HandleFunc("GET /static/main.kinosail.bundle.js", serveScript(mainBundle))
	mux.HandleFunc("GET /static/pwa.js", serveScript(pwaJS))
	mux.HandleFunc("GET /static/subtitle-inspector.js", serveScript(subtitleInspectorJS))
	mux.HandleFunc("GET /static/subtitle-inspector.css", serveAsset(subtitleInspectorCSS, "text/css; charset=utf-8"))
	mux.HandleFunc("GET /static/supporter.js", serveScript(supporterJS))
	mux.HandleFunc("GET /static/supporter.css", serveSupporterStyle)
	mux.HandleFunc("GET /static/supporter/badges/{file}", serveSupporterBadge)
	mux.HandleFunc("GET /static/theme.js", serveScript(themeJS))
	mux.HandleFunc("GET /static/app.css", serveStyle)
	mux.HandleFunc("GET /manifest.webmanifest", serveAsset(manifest, "application/manifest+json"))
	mux.HandleFunc("GET /static/manrope.woff2", serveAsset(webassets.Manrope, "font/woff2"))
	mux.HandleFunc("GET /static/icon.svg", serveAsset(icon, "image/svg+xml"))
	mux.HandleFunc("GET /static/tmdb-logo.svg", serveAsset(tmdbLogo, "image/svg+xml"))
	mux.HandleFunc("GET /static/icon-192.png", serveAsset(icon192, "image/png"))
	mux.HandleFunc("GET /static/icon-512.png", serveAsset(icon512, "image/png"))
	mux.HandleFunc("GET /static/icon-maskable-512.png", serveAsset(iconMaskable512, "image/png"))
	mux.HandleFunc("GET /static/apple-touch-icon.png", serveAsset(appleTouchIcon, "image/png"))
	mux.HandleFunc("GET /static/cinema-backdrop.jpg", serveAsset(cinemaBackdrop, "image/jpeg"))
	mux.HandleFunc("GET /service-worker.js", serveServiceWorker)
	mux.HandleFunc("GET /offline", serveOffline)
	mux.HandleFunc("GET /favicon.ico", serveAsset(icon, "image/svg+xml"))
	registerIdentity(mux, auth, config.OIDC, config.SAML)
	registerSettings(mux, settings, updates, index, auth, config.TrustedHTTPS, subtitles, metadata, backups, maintenance, events)
	registerOwnerAccess(mux, auth, management, config.AuthURL)
	registerSupporter(mux, auth, supporter)
	registerAgentConnections(mux, auth, agentConnections, config.DataDir)
	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("GET /language", showLanguage)
	mux.HandleFunc("POST /language", saveWebLanguage)
	subtitleManager.register(mux, auth)
	if managedLifecycle {
		sharedmetadata.Schedule(config.Lifecycle, metadata.available(), index.AddAnalyzer, func(ctx context.Context) error { return metadata.refreshMissing(ctx, index) })
	}
	registerAPI(mux, apiServices{index, auth, settings, probe, metadata, subtitles, backups, maintenance, agentConnections, config.InternetAccess, config.TrustedHTTPS, supporter, updates, config.AuthURL, events})
	mcpAdapter := registerMCPWithConnections(mux, config.MCP, auth, apiRouting(mux), agentConnections)
	mcpAdapter.ObserveEvents(config.Lifecycle, events)
	startApplicationMCPHost(config, managedLifecycle, mcpAdapter, auth.profiles)
	registerFiles(mux, index, auth)
	mux.Handle("POST /scan", auth.owner(catalog.RescanHandler(index.Index, localizedError)))
	mux.Handle("GET /{$}", auth.owner(http.HandlerFunc(subtitleManager.dashboard)))
	handler := withApplicationRequests(maintenance.Track(apiRouting(mux)), mux, auth, config, settings, updates)
	if management != nil && managedLifecycle {
		management.Attach(config.Lifecycle, handler)
	}
	startupReady = true
	return handler
}

func withApplicationRequests(app http.Handler, appMux *http.ServeMux, auth *authentication, config Config, settings *settingsStore, updates *updateChecker) http.Handler {
	pattern := httpguard.MemoizedRoutePattern(func(request *http.Request) string {
		_, matched := appMux.Handler(request)
		return matched
	})
	return protectApplicationTransport(auth, config, pattern, applicationRequests(auth, settings, updates, pattern, app))
}

package server

import (
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail/packages/appcli"
)

const onboardingLANHTML = `<section class="wide onboarding-first-value" id="watch-everywhere" aria-labelledby="watch-everywhere-title"><h2 id="watch-everywhere-title">Watch on your TV and phone</h2>{{if .LAN.Managed}}<p>{{if .LAN.Shared}}Kinosail is shared with devices on your home network.{{else}}Kinosail only works on this computer.{{end}} Your installation controls this setting.</p>{{if .LAN.Addresses}}<p>On another device, open:</p><ul class="lan-addresses">{{range .LAN.Addresses}}<li><code>{{.}}</code></li>{{end}}</ul>{{end}}<p class="onboarding-actions"><a class="button" href="/onboarding/household">Continue</a></p>{{else}}<form action="/onboarding/devices" method="post"><label class="wizard-check"><input type="checkbox" name="share" value="true" {{if or .LAN.Shared (not .LAN.Chosen)}}checked{{end}}><span><strong>Share Kinosail with devices on my home network</strong><small>Your TVs, phones, and tablets on the same Wi-Fi can play your library. People still need a profile and password. Nothing is shared outside your home.</small></span></label><button>Continue</button></form>{{if .LAN.Shared}}{{if .LAN.Addresses}}<p>On another device, open:</p><ul class="lan-addresses">{{range .LAN.Addresses}}<li><code>{{.}}</code></li>{{end}}</ul><p>The first time, your browser warns about the certificate. This is expected for a new server.</p>{{end}}{{end}}{{end}}</section>`

const onboardingRestartHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="refresh" content="12;url={{.Next}}"><title>Restarting · Kinosail Player</title><link rel="stylesheet" href="/static/app.css"><script src="/static/theme.js"></script></head><body class="auth"><main class="grant-card" aria-live="polite"><img class="brand-mark" src="/static/icon.svg?v=8" alt=""><span class="eyebrow">Almost there</span><h1>{{if .Automatic}}Kinosail is restarting{{else}}Restart Kinosail to finish{{end}}</h1><p>{{if .Automatic}}This takes a few seconds. This page continues by itself.{{else}}Your choice is saved. Restart Kinosail, or restart this computer, to share it with your other devices.{{end}}</p><p><a class="button" href="{{.Next}}">Continue</a></p></main></body></html>`

var onboardingRestartView = newLocalizedTemplate("onboarding-restart", onboardingRestartHTML)

type lanView struct {
	Managed, Shared, Chosen bool
	Addresses               []string
}

func (store *settingsStore) lanSharing() lanView {
	store.mu.RLock()
	defer store.mu.RUnlock()
	host, port, configured := net.SplitHostPort(store.config.String("listen"))
	running, _, started := net.SplitHostPort(store.startupConfig.String("listen"))
	view := lanView{Managed: store.config.Managed("listen"), Shared: configured == nil && !loopbackHost(host), Chosen: store.config.Source("listen") == configuration.GUI}
	if started == nil && !loopbackHost(running) {
		view.Addresses = lanAddresses(port, store.startupConfig.Bool("tls.enabled"))
	}
	return view
}

func saveOnboardingDevices(settings *settingsStore, restart func() bool) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		share := request.PostFormValue("share") == "true"
		current := settings.lanSharing()
		if current.Managed || current.Shared == share && current.Chosen {
			http.Redirect(writer, request, "/onboarding/household", http.StatusSeeOther)
			return
		}
		_, port, err := net.SplitHostPort(settings.configuration().String("listen"))
		if err != nil {
			localizedError(writer, request, "the Server listening address is invalid", http.StatusConflict)
			return
		}
		host := "127.0.0.1"
		if share {
			host = "0.0.0.0"
		}
		if err := settings.changeConfiguration(request.Context(), "listen", net.JoinHostPort(host, port), false); err != nil {
			slog.Warn("network sharing change rejected", "operation", "onboarding.devices", "outcome", "rejected", "error_class", "configuration")
			localizedError(writer, request, "could not save network sharing", http.StatusBadRequest)
			return
		}
		slog.Info("network sharing changed", "operation", "onboarding.devices", "outcome", "saved", "shared", share)
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		automatic := restart()
		_ = onboardingRestartView.Execute(writer, request, struct {
			Next      string
			Automatic bool
		}{"/onboarding/household", automatic})
	}
}

func supervisedRestart() bool {
	if supervised, _ := strconv.ParseBool(os.Getenv("KINOSAIL_SUPERVISED")); !supervised {
		return false
	}
	time.AfterFunc(time.Second, func() {
		slog.Info("restarting to apply settings", "operation", "restart", "outcome", "requested")
		appcli.RequestShutdown()
	})
	return true
}

func loopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func lanAddresses(port string, tls bool) []string {
	scheme := "http"
	if tls {
		scheme = "https"
	}
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var result []string
	for _, address := range addresses {
		network, ok := address.(*net.IPNet)
		if !ok || network.IP.To4() == nil || !network.IP.IsPrivate() {
			continue
		}
		result = append(result, scheme+"://"+net.JoinHostPort(network.IP.String(), port))
		if len(result) == 4 {
			break
		}
	}
	return result
}

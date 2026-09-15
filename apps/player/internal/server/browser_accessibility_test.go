package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestCoreJourneysInRepresentativeBrowserViewports(t *testing.T) { //nolint:cyclop,gocognit // Browser setup assertions intentionally stay in one release test.
	chrome := chromeExecutable(t)
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Home.mp4"), []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	data := t.TempDir()
	app := server.New(server.Config{DataDir: data, MediaDir: media, RequireAuth: true})
	owner := signInTestProfile(t, app, "/setup", "name=Owner&password=test-password")
	app = server.New(server.Config{DataDir: data, MediaDir: media, RequireAuth: true})
	var csrf atomic.Value
	web := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		request.AddCookie(owner)
		if request.URL.Path == "/test-api-key" {
			request.Method, request.URL.Path, request.URL.RawQuery, request.Body, request.ContentLength = http.MethodPost, "/settings/api-keys", "", io.NopCloser(strings.NewReader("name=Phone&scopes=library")), int64(len("name=Phone&scopes=library"))
			request.AddCookie(&http.Cookie{Name: "kinosail_language", Value: "es", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.Header.Set("Origin", "http://"+request.Host)
			request.Header.Set("X-Kinosail-CSRF", csrf.Load().(string))
		}
		app.ServeHTTP(writer, request)
	}))
	t.Cleanup(web.Close)
	home, err := http.Get(web.URL + "/?lang=es") //nolint:noctx // Test server request is bounded by the test lifecycle.
	if err != nil {
		t.Fatal(err)
	}
	defer home.Body.Close()
	body, err := io.ReadAll(home.Body)
	if err != nil {
		t.Fatal(err)
	}
	csrfMatch := regexp.MustCompile(`name="kinosail-csrf" content="([A-Za-z0-9_-]+)"`).FindSubmatch(body)
	if len(csrfMatch) != 2 {
		t.Fatalf("home lacks CSRF token: %q", body)
	}
	csrf.Store(string(csrfMatch[1]))
	match := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(string(body))
	if len(match) != 2 {
		t.Fatalf("home lacks playable media: status=%d body=%q", home.StatusCode, body)
	}
	id := match[1]
	for _, test := range []struct {
		name, path, language string
		width, height, card  int
	}{{"phone Library", "/?lang=es", "es", 390, 844, 0}, {"phone onboarding", "/onboarding/migrate?lang=en", "en", 390, 844, 0}, {"phone onboarding household", "/onboarding/household?lang=en", "en", 390, 844, 0}, {"phone settings", "/settings?lang=en", "en", 390, 844, 0}, {"phone settings playback", "/settings?lang=en#playback", "en", 390, 844, 0}, {"phone settings access", "/settings?lang=en#access", "en", 390, 844, 0}, {"phone settings system", "/settings?lang=en#system", "en", 390, 844, 0}, {"phone API key", "/test-api-key?lang=es", "es", 390, 844, 1}, {"tablet settings access", "/settings?lang=en#access", "en", 720, 450, 0}, {"laptop Library", "/?lang=zh-Hans", "zh-Hans", 1280, 900, 0}, {"laptop onboarding", "/onboarding/migrate?lang=en", "en", 1280, 900, 0}, {"laptop onboarding household", "/onboarding/household?lang=en", "en", 1280, 900, 0}, {"laptop settings", "/settings?lang=en", "en", 1280, 900, 0}, {"laptop settings playback", "/settings?lang=en#playback", "en", 1280, 900, 0}, {"laptop settings access", "/settings?lang=en#access", "en", 1280, 900, 0}, {"laptop settings system", "/settings?lang=en#system", "en", 1280, 900, 0}, {"desktop settings access", "/settings?lang=en#access", "en", 1440, 900, 0}, {"TV player", "/watch/" + id + "?lang=ar", "ar", 1920, 1080, 0}} {
		t.Run(test.name, func(t *testing.T) {
			result := browserAudit(t, chrome, web.URL+test.path, test.width, test.height)
			wantActions := 0
			if test.name == "laptop onboarding" || test.name == "laptop onboarding household" {
				wantActions = 1
			}
			if result.Width != test.width || result.Height != test.height || result.Overflow != 0 || result.Main != 1 || result.Unnamed != 0 || result.StatusOverlaps != 0 || result.Language != test.language || result.Card != test.card || result.Actions != wantActions {
				t.Fatalf("browser audit = %+v", result)
			}
		})
	}
}

func TestAgentConnectionSettingsInRepresentativeBrowserViewports(t *testing.T) {
	chrome := chromeExecutable(t)
	web := httptest.NewServer(server.New(server.Config{DataDir: t.TempDir()}))
	t.Cleanup(web.Close)
	for _, viewport := range []struct{ width, height int }{{390, 844}, {1280, 900}} {
		result := browserAudit(t, chrome, web.URL+"/settings/agent-connections", viewport.width, viewport.height)
		if result.Width != viewport.width || result.Height != viewport.height || result.Overflow != 0 || result.Main != 1 || result.Unnamed != 0 {
			t.Fatalf("agent connection browser audit = %+v", result)
		}
	}
}

func TestCustomizedNavigationInRepresentativeBrowserViewports(t *testing.T) {
	chrome := chromeExecutable(t)
	data := t.TempDir()
	app := server.New(server.Config{DataDir: data})
	response := webFormCall(t, app, "", "/settings/navigation", url.Values{"items": {"home", "books", "movies"}})
	if response.Code != http.StatusSeeOther {
		t.Fatalf("save navigation = %d %q", response.Code, response.Body.String())
	}
	web := httptest.NewServer(server.New(server.Config{DataDir: data}))
	t.Cleanup(web.Close)
	for _, viewport := range []struct{ width, height int }{{1440, 900}, {1024, 768}, {720, 450}, {390, 844}, {320, 800}} {
		for _, path := range []string{"/", "/settings#navigation"} {
			result := browserAudit(t, chrome, web.URL+path, viewport.width, viewport.height)
			if result.Width != viewport.width || result.Height != viewport.height || result.Overflow != 0 || result.Main != 1 || result.Unnamed != 0 {
				t.Fatalf("%s at %dpx browser audit = %+v", path, viewport.width, result)
			}
		}
	}
}

type browserAuditResult struct {
	Width, Height, Overflow, Main, Unnamed, Card, Actions, StatusOverlaps int
	Language                                                              string
	Widest                                                                string
}

func browserAudit(t *testing.T, chrome, target string, width, height int) browserAuditResult { //nolint:cyclop,funlen,gocognit // CDP lifecycle failures are each asserted at their source.
	t.Helper()
	command, profile, port := startBrowser(t, chrome, target, width, height)
	t.Cleanup(func() { removeBrowserProfile(t, profile) })
	var connection *websocket.Conn
	t.Cleanup(func() { stopBrowser(command, connection) })
	endpoint := pageWebSocket(t, port, target)
	connection, upgradeResponse, err := websocket.Dial(t.Context(), endpoint, nil)
	if upgradeResponse != nil && upgradeResponse.Body != nil {
		defer upgradeResponse.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	auditContext, cancelAudit := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancelAudit()
	emulation := map[string]any{"id": 1, "method": "Emulation.setDeviceMetricsOverride", "params": map[string]any{"width": width, "height": height, "screenWidth": width, "screenHeight": height, "deviceScaleFactor": 1, "mobile": width < 700}}
	if err := wsjson.Write(auditContext, connection, emulation); err != nil {
		t.Fatal(err)
	}
	var acknowledged struct {
		ID    int            `json:"id"`
		Error map[string]any `json:"error"`
	}
	for acknowledged.ID != 1 {
		if err := wsjson.Read(auditContext, connection, &acknowledged); err != nil {
			t.Fatal(err)
		}
	}
	if acknowledged.Error != nil {
		t.Fatalf("emulate browser viewport: %v", acknowledged.Error)
	}
	expression := `(()=>{if(document.readyState!=="complete"||!document.querySelector("main")||!document.documentElement.lang)return "";const card=document.querySelector(".grant-card"),box=element=>{const rect=element?.getBoundingClientRect();return (element?.tagName)+"."+(element?.className)+":"+(rect?.left)+","+(rect?.right)+"/"+(element?.scrollWidth)},actions=[...document.querySelectorAll(".settings-shell>p:last-child>a.mode")],actionBoxes=actions.map(element=>element.getBoundingClientRect()),actionRow=actions[0]?.parentElement.getBoundingClientRect(),statusOverlaps=[...document.querySelectorAll(".settings-flow strong.status")].filter(status=>{const next=status.parentElement?.nextElementSibling;if(!next)return false;const a=status.getBoundingClientRect(),b=next.getBoundingClientRect();return a.bottom>b.top&&a.top<b.bottom&&a.right>b.left&&a.left<b.right}).length;return JSON.stringify({width:innerWidth,height:innerHeight,overflow:Math.max(0,document.documentElement.scrollWidth-innerWidth),main:document.querySelectorAll("main").length,language:document.documentElement.lang,unnamed:[...document.querySelectorAll("button,input:not([type=hidden]),select,textarea,audio,video")].filter(element=>!(element.getAttribute("aria-label")||element.labels?.[0]?.textContent?.trim()||element.textContent?.trim()||element.title)).length,card:card&&getComputedStyle(card).borderTopWidth!=="0px"?1:0,actions:actions.length===2&&actionRow.width>500&&Math.abs(actionBoxes[0].y-actionBoxes[1].y)<1?1:0,statusOverlaps,widest:[...(document.querySelector(".app-header")?.children||[])].map(box).join(";")})})()`
	deadline := time.Now().Add(15 * time.Second)
	for id := 2; time.Now().Before(deadline); id++ {
		request := map[string]any{"id": id, "method": "Runtime.evaluate", "params": map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true}}
		if err := wsjson.Write(auditContext, connection, request); err != nil {
			t.Fatal(err)
		}
		var response struct {
			ID     int `json:"id"`
			Result struct {
				Result struct {
					Value string `json:"value"`
				} `json:"result"`
			} `json:"result"`
		}
		for response.ID != id {
			if err := wsjson.Read(auditContext, connection, &response); err != nil {
				t.Fatal(err)
			}
		}
		var result browserAuditResult
		if json.Unmarshal([]byte(response.Result.Result.Value), &result) == nil && result.Main > 0 && result.Language != "" {
			return result
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("Chrome page did not finish navigating")
	return browserAuditResult{}
}

func startBrowser(t *testing.T, chrome, target string, width, height int) (*exec.Cmd, string, int) {
	t.Helper()
	for attempt := range 2 {
		profile, err := os.MkdirTemp("", "kinosail-browser-")
		if err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(t.Context(), chrome, "--headless=new", "--disable-gpu", "--disable-dev-shm-usage", "--remote-debugging-port=0", "--user-data-dir="+profile, fmt.Sprintf("--window-size=%d,%d", width, height), target) //nolint:gosec // Executable and arguments are test-owned.
		if err := command.Start(); err == nil {
			if port, ready := devToolsPort(profile, 15*time.Second); ready {
				return command, profile, port
			}
			stopBrowser(command, nil)
		} else if attempt == 1 {
			removeBrowserProfile(t, profile)
			t.Fatal(err)
		}
		removeBrowserProfile(t, profile)
	}
	t.Fatal("Chrome DevTools did not start after 2 attempts")
	return nil, "", 0
}

func stopBrowser(command *exec.Cmd, connection *websocket.Conn) {
	if connection != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = wsjson.Write(ctx, connection, map[string]any{"id": 5, "method": "Browser.close"})
		cancel()
	}
	exited := make(chan struct{})
	go func() { _ = command.Wait(); close(exited) }()
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		_ = command.Process.Kill()
		<-exited
	}
	if connection != nil {
		_ = connection.CloseNow()
	}
}

func removeBrowserProfile(t *testing.T, profile string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := os.RemoveAll(profile)
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("remove Chrome profile: %v", err)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func chromeExecutable(t *testing.T) string {
	t.Helper()
	for _, candidate := range []string{os.Getenv("CHROME_BIN"), "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "google-chrome", "chromium"} {
		if candidate == "" {
			continue
		}
		if strings.Contains(candidate, "/") {
			//nolint:gosec // G703: the optional test executable path is intentionally installation-owned.
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		} else if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	t.Skip("Chrome is not installed")
	return ""
}

func pageWebSocket(t *testing.T, port int, target string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		request, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/list", port), nil)
		response, err := http.DefaultClient.Do(request)
		if err == nil {
			var pages []struct {
				URL                  string `json:"url"`
				WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
			}
			err = json.NewDecoder(response.Body).Decode(&pages)
			_ = response.Body.Close()
			for _, page := range pages {
				if page.URL == target && err == nil {
					return page.WebSocketDebuggerURL
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("Chrome page target did not start")
	return ""
}

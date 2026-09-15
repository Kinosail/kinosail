package casting

import (
	"context"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func reply(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestDiscoveryPinsDescriptionAndControlToLocalResponder(t *testing.T) {
	source := netip.MustParseAddr("192.168.1.20")
	response := func(location string) string {
		return "HTTP/1.1 200 OK\r\nST: " + rendererType + "\r\nLOCATION: " + location + "\r\n\r\n"
	}
	for _, target := range []string{"http://127.0.0.1/", "http://169.254.169.254/", "http://8.8.8.8/", "http://192.168.1.21/", "http://tv.local/", "http://user@192.168.1.20/", "file:///etc/passwd", "http://192.168.1.20/#fragment"} {
		if ssdpLocation(response(target), source) != "" {
			t.Errorf("accepted %q", target)
		}
	}
	base, _ := url.Parse("http://192.168.1.20:1400/device.xml")
	if ssdpLocation(response(base.String()), source) != base.String() {
		t.Fatal("local response rejected")
	}
	for _, ref := range []string{"//192.168.1.21/control", "http://8.8.8.8/control", "http://192.168.1.20:80/control", "http://user@192.168.1.20:1400/control", "/control\r\nX: evil"} {
		if _, err := sameDeviceURL(base, ref); err == nil {
			t.Errorf("accepted control %q", ref)
		}
	}
	if result, err := sameDeviceURL(base, "/control"); err != nil || result != "http://192.168.1.20:1400/control" {
		t.Fatalf("control=%q %v", result, err)
	}
	if _, err := localDial(t.Context(), "tcp", "127.0.0.1:80"); err == nil {
		t.Fatal("loopback dial accepted")
	}
}

func TestLoadValidatesBeforeNetworkAndEscapesMetadata(t *testing.T) { //nolint:cyclop // Invalid requests and valid SOAP actions share one recorded network boundary.
	r := NewRenderers()
	device := Device{ID: strings.Repeat("a", 32), Name: "Living room", Protocol: "dlna", control: "http://192.168.1.20:1400/control", service: transportType, expires: time.Now().Add(time.Hour)}
	r.devices[device.ID] = device
	calls := []string{}
	r.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		calls = append(calls, request.Header.Get("SOAPAction")+" "+string(body))
		return reply(`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body/></s:Envelope>`), nil
	})}
	for _, test := range []struct {
		id, source, title string
		position          float64
	}{{"bad", "http://192.168.1.1/media", "Film", 0}, {device.ID, "file:///private/media", "Film", 0}, {device.ID, "http://user:secret@192.168.1.1/media", "Film", 0}, {device.ID, "http://192.168.1.1/media", "", 0}, {device.ID, "http://192.168.1.1/media", "Film", -1}} {
		if r.Load(t.Context(), test.id, test.source, "video/mp4", test.title, test.position) == nil {
			t.Fatal("invalid load succeeded")
		}
	}
	if len(calls) != 0 {
		t.Fatal("invalid load caused network effects")
	}
	if err := r.Load(t.Context(), device.ID, "http://192.168.1.1/cast/item/media?ticket=abc&x=1", "video/mp4", "Film <&>", 12); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 || !strings.Contains(calls[0], "#SetAVTransportURI") || !strings.Contains(calls[0], "&amp;x=1") || !strings.Contains(calls[1], "#Play") || !strings.Contains(calls[2], "00:00:12") {
		t.Fatalf("commands %#v", calls)
	}
	calls = nil
	number := 1.0
	for _, command := range []Command{{Action: "shutdown"}, {Action: "seek"}, {Action: "pause", Position: &number}} {
		if r.Command(t.Context(), device.ID, command) == nil {
			t.Fatal("invalid command accepted")
		}
	}
	if len(calls) != 0 {
		t.Fatal("invalid command caused a side effect")
	}
}

func TestRendererResponsesAreBoundedAndExpiredIDsCannotControl(t *testing.T) {
	r := NewRenderers()
	r.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return reply(strings.Repeat("x", 256*1024+1)), nil })}
	if _, err := r.describe(t.Context(), "http://192.168.1.20/description.xml"); err == nil {
		t.Fatal("oversized response accepted")
	}
	r.devices[strings.Repeat("a", 32)] = Device{ID: strings.Repeat("a", 32), expires: time.Now().Add(-time.Second)}
	if err := r.Command(t.Context(), strings.Repeat("a", 32), Command{Action: "stop"}); err == nil {
		t.Fatal("expired device accepted")
	}
	calls := 0
	r.discover = func(context.Context) ([]Device, error) { calls++; return []Device{}, nil }
	if _, err := r.Discover(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Discover(t.Context()); err == nil || calls != 1 {
		t.Fatal("scan throttle failed")
	}
}

func TestTransportTimesRejectMalformedAndOutOfRange(t *testing.T) {
	for _, value := range []string{"1", "01:99:00", "-1:00:00", "01:02:NaN", "01:02:Inf", "01:02:60", strings.Repeat("1", 17)} {
		if _, err := parseTime(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	if seconds, err := parseTime("01:02:03.5"); err != nil || seconds != 3723.5 {
		t.Fatalf("time=%v %v", seconds, err)
	}
}

func TestRendererPortsAreBoundedBeforeNetwork(t *testing.T) {
	r := NewRenderers()
	calls := 0
	r.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return reply("ok"), nil })}
	for _, port := range []string{"65536", "999999999999999999999", "-1", "+80", "http", "80%00"} {
		if _, err := r.request(t.Context(), http.MethodGet, "http://192.168.1.20:"+port+"/device.xml", "", ""); err == nil {
			t.Errorf("accepted port %q", port)
		}
	}
	if calls != 0 {
		t.Fatal("invalid renderer port reached the network")
	}
	for _, port := range []string{"0", "80", "65535", "00080"} {
		if _, err := r.request(t.Context(), http.MethodGet, "http://192.168.1.20:"+port+"/device.xml", "", ""); err != nil {
			t.Errorf("rejected numeric port %q: %v", port, err)
		}
	}
	if calls != 4 {
		t.Fatalf("valid requests = %d", calls)
	}
}

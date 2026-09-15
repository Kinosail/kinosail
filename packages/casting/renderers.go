package casting

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	transportType = "urn:schemas-upnp-org:service:AVTransport:1"
	rendererType  = "urn:schemas-upnp-org:device:MediaRenderer:1"
)

var ErrUnavailable = errors.New("TV is unavailable; check that it is on the same network as Kinosail Server")

type Device struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	control  string
	service  string
	expires  time.Time
}

// Renderers only controls endpoints learned from bounded local SSDP responses.
// Callers receive opaque IDs; they cannot choose an arbitrary destination URL.
type Renderers struct {
	mu       sync.Mutex
	devices  map[string]Device
	scanning bool
	lastScan time.Time
	client   *http.Client
	now      func() time.Time
	discover func(context.Context) ([]Device, error)
}

func NewRenderers() *Renderers {
	r := &Renderers{devices: make(map[string]Device), now: time.Now}
	r.client = &http.Client{Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("TV redirects are not allowed") }, Transport: &http.Transport{Proxy: nil, ResponseHeaderTimeout: 3 * time.Second, DisableKeepAlives: true, DialContext: localDial}}
	r.discover = r.scan
	return r
}

func localAddress(ip netip.Addr) bool {
	return ip.IsValid() && ip.Is4() && ip.IsPrivate() && !ip.IsLoopback() && !ip.IsUnspecified() && !ip.IsMulticast()
}

func localDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)
	ip, parseErr := netip.ParseAddr(host)
	if err != nil || parseErr != nil || !localAddress(ip) {
		return nil, ErrUnavailable
	}
	return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, network, address)
}

func (r *Renderers) Device(id string) (Device, bool) {
	if !ValidSessionID(id) {
		return Device{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	device, found := r.devices[id]
	return device, found && device.expires.After(r.now())
}

func (r *Renderers) Discover(ctx context.Context) ([]Device, error) {
	r.mu.Lock()
	if r.scanning || r.now().Sub(r.lastScan) < 10*time.Second {
		r.mu.Unlock()
		return nil, errors.New("Wait a few seconds before searching for TVs again") //nolint:staticcheck // This complete sentence is displayed in the TV picker.
	}
	r.scanning = true
	r.lastScan = r.now()
	r.mu.Unlock()
	devices, err := r.discover(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scanning = false
	if err != nil {
		return nil, ErrUnavailable
	}
	for id, device := range r.devices {
		if !device.expires.After(r.now()) {
			delete(r.devices, id)
		}
	}
	for _, device := range devices {
		if len(r.devices) >= 64 {
			break
		}
		r.devices[device.ID] = device
	}
	result := make([]Device, 0, len(devices))
	for _, device := range r.devices {
		result = append(result, device)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (r *Renderers) scan(ctx context.Context) ([]Device, error) {
	ctx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 0})
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	message := "M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 1\r\nST: " + rendererType + "\r\n\r\n"
	if _, err = conn.WriteToUDP([]byte(message), &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}); err != nil {
		return nil, err
	}
	locations := make(map[string]bool)
	buffer := make([]byte, 8193)
	for attempts := 0; attempts < 128; attempts++ {
		n, from, readErr := conn.ReadFromUDPAddrPort(buffer)
		if readErr != nil {
			break
		}
		if n > 8192 || !localAddress(from.Addr().Unmap()) {
			continue
		}
		location := ssdpLocation(string(buffer[:n]), from.Addr().Unmap())
		if location != "" && len(locations) < 32 {
			locations[location] = true
		}
	}
	return r.describeLocations(ctx, locations)
}

func (r *Renderers) describeLocations(ctx context.Context, locations map[string]bool) ([]Device, error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	devices := make([]Device, 0, len(locations))
	slots := make(chan struct{}, 4)
	for location := range locations {
		wg.Add(1)
		go func(location string) {
			defer wg.Done()
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-slots }()
			device, err := r.describe(ctx, location)
			if err == nil {
				mu.Lock()
				devices = append(devices, device)
				mu.Unlock()
			}
		}(location)
	}
	wg.Wait()
	if ctx.Err() != nil && len(devices) == 0 {
		return nil, ctx.Err()
	}
	return devices, nil
}

func ssdpLocation(text string, source netip.Addr) string {
	response, err := http.ReadResponse(bufio.NewReader(strings.NewReader(text)), nil)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || len(response.Header.Values("Location")) != 1 || response.Header.Get("ST") != rendererType {
		return ""
	}
	location := response.Header.Get("Location")
	parsed, err := url.Parse(location)
	if err != nil || !validLocalURL(parsed) || parsed.Hostname() != source.String() {
		return ""
	}
	return parsed.String()
}

func validLocalURL(value *url.URL) bool { //nolint:cyclop // This flat endpoint allowlist must run before every renderer network request.
	if value == nil || len(value.String()) > 2048 || value.Scheme != "http" || value.User != nil || value.Fragment != "" || value.Opaque != "" || strings.ContainsAny(value.String(), "\r\n") {
		return false
	}
	ip, err := netip.ParseAddr(value.Hostname())
	if err != nil || !localAddress(ip) {
		return false
	}
	if value.Port() != "" {
		_, err = strconv.ParseUint(value.Port(), 10, 16)
		if err != nil {
			return false
		}
	}
	return true
}

func sameDeviceURL(base *url.URL, ref string) (string, error) {
	if ref == "" || len(ref) > 2048 || strings.ContainsAny(ref, "\r\n") {
		return "", ErrUnavailable
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return "", ErrUnavailable
	}
	endpoint := base.ResolveReference(parsed)
	if !validLocalURL(endpoint) || endpoint.Host != base.Host {
		return "", ErrUnavailable
	}
	return endpoint.String(), nil
}

func (r *Renderers) request(ctx context.Context, method, target, action, body string) ([]byte, error) {
	parsed, err := url.Parse(target)
	if err != nil || !validLocalURL(parsed) {
		return nil, ErrUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, method, target, strings.NewReader(body))
	if err != nil {
		return nil, ErrUnavailable
	}
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "text/xml; charset=utf-8")
		request.Header.Set("SOAPAction", `"`+action+`"`)
	}
	response, err := r.client.Do(request)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 256*1024+1))
	if err != nil || len(data) > 256*1024 || response.StatusCode != http.StatusOK {
		return nil, ErrUnavailable
	}
	return data, nil
}

func rendererID(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:16])
}

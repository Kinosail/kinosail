package casting

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func rendererDescription(kind, name, services string) string {
	return `<root><device><deviceType>` + kind + `</deviceType><friendlyName>` + name + `</friendlyName><serviceList>` + services + `</serviceList></device></root>`
}

func rendererService(kind, control string) string {
	return `<service><serviceType>` + kind + `</serviceType><controlURL>` + control + `</controlURL></service>`
}

func TestRendererDescriptionRequiresSupportedSameDeviceControl(t *testing.T) {
	for _, version := range []string{transportType, "urn:schemas-upnp-org:service:AVTransport:2", "urn:schemas-upnp-org:service:AVTransport:3"} {
		t.Run(version, func(t *testing.T) {
			r := NewRenderers()
			now := time.Now()
			r.now = func() time.Time { return now }
			r.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodGet {
					t.Fatal("description attempted mutation")
				}
				return reply(rendererDescription(rendererType, " Living room ", rendererService("unsupported", "/ignored")+rendererService(version, "/control"))), nil
			})}
			device, err := r.describe(t.Context(), "http://192.168.1.20/device.xml")
			if err != nil || !ValidSessionID(device.ID) || device.Name != "Living room" || device.Protocol != "dlna" || device.control != "http://192.168.1.20/control" || device.service != version || !device.expires.Equal(now.Add(10*time.Minute)) {
				t.Fatalf("description=%+v err=%v", device, err)
			}
		})
	}
}

func TestRendererDescriptionsRejectMalformedAndUnsafeDevices(t *testing.T) {
	service := rendererService(transportType, "/control")
	for _, body := range []string{
		"<", rendererDescription("unknown", "TV", service), rendererDescription(rendererType, "", service),
		rendererDescription(rendererType, strings.Repeat("x", 129), service), rendererDescription(rendererType, "TV", strings.Repeat(service, 33)),
		rendererDescription(rendererType, "TV", rendererService("unsupported", "/control")),
		rendererDescription(rendererType, "TV", rendererService(transportType, "http://8.8.8.8/control")),
	} {
		r := NewRenderers()
		calls := 0
		r.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if request.URL.Hostname() != "192.168.1.20" {
				t.Fatal("followed unsafe control address")
			}
			return reply(body), nil
		})}
		if _, err := r.describe(t.Context(), "http://192.168.1.20/device.xml"); err == nil || calls != 1 {
			t.Fatalf("invalid description accepted; calls=%d", calls)
		}
	}
}

func TestDescribeLocationsOmitsFailedDevicesAndHonorsCancellation(t *testing.T) {
	r := NewRenderers()
	r.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/broken" {
			return reply("<"), nil
		}
		return reply(rendererDescription(rendererType, "TV", rendererService(transportType, "/control"))), nil
	})}
	devices, err := r.describeLocations(t.Context(), map[string]bool{"http://192.168.1.20/valid": true, "http://192.168.1.20/broken": true})
	if err != nil || len(devices) != 1 {
		t.Fatalf("devices=%v err=%v", devices, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if devices, err := r.describeLocations(ctx, nil); !errors.Is(err, context.Canceled) || len(devices) != 0 {
		t.Fatalf("cancelled scan=%v %v", devices, err)
	}
}

func TestDiscoveryPrunesExpiredDevicesAndBoundsResults(t *testing.T) {
	r := NewRenderers()
	now := time.Now()
	r.now = func() time.Time { return now }
	r.devices[strings.Repeat("a", 32)] = Device{expires: now.Add(-time.Second)}
	r.discover = func(context.Context) ([]Device, error) {
		devices := make([]Device, 65)
		for index := range devices {
			id := rendererID(string(rune(index)))
			devices[index] = Device{ID: id, Name: string(rune(100 - index)), expires: now.Add(time.Minute)}
		}
		return devices, nil
	}
	devices, err := r.Discover(t.Context())
	if err != nil || len(devices) != 64 {
		t.Fatalf("scan=%d %v", len(devices), err)
	}
	for index := 1; index < len(devices); index++ {
		if devices[index-1].Name > devices[index].Name {
			t.Fatal("devices not sorted")
		}
	}
	now = now.Add(time.Minute)
	r.discover = func(context.Context) ([]Device, error) { return nil, errors.New("unavailable") }
	if _, err := r.Discover(t.Context()); !errors.Is(err, ErrUnavailable) || r.scanning {
		t.Fatalf("failed scan=%v scanning=%v", err, r.scanning)
	}
}

// Package dlna serves Player's local-network media discovery protocol.
package dlna

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const contentDirectory = "urn:schemas-upnp-org:service:ContentDirectory:1"

// Settings returns the current DLNA capability and display name.
type Settings interface {
	Token() string
	ServerName() string
}

// SettingsFuncs adapts app-owned settings functions.
type SettingsFuncs struct {
	TokenFunc      func() string
	ServerNameFunc func() string
}

func (adapter SettingsFuncs) Token() string      { return adapter.TokenFunc() }
func (adapter SettingsFuncs) ServerName() string { return adapter.ServerNameFunc() }

// Library provides safe, indexed media access.
type Library interface {
	Snapshot() ([]library.Item, error)
	Find(string) (library.Item, bool)
	Safe(string) bool
}

// LibraryFuncs adapts app-owned library functions.
type LibraryFuncs struct {
	SnapshotFunc func() ([]library.Item, error)
	FindFunc     func(string) (library.Item, bool)
	SafeFunc     func(string) bool
}

func (adapter LibraryFuncs) Snapshot() ([]library.Item, error) { return adapter.SnapshotFunc() }
func (adapter LibraryFuncs) Find(id string) (library.Item, bool) {
	return adapter.FindFunc(id)
}
func (adapter LibraryFuncs) Safe(path string) bool { return adapter.SafeFunc(path) }

// Config connects shared DLNA behavior to app-owned state and presentation.
type Config struct {
	Base     string
	Settings Settings
	Library  Library
	NotFound func(http.ResponseWriter, *http.Request)
	discover func(context.Context, string, Settings)
}

// Register adds Player's DLNA routes and local discovery listener.
func Register(mux *http.ServeMux, ctx context.Context, config Config) error {
	if mux == nil || ctx == nil || config.Settings == nil || config.Library == nil || config.NotFound == nil {
		return errors.New("invalid DLNA configuration")
	}
	handler := server{config: config}
	mux.HandleFunc("GET /dlna/{token}/device.xml", handler.device)
	mux.HandleFunc("GET /dlna/{token}/content.xml", handler.service)
	mux.HandleFunc("POST /dlna/{token}/content", handler.browse)
	mux.HandleFunc("GET /dlna/{token}/media/{id}", handler.media)
	mux.HandleFunc("HEAD /dlna/{token}/media/{id}", handler.media)
	if ValidURL(config.Base) {
		discover := config.discover
		if discover == nil {
			discover = serveSSDP
		}
		go discover(ctx, config.Base, config.Settings)
	}
	return nil
}

type server struct{ config Config }

// ValidURL reports whether a deployment value is a safe absolute HTTP URL.
func ValidURL(value string) bool {
	if value == "" || len(value) > 2048 || strings.ContainsAny(value, "\r\n\x00") {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func (server server) authorized(request *http.Request) bool {
	token := server.config.Settings.Token()
	provided := request.PathValue("token")
	return token != "" && len(token) <= 256 && len(provided) <= 256 && subtle.ConstantTimeCompare([]byte(token), []byte(provided)) == 1
}

func (server server) device(writer http.ResponseWriter, request *http.Request) {
	if !server.authorized(request) {
		server.config.NotFound(writer, request)
		return
	}
	token, id := server.config.Settings.Token(), identifier(server.config.Base)
	writer.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = fmt.Fprintf(writer, `<?xml version="1.0"?><root xmlns="urn:schemas-upnp-org:device-1-0"><specVersion><major>1</major><minor>0</minor></specVersion><device><deviceType>urn:schemas-upnp-org:device:MediaServer:1</deviceType><friendlyName>%s</friendlyName><manufacturer>Kinosail</manufacturer><modelName>Kinosail Server</modelName><UDN>uuid:%s</UDN><serviceList><service><serviceType>%s</serviceType><serviceId>urn:upnp-org:serviceId:ContentDirectory</serviceId><SCPDURL>/dlna/%s/content.xml</SCPDURL><controlURL>/dlna/%s/content</controlURL><eventSubURL>/dlna/%s/events</eventSubURL></service></serviceList></device></root>`, html.EscapeString(server.config.Settings.ServerName()), id, contentDirectory, token, token, token)
}

func (server server) service(writer http.ResponseWriter, request *http.Request) {
	if !server.authorized(request) {
		server.config.NotFound(writer, request)
		return
	}
	writer.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = writer.Write([]byte(`<?xml version="1.0"?><scpd xmlns="urn:schemas-upnp-org:service-1-0"><specVersion><major>1</major><minor>0</minor></specVersion><actionList><action><name>Browse</name></action></actionList><serviceStateTable><stateVariable sendEvents="no"><name>SystemUpdateID</name><dataType>ui4</dataType></stateVariable></serviceStateTable></scpd>`))
}

func (server server) browse(writer http.ResponseWriter, request *http.Request) {
	action := request.Header.Get("SOAPAction")
	if !server.authorized(request) || len(action) > 256 || !strings.Contains(action, contentDirectory+"#Browse") {
		server.config.NotFound(writer, request)
		return
	}
	items, _ := server.config.Library.Snapshot()
	items = safeItems(server.config.Library, items)
	capability := server.config.Settings.Token()
	var didl strings.Builder
	didl.WriteString(`<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/">`)
	for _, item := range items {
		_, _ = fmt.Fprintf(&didl, `<item id="%s" parentID="0" restricted="1"><dc:title>%s</dc:title><upnp:class>%s</upnp:class><res protocolInfo="http-get:*:%s:*">%s/dlna/%s/media/%s</res></item>`, item.ID, html.EscapeString(item.Title), Class(item), MediaType(item), strings.TrimRight(server.config.Base, "/"), capability, item.ID)
	}
	didl.WriteString(`</DIDL-Lite>`)
	writer.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	_, _ = fmt.Fprintf(writer, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><u:BrowseResponse xmlns:u="%s"><Result>%s</Result><NumberReturned>%d</NumberReturned><TotalMatches>%d</TotalMatches><UpdateID>1</UpdateID></u:BrowseResponse></s:Body></s:Envelope>`, contentDirectory, html.EscapeString(didl.String()), len(items), len(items))
}

func (server server) media(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	item, found := server.config.Library.Find(id)
	if !server.authorized(request) || id == "" || len(id) > 512 || !found || !server.config.Library.Safe(item.Path) {
		server.config.NotFound(writer, request)
		return
	}
	writer.Header().Set("Content-Type", MediaType(item))
	http.ServeFile(writer, request, item.Path)
}

// Class returns the UPnP class for one library item.
func Class(item library.Item) string {
	if item.Kind == "audio" {
		return "object.item.audioItem.musicTrack"
	}
	if item.Kind == "photo" {
		return "object.item.imageItem.photo"
	}
	return "object.item.videoItem.movie"
}

func safeItems(index Library, items []library.Item) []library.Item {
	result := make([]library.Item, 0, len(items))
	for _, item := range items {
		if index.Safe(item.Path) && (item.Kind == "video" || item.Kind == "audio" || item.Kind == "photo") {
			result = append(result, item)
		}
	}
	return result
}

// MediaType maps Player's scanned formats to DLNA response types.
func MediaType(item library.Item) string {
	if mime := map[string]string{
		"3G2": "video/3gpp2", "3GP": "video/3gpp", "ASF": "video/x-ms-asf", "AVI": "video/x-msvideo", "FLV": "video/x-flv", "M2TS": "video/mp2t", "M4V": "video/mp4", "MKV": "video/x-matroska", "MOV": "video/quicktime", "MP4": "video/mp4", "MPEG": "video/mpeg", "MPG": "video/mpeg", "MTS": "video/mp2t", "OGV": "video/ogg", "TS": "video/mp2t", "VOB": "video/mpeg", "WEBM": "video/webm", "WMV": "video/x-ms-wmv",
		"AAC": "audio/aac", "AIF": "audio/aiff", "AIFF": "audio/aiff", "FLAC": "audio/flac", "M4A": "audio/mp4", "M4B": "audio/mp4", "MP3": "audio/mpeg", "OGA": "audio/ogg", "OGG": "audio/ogg", "OPUS": "audio/ogg", "WAV": "audio/wav", "WEBA": "audio/webm",
		"AVIF": "image/avif", "BMP": "image/bmp", "GIF": "image/gif", "JPEG": "image/jpeg", "JPG": "image/jpeg", "PNG": "image/png", "WEBP": "image/webp",
	}[item.Container]; mime != "" {
		return mime
	}
	return "application/octet-stream"
}

func serveSSDP(ctx context.Context, base string, settings Settings) {
	serveSSDPWith(ctx, base, settings, func(network string, address *net.UDPAddr) (ssdpConnection, error) {
		return net.ListenMulticastUDP(network, nil, address)
	})
}

func serveSSDPWith(ctx context.Context, base string, settings Settings, listen func(string, *net.UDPAddr) (ssdpConnection, error)) {
	address := &net.UDPAddr{IP: net.ParseIP("239.255.255.250"), Port: 1900}
	connection, err := listen("udp4", address)
	if err != nil {
		return
	}
	defer connection.Close()
	serveSSDPConnection(ctx, base, settings, connection)
}

type ssdpConnection interface {
	Close() error
	SetReadDeadline(time.Time) error
	ReadFromUDP([]byte) (int, *net.UDPAddr, error)
	WriteToUDP([]byte, *net.UDPAddr) (int, error)
}

func serveSSDPConnection(ctx context.Context, base string, settings Settings, connection ssdpConnection) {
	buffer := make([]byte, 2048)
	for ctx.Err() == nil {
		_ = connection.SetReadDeadline(time.Now().Add(time.Second))
		length, sender, err := connection.ReadFromUDP(buffer)
		search := strings.ToUpper(string(buffer[:length]))
		recognized := strings.Contains(search, "ST: SSDP:ALL") || strings.Contains(search, "ST: URN:SCHEMAS-UPNP-ORG:DEVICE:MEDIASERVER:")
		if token := settings.Token(); err == nil && strings.Contains(search, "M-SEARCH") && recognized && token != "" && len(token) <= 256 {
			_, _ = connection.WriteToUDP([]byte(SSDPResponse(base, token)), sender)
		}
	}
}

// SSDPResponse returns one local-network discovery reply.
func SSDPResponse(base, token string) string {
	return "HTTP/1.1 200 OK\r\nCACHE-CONTROL: max-age=1800\r\nEXT:\r\nLOCATION: " + strings.TrimRight(base, "/") + "/dlna/" + token + "/device.xml\r\nSERVER: Kinosail/1 UPnP/1.0\r\nST: urn:schemas-upnp-org:device:MediaServer:1\r\nUSN: uuid:" + identifier(base) + "::urn:schemas-upnp-org:device:MediaServer:1\r\n\r\n"
}

func identifier(base string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(base)))[:32]
}

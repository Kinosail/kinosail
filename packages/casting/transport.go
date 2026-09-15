package casting

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type description struct {
	Device struct {
		Type     string `xml:"deviceType"`
		Name     string `xml:"friendlyName"`
		Services []struct {
			Type    string `xml:"serviceType"`
			Control string `xml:"controlURL"`
		} `xml:"serviceList>service"`
	} `xml:"device"`
}

func (r *Renderers) describe(ctx context.Context, location string) (Device, error) {
	body, err := r.request(ctx, http.MethodGet, location, "", "")
	if err != nil {
		return Device{}, err
	}
	var description description
	if xml.Unmarshal(body, &description) != nil || description.Device.Type != rendererType || !validLabel(description.Device.Name, 128) || len(description.Device.Services) > 32 {
		return Device{}, ErrUnavailable
	}
	base, _ := url.Parse(location)
	for _, service := range description.Device.Services {
		if !supportedTransport(service.Type) {
			continue
		}
		control, err := sameDeviceURL(base, service.Control)
		if err != nil {
			return Device{}, err
		}
		return Device{ID: rendererID(control), Name: strings.TrimSpace(description.Device.Name), Protocol: "dlna", control: control, service: service.Type, expires: r.now().Add(10 * time.Minute)}, nil
	}
	return Device{}, ErrUnavailable
}

func validLabel(value string, maximum int) bool {
	if strings.TrimSpace(value) == "" || len(value) > maximum {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsControl)
}

func escape(value string) string {
	var out bytes.Buffer
	_ = xml.EscapeText(&out, []byte(value))
	return out.String()
}

func (r *Renderers) action(ctx context.Context, device Device, name, args string) ([]byte, error) {
	body := `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><u:` + name + ` xmlns:u="` + device.service + `"><InstanceID>0</InstanceID>` + args + `</u:` + name + `></s:Body></s:Envelope>`
	return r.request(ctx, http.MethodPost, device.control, device.service+"#"+name, body)
}

func (r *Renderers) Load(ctx context.Context, id, source, mime, title string, position float64) error {
	device, ok := r.Device(id)
	if !ok || !validMediaURL(source) || !validMediaMetadata(title, mime) || !Position(position) {
		return ErrInvalid
	}
	metadata := `<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"><item id="0" parentID="0" restricted="1"><dc:title>` + escape(title) + `</dc:title><upnp:class>`
	class := "object.item.videoItem"
	if strings.HasPrefix(mime, "audio/") {
		class = "object.item.audioItem.musicTrack"
	}
	metadata += class + `</upnp:class><res protocolInfo="http-get:*:` + escape(mime) + `:*">` + escape(source) + `</res></item></DIDL-Lite>`
	if _, err := r.action(ctx, device, "SetAVTransportURI", "<CurrentURI>"+escape(source)+"</CurrentURI><CurrentURIMetaData>"+escape(metadata)+"</CurrentURIMetaData>"); err != nil {
		return err
	}
	if _, err := r.action(ctx, device, "Play", "<Speed>1</Speed>"); err != nil {
		return err
	}
	if position > 0 {
		if _, err := r.action(ctx, device, "Seek", "<Unit>REL_TIME</Unit><Target>"+transportTime(position)+"</Target>"); err != nil {
			_, _ = r.action(ctx, device, "Stop", "")
			return errors.New("This TV could not resume the title; start from the beginning or choose another casting option") //nolint:staticcheck // This complete sentence is displayed in the TV picker.
		}
	}
	return nil
}

func transportTime(position float64) string {
	seconds := int(position)
	return fmt.Sprintf("%02d:%02d:%02d", seconds/3600, (seconds/60)%60, seconds%60)
}

func (r *Renderers) Command(ctx context.Context, id string, command Command) error {
	if command.Validate() != nil {
		return ErrInvalid
	}
	device, ok := r.Device(id)
	if !ok {
		return ErrUnavailable
	}
	name, args := "", ""
	switch command.Action {
	case "play":
		name, args = "Play", "<Speed>1</Speed>"
	case "pause":
		name = "Pause"
	case "stop":
		name = "Stop"
	case "seek":
		name, args = "Seek", "<Unit>REL_TIME</Unit><Target>"+transportTime(*command.Position)+"</Target>"
	}
	_, err := r.action(ctx, device, name, args)
	return err
}

type Status struct {
	Source   string  `json:"-"`
	State    string  `json:"state"`
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
}

func (r *Renderers) Status(ctx context.Context, id string) (Status, error) {
	device, ok := r.Device(id)
	if !ok {
		return Status{}, ErrUnavailable
	}
	transport, err := r.action(ctx, device, "GetTransportInfo", "")
	if err != nil {
		return Status{}, err
	}
	var state struct {
		State string `xml:"Body>GetTransportInfoResponse>CurrentTransportState"`
	}
	if xml.Unmarshal(transport, &state) != nil {
		return Status{}, ErrUnavailable
	}
	value := Status{}
	value.State = transportState(state.State)
	if value.State == "" {
		return Status{}, ErrUnavailable
	}
	info, err := r.action(ctx, device, "GetPositionInfo", "")
	if err != nil {
		return Status{}, err
	}
	var position struct {
		URI      string `xml:"Body>GetPositionInfoResponse>TrackURI"`
		Position string `xml:"Body>GetPositionInfoResponse>RelTime"`
		Duration string `xml:"Body>GetPositionInfoResponse>TrackDuration"`
	}
	if xml.Unmarshal(info, &position) != nil {
		return Status{}, ErrUnavailable
	}
	if parsed, parseErr := url.Parse(position.URI); parseErr == nil {
		value.Source = parsed.Scheme + "://" + parsed.Host + parsed.EscapedPath()
	}
	value.Position, err = parseTime(position.Position)
	if err != nil {
		return Status{}, err
	}
	value.Duration, err = parseTime(position.Duration)
	if err != nil {
		return Status{}, err
	}
	// Successful polling keeps an actively controlled endpoint available without rescanning.
	r.mu.Lock()
	device.expires = r.now().Add(10 * time.Minute)
	r.devices[id] = device
	r.mu.Unlock()
	return value, nil
}

func parseTime(value string) (float64, error) {
	if value == "NOT_IMPLEMENTED" || value == "" {
		return 0, nil
	}
	if len(value) > 16 {
		return 0, ErrInvalid
	}
	parts := strings.Split(value, ":")
	if len(parts) != 3 || len(parts[1]) != 2 {
		return 0, ErrInvalid
	}
	hours, e1 := clockUnit(parts[0], 0)
	minutes, e2 := clockUnit(parts[1], 60)
	seconds, e3 := clockSeconds(parts[2])
	result := float64(hours*3600+minutes*60) + seconds
	if e1 != nil || e2 != nil || e3 != nil || !Position(result) {
		return 0, ErrInvalid
	}
	return result, nil
}

func supportedTransport(kind string) bool {
	switch kind {
	case transportType, "urn:schemas-upnp-org:service:AVTransport:2", "urn:schemas-upnp-org:service:AVTransport:3":
		return true
	}
	return false
}

func validMediaURL(source string) bool {
	parsed, err := url.Parse(source)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil && parsed.Fragment == "" && len(source) <= 4096
}

func transportState(state string) string {
	switch state {
	case "PLAYING":
		return "playing"
	case "PAUSED_PLAYBACK":
		return "paused"
	case "STOPPED", "NO_MEDIA_PRESENT":
		return "stopped"
	case "TRANSITIONING":
		return "buffering"
	default:
		return ""
	}
}

func clockUnit(raw string, upper int) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 || upper > 0 && value >= upper {
		return 0, ErrInvalid
	}
	return value, nil
}

func validMediaMetadata(title, mime string) bool {
	return validLabel(title, 1024) && validLabel(mime, 128) && !strings.ContainsAny(mime, "<>\"'")
}

func clockSeconds(raw string) (float64, error) {
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil || seconds < 0 || seconds >= 60 {
		return 0, ErrInvalid
	}
	return seconds, nil
}

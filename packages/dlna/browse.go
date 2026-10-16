package dlna

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"html"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	maximumBrowseBody = 64 << 10
	maximumObjectID   = 256
	didlOpen          = `<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/">`
)

type browseRequest struct {
	ObjectID       string `xml:"ObjectID"`
	BrowseFlag     string `xml:"BrowseFlag"`
	StartingIndex  string `xml:"StartingIndex"`
	RequestedCount string `xml:"RequestedCount"`
	start, count   int
}

type browseError struct {
	code    int
	message string
}

var (
	errInvalidArgs     = browseError{402, "Invalid Args"}
	errNoSuchObject    = browseError{701, "No such object"}
	errNoSuchContainer = browseError{710, "No such container"}
)

func (err browseError) Error() string { return err.message }

func readBrowseRequest(request *http.Request, writer http.ResponseWriter) (browseRequest, error) {
	arguments := browseRequest{ObjectID: "0", BrowseFlag: "BrowseDirectChildren"}
	body, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, maximumBrowseBody))
	if err != nil {
		return arguments, errInvalidArgs
	}
	decoder := xml.NewDecoder(bytes.NewReader(body))
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return arguments, errInvalidArgs
		}
		if element, ok := token.(xml.StartElement); ok && element.Name.Local == "Browse" {
			if decoder.DecodeElement(&arguments, &element) != nil {
				return arguments, errInvalidArgs
			}
			break
		}
	}
	var startErr, countErr error
	arguments.start, startErr = browseNumber(arguments.StartingIndex)
	arguments.count, countErr = browseNumber(arguments.RequestedCount)
	if startErr != nil || countErr != nil || arguments.ObjectID == "" || len(arguments.ObjectID) > maximumObjectID || arguments.BrowseFlag != "BrowseDirectChildren" && arguments.BrowseFlag != "BrowseMetadata" {
		return arguments, errInvalidArgs
	}
	return arguments, nil
}

func browseNumber(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	number, err := strconv.ParseUint(value, 10, 31)
	return int(number), err
}

func playable(view *library.View) []*library.Item {
	return library.Derive(view, "dlna.playable", func(view *library.View) []*library.Item {
		items := view.Items()
		result := make([]*library.Item, 0, len(items))
		for position := range items {
			if playableKind(items[position]) {
				result = append(result, &items[position])
			}
		}
		return result
	})
}

func playableKind(item library.Item) bool {
	return item.Kind == "video" || item.Kind == "audio" || item.Kind == "photo"
}

func (server server) browse(writer http.ResponseWriter, request *http.Request) {
	action := request.Header.Get("SOAPAction")
	if !server.authorized(request) || len(action) > 256 || !strings.Contains(action, contentDirectory+"#Browse") {
		server.config.NotFound(writer, request)
		return
	}
	arguments, err := readBrowseRequest(request, writer)
	snapshot, _ := server.config.Library.Library()
	items := playable(snapshot.View(library.NewPolicy(true, "", nil)))
	var page []*library.Item
	total := 0
	if err == nil {
		page, total, err = browsePage(snapshot, items, arguments)
	}
	var failure browseError
	if errors.As(err, &failure) {
		slog.WarnContext(request.Context(), "DLNA browse rejected", "operation", "dlna.browse", "failure_class", strings.ReplaceAll(strings.ToLower(failure.message), " ", "_"), "upnp_error", failure.code)
		writeBrowseFault(writer, failure)
		return
	}
	writer.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	output := bufio.NewWriter(writer)
	_, _ = output.WriteString(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><u:BrowseResponse xmlns:u="` + contentDirectory + `"><Result>`)
	_, _ = output.WriteString(html.EscapeString(didlOpen))
	if arguments.ObjectID == "0" && arguments.BrowseFlag == "BrowseMetadata" {
		_, _ = output.WriteString(html.EscapeString(`<container id="0" parentID="-1" restricted="1" childCount="` + strconv.Itoa(len(items)) + `"><dc:title>`))
		_, _ = output.WriteString(html.EscapeString(html.EscapeString(server.config.Settings.ServerName())))
		_, _ = output.WriteString(html.EscapeString(`</dc:title><upnp:class>object.container.storageFolder</upnp:class></container>`))
	}
	capability, base := server.config.Settings.Token(), strings.TrimRight(server.config.Base, "/")
	for _, item := range page {
		_, _ = output.WriteString(html.EscapeString(`<item id="` + item.ID + `" parentID="0" restricted="1"><dc:title>`))
		_, _ = output.WriteString(html.EscapeString(html.EscapeString(item.Title)))
		_, _ = output.WriteString(html.EscapeString(`</dc:title><upnp:class>` + Class(*item) + `</upnp:class><res protocolInfo="http-get:*:` + MediaType(*item) + `:*">` + base + `/dlna/` + capability + `/media/` + item.ID + `</res></item>`))
	}
	_, _ = output.WriteString(html.EscapeString(`</DIDL-Lite>`) + `</Result><NumberReturned>` + strconv.Itoa(max(len(page), returnedContainer(arguments))) + `</NumberReturned><TotalMatches>` + strconv.Itoa(total) + `</TotalMatches><UpdateID>1</UpdateID></u:BrowseResponse></s:Body></s:Envelope>`)
	_ = output.Flush()
}

func returnedContainer(arguments browseRequest) int {
	if arguments.ObjectID == "0" && arguments.BrowseFlag == "BrowseMetadata" {
		return 1
	}
	return 0
}

func browsePage(snapshot *library.Snapshot, items []*library.Item, arguments browseRequest) ([]*library.Item, int, error) {
	if arguments.ObjectID == "0" {
		if arguments.BrowseFlag == "BrowseMetadata" {
			return nil, 1, nil
		}
		start := min(arguments.start, len(items))
		end := len(items)
		if arguments.count > 0 {
			end = min(end, start+arguments.count)
		}
		return items[start:end], len(items), nil
	}
	item, found := snapshot.Find(arguments.ObjectID)
	if !found || !playableKind(item) {
		return nil, 0, errNoSuchObject
	}
	if arguments.BrowseFlag == "BrowseDirectChildren" {
		return nil, 0, errNoSuchContainer
	}
	return []*library.Item{&item}, 1, nil
}

func writeBrowseFault(writer http.ResponseWriter, failure browseError) {
	writer.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	writer.WriteHeader(http.StatusInternalServerError)
	_, _ = io.WriteString(writer, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><s:Fault><faultcode>s:Client</faultcode><faultstring>UPnPError</faultstring><detail><UPnPError xmlns="urn:schemas-upnp-org:control-1-0"><errorCode>`+strconv.Itoa(failure.code)+`</errorCode><errorDescription>`+failure.message+`</errorDescription></UPnPError></detail></s:Fault></s:Body></s:Envelope>`)
}

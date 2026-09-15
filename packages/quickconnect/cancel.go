package quickconnect

import (
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/apihttp"
	"github.com/MikeO7/kinosail/packages/httpguard"
)

// Cancel removes only the request whose device-held polling secret matches.
// Missing or already consumed requests are harmless, so cancellation is idempotent.
func (broker *Broker) Cancel(secret string) error {
	if secret == "" || strings.TrimSpace(secret) != secret || !validField(secret, 128) {
		return ErrInvalidRequest
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	key := secretKey(secret)
	if pending, found := broker.pending[key]; found {
		broker.remove(key, pending.connection.Code)
	}
	return nil
}

// CancelAPI lets a requesting device withdraw its own pending authorization.
func (application *Application) CancelAPI(writer http.ResponseWriter, request *http.Request) {
	if !application.limiters.Requests.Allow("cancel:"+httpguard.RemoteHost(request.RemoteAddr), 60) {
		tooMany(writer, "too many Quick Connect requests")
		return
	}
	secret, ok := readPollSecret(writer, request)
	if !ok {
		return
	}
	if err := application.broker.Cancel(secret); err != nil {
		apihttp.Error(writer, err, http.StatusBadRequest)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

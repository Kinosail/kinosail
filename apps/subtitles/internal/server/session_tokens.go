package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

func sessionToken(request *http.Request) string { return identitycore.SessionToken(request, nil) }

func sessionKey(token string) string { return identitycore.SessionKey(token) }

func cleanDeviceName(name string) string { return identitycore.CleanDeviceName(name) }

package server_test

import (
	"net/http"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

type apiTestCall = servertest.APITestCall

var (
	apiServer = (servertest.APIFixture{
		NewHandler: func(media, data string) http.Handler {
			return server.New(server.Config{MediaDir: media, DataDir: data, RequireAuth: true})
		},
		TOTP: testTOTP,
	}).Server
	assertAPICalls = servertest.AssertAPICalls
	assertAPIBody  = servertest.AssertAPIBody
	apiCall        = servertest.APICall
	webFormCall    = servertest.WebFormCall
	mustJSON       = servertest.MustJSON
)

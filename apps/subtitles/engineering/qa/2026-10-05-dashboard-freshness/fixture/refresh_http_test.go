package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

const (
	r16ResponseLimit = 65536
	r16UserAgent     = "R16-private-control"
)

func (target *r16Target) request(ctx context.Context, method, route string, body []byte, authority r16Authority) r16Response {
	target.track(1)
	defer target.track(-1)
	owned, cancel := context.WithCancel(ctx)
	defer cancel()
	unlink := target.joinLifecycle(cancel)
	defer unlink()
	endpoint, err := target.endpoint(method, route)
	if err != nil || len(body) > r16ResponseLimit || target.client == nil {
		target.fail()
		return r16Response{}
	}
	request, err := http.NewRequestWithContext(owned, method, endpoint, bytes.NewReader(body))
	if err != nil {
		target.fail()
		return r16Response{}
	}
	target.headers(request, authority)
	if !target.admitted(request) {
		target.fail()
		return r16Response{}
	}
	client := target.clientFor(authority)
	response, err := client.Do(request)
	if err != nil {
		_ = r16CloseFailedResponse(response)
		target.fail()
		return r16Response{}
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, r16ResponseLimit+1))
	closeErr := response.Body.Close()
	complete := readErr == nil && closeErr == nil && len(data) <= r16ResponseLimit
	if !complete {
		target.fail()
		return r16Response{Status: response.StatusCode}
	}
	return r16Response{Status: response.StatusCode, Body: data, Complete: true}
}

func (target *r16Target) clientFor(authority r16Authority) *http.Client {
	if authority != r16Anonymous {
		return target.client
	}
	client := *target.client
	client.Jar = nil
	return &client
}

func (target *r16Target) headers(request *http.Request, authority r16Authority) {
	request.Header.Set("User-Agent", r16UserAgent)
	request.Header.Set("Origin", target.origin)
	request.Header.Set("Accept", "application/json")
	if request.Method == http.MethodPost {
		request.Header.Set("Content-Type", "application/json")
	}
	if request.Method != http.MethodPost && request.Method != http.MethodDelete {
		return
	}
	target.mu.Lock()
	csrf := target.csrf
	target.mu.Unlock()
	switch authority {
	case r16Owner:
		request.Header.Set("X-Kinosail-CSRF", csrf)
	case r16Anonymous, r16MissingCSRF:
		return
	case r16WrongCSRF:
		if len(csrf) != 43 {
			target.fail()
			return
		}
		first := "A"
		if csrf[0] == 'A' {
			first = "B"
		}
		request.Header.Set("X-Kinosail-CSRF", first+csrf[1:])
	}
}

func (target *r16Target) joinLifecycle(cancel context.CancelFunc) func() bool {
	return context.AfterFunc(target.lifecycle, cancel)
}

func r16CloseFailedResponse(response *http.Response) error {
	if response == nil {
		return nil
	}
	return response.Body.Close()
}

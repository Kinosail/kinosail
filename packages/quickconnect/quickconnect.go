// Package quickconnect coordinates one-time device authorization.
package quickconnect

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"math/big"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	defaultTTL = 5 * time.Minute
	maxPending = 1024
)

var (
	ErrCapacity        = errors.New("too many Quick Connect requests")                       //nolint:staticcheck // Product term starts with a capital.
	ErrPending         = errors.New("Quick Connect is pending")                              //nolint:staticcheck // Product term starts with a capital.
	ErrNotFound        = errors.New("Quick Connect request was not found")                   //nolint:staticcheck // Product term starts with a capital.
	ErrInvalidCode     = errors.New("Quick Connect code is invalid or expired")              //nolint:staticcheck // Product term starts with a capital.
	ErrAlreadyApproved = errors.New("Quick Connect code is already approved")                //nolint:staticcheck // Product term starts with a capital.
	ErrRemoteViewer    = errors.New("public Quick Connect requires a remote-enabled Viewer") //nolint:staticcheck // Product terms start with capitals.
	ErrInvalidRequest  = errors.New("Quick Connect request is invalid")                      //nolint:staticcheck // Product term starts with a capital.
)

// Request describes the device that needs authorization.
type Request struct {
	Device, DeviceID, Client, Version string
	Numeric, Remote                   bool
}

// Connection is the current state exposed to protocol adapters.
type Connection struct {
	Code, Device, DeviceID, Client, Version string
	Created                                 time.Time
	Approved                                bool
}

// Viewer contains the authorization facts needed for approval.
type Viewer struct {
	ID               string
	Revision         uint64
	Owner            bool
	Remote           bool
	Secured          bool
	StronglyVerified bool
}

// Grant identifies the approved Viewer and session to create.
type Grant struct {
	ProfileID, Device string
	ProfileRevision   uint64
	Remote            bool
	Owner             bool
}

// Broker owns pending Quick Connect state.
type Broker struct {
	mu      sync.Mutex
	byCode  map[string]string
	pending map[string]pendingConnection
	ttl     time.Duration
	now     func() time.Time
	code    func(bool) (string, error)
}

type pendingConnection struct {
	connection      Connection
	expires         time.Time
	profileID       string
	profileRevision uint64
	remote          bool
	owner           bool
}

// New returns an empty Broker.
func New(ttl time.Duration) *Broker {
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &Broker{
		byCode: make(map[string]string), pending: make(map[string]pendingConnection), ttl: ttl, now: time.Now,
		code: func(numeric bool) (string, error) { return connectCode(numeric, rand.Reader) },
	}
}

// Create starts one bounded authorization request.
func (broker *Broker) Create(request Request) (string, Connection, error) {
	if !validRequest(request) {
		return "", Connection{}, ErrInvalidRequest
	}
	secret, now := "qc_"+rand.Text(), broker.now()
	broker.mu.Lock()
	defer broker.mu.Unlock()
	broker.prune(now)
	if len(broker.pending) >= maxPending {
		return "", Connection{}, ErrCapacity
	}
	code, err := broker.code(request.Numeric)
	for err == nil && broker.byCode[code] != "" {
		code, err = broker.code(request.Numeric)
	}
	if err != nil {
		return "", Connection{}, err
	}
	connection := Connection{
		Code: code, Device: request.Device, DeviceID: request.DeviceID, Client: request.Client,
		Version: request.Version, Created: now,
	}
	key := secretKey(secret)
	broker.pending[key], broker.byCode[code] = pendingConnection{connection: connection, expires: now.Add(broker.ttl), remote: request.Remote}, key
	return secret, connection, nil
}

// Status returns an unexpired authorization request.
func (broker *Broker) Status(secret string) (Connection, bool) {
	if !validField(secret, 128) {
		return Connection{}, false
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	broker.prune(broker.now())
	pending, found := broker.pending[secretKey(secret)]
	if !found {
		return Connection{}, false
	}
	connection := pending.connection
	connection.Approved = pending.profileID != ""
	return connection, true
}

// Approve binds one request to a Viewer.
func (broker *Broker) Approve(value string, viewer Viewer) error {
	code, err := approvalCode(value, viewer)
	if err != nil {
		return err
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	key := broker.byCode[code]
	pending, found := broker.pending[key]
	if !found || !pending.expires.After(broker.now()) {
		broker.remove(key, code)
		return ErrInvalidCode
	}
	if err := approvalAllowed(pending, viewer); err != nil {
		return err
	}
	pending.profileID, pending.profileRevision, pending.owner = viewer.ID, viewer.Revision, viewer.Owner
	broker.pending[key] = pending
	return nil
}

func approvalAllowed(pending pendingConnection, viewer Viewer) error {
	if pending.profileID != "" && pending.profileID != viewer.ID {
		return ErrAlreadyApproved
	}
	if pending.remote && (viewer.Owner || !viewer.Remote || !viewer.Secured || !viewer.StronglyVerified) {
		return ErrRemoteViewer
	}
	return nil
}

func approvalCode(value string, viewer Viewer) (string, error) {
	if len(value) > 16 || viewer.ID == "" || !validField(viewer.ID, 128) {
		return "", ErrInvalidRequest
	}
	return strings.ToUpper(strings.TrimSpace(value)), nil
}

// Consume removes one approved request and returns its grant.
func (broker *Broker) Consume(secret string) (Grant, error) {
	if !validField(secret, 128) {
		return Grant{}, ErrNotFound
	}
	key := secretKey(secret)
	broker.mu.Lock()
	defer broker.mu.Unlock()
	broker.prune(broker.now())
	pending, found := broker.pending[key]
	if !found {
		return Grant{}, ErrNotFound
	}
	if pending.profileID == "" {
		return Grant{}, ErrPending
	}
	broker.remove(key, pending.connection.Code)
	return Grant{ProfileID: pending.profileID, Device: pending.connection.Device, ProfileRevision: pending.profileRevision, Remote: pending.remote, Owner: pending.owner}, nil
}

// RevokeRemote removes every public authorization request.
func (broker *Broker) RevokeRemote() {
	if broker == nil {
		return
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	for key, pending := range broker.pending {
		if pending.remote {
			broker.remove(key, pending.connection.Code)
		}
	}
}

func (broker *Broker) prune(now time.Time) {
	for key, pending := range broker.pending {
		if !pending.expires.After(now) {
			broker.remove(key, pending.connection.Code)
		}
	}
}

func (broker *Broker) remove(key, code string) {
	delete(broker.pending, key)
	delete(broker.byCode, code)
}

func connectCode(numeric bool, random io.Reader) (string, error) {
	if !numeric {
		return rand.Text()[:8], nil
	}
	value, err := rand.Int(random, big.NewInt(900000))
	if err != nil {
		return "", err
	}
	return big.NewInt(100000 + value.Int64()).String(), nil
}

func secretKey(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func validRequest(request Request) bool {
	return validField(request.Device, 80) && validField(request.DeviceID, 128) && validField(request.Client, 80) && validField(request.Version, 40)
}

func validField(value string, maximum int) bool {
	if len(value) > maximum {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

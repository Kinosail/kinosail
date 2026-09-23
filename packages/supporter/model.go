// Package supporter owns Player's optional supporter certificate protocol and state policy.
package supporter

import (
	"errors"
	"net/http"
	"time"
)

const (
	FamilyPatron    = "patron-order"
	FamilyLiving    = "living-standard"
	EditionOnce     = "one-time"
	EditionMonthly  = "monthly"
	EditionYearly   = "yearly"
	MaximumLevel    = 10
	activationSlots = 4
	completeFleetID = "complete-fleet"
)

// LegacyPolicy selects the historical certificate formats an app already persisted.
type LegacyPolicy uint8

const (
	LegacyNone LegacyPolicy = iota
	LegacyPlayer
	LegacySubtitles
)

var (
	ErrInvalid     = errors.New("invalid supporter activation")
	ErrUnavailable = errors.New("supporter activation unavailable")
	ErrUncertain   = errors.New("supporter activation result uncertain")
	ErrConflict    = errors.New("supporter activation conflicts with saved recognition")
)

// App identifies one Kinosail derivative while Player owns the protocol rules.
type App struct {
	ID, Name, Audience, MasterworkName                                                                         string
	FamilyPatron, FamilyLiving                                                                                 string
	EmptyBadges, NestedApp, TrackActivations, AllowEmptyRecognitionField, GrantPublicKey                       bool
	RejectPatronDowngrade, ExposeActivationAvailable, AcceptInvalidRequestError, BindSupporterIDToInstallation bool
	Legacy                                                                                                     LegacyPolicy
}

// Config supplies one app identity and the explicit remote activation adapter.
type Config struct {
	App              App
	ActivationURL    string
	SupportURL       string
	TrustedPublicKey string
	HTTPClient       *http.Client
	Now              func() time.Time
}

// Grant is one signed, app-bound supporter certificate.
type Grant struct {
	ActivationID string `json:"activationId,omitempty"`
	Certificate  string `json:"certificate,omitempty"`
	Signature    string `json:"signature,omitempty"`
	PublicKey    string `json:"publicKey,omitempty"`
	KeyHash      string `json:"keyHash,omitempty"`
}

// Activation preserves a bounded key-to-provider reference without storing the key.
type Activation struct {
	ActivationID string `json:"activationId,omitempty"`
	KeyHash      string `json:"keyHash,omitempty"`
}

// State is the complete local supporter record.
type State struct {
	InstallationKey string                      `json:"installationKey,omitempty"`
	PublicKey       string                      `json:"publicKey,omitempty"`
	Monthly         *Grant                      `json:"monthly,omitempty"`
	Yearly          *Grant                      `json:"yearly,omitempty"`
	MonthlyLevel    int                         `json:"monthlyLevel,omitempty"`
	YearlyLevel     int                         `json:"yearlyLevel,omitempty"`
	PatronOrder     *Grant                      `json:"patronOrder,omitempty"`
	LivingStandard  *Grant                      `json:"livingStandard,omitempty"`
	PatronLevel     int                         `json:"patronLevel,omitempty"`
	LivingLevel     int                         `json:"livingLevel,omitempty"`
	Activations     [activationSlots]Activation `json:"activations,omitempty,omitzero"`
	ActivationID    string                      `json:"activationId,omitempty"`
	Certificate     string                      `json:"certificate,omitempty"`
	Signature       string                      `json:"signature,omitempty"`
	KeyHash         string                      `json:"keyHash,omitempty"`
}

// Collection describes a certificate that recognizes support across applications.
type Collection struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Edition string   `json:"edition"`
	AppIDs  []string `json:"appIds"`
}

// Certificate is the verified signed record used by local presentation adapters.
type Certificate struct {
	Edition          string      `json:"edition,omitempty"`
	Version          int         `json:"version"`
	Audience         string      `json:"audience"`
	AppID            string      `json:"appId"`
	Family           string      `json:"family,omitempty"`
	Level            int         `json:"level,omitempty"`
	Tier             string      `json:"tier"`
	SupporterID      string      `json:"supporterId"`
	SupportedSince   string      `json:"supportedSince"`
	IssuedAt         string      `json:"issuedAt"`
	ExpiresAt        string      `json:"expiresAt"`
	Sustaining       bool        `json:"sustaining"`
	SubscriptionTier string      `json:"subscriptionTier,omitempty"`
	Founding         bool        `json:"founding"`
	InstallationKey  string      `json:"installationKey"`
	RecognitionName  string      `json:"recognitionName,omitempty"`
	Collection       *Collection `json:"collection,omitempty"`
}

// Badge is one verified local presentation of a grant.
type Badge struct {
	Edition          string      `json:"edition,omitempty"`
	Family           string      `json:"family"`
	Title            string      `json:"-"`
	Tier             string      `json:"tier"`
	Name             string      `json:"name"`
	SupporterID      string      `json:"supporterId,omitempty"`
	SupportedSince   string      `json:"supportedSince,omitempty"`
	ExpiresAt        string      `json:"expiresAt,omitempty"`
	RecognitionName  string      `json:"recognitionName,omitempty"`
	SubscriptionTier string      `json:"subscriptionTier,omitempty"`
	SubscriptionName string      `json:"subscriptionName,omitempty"`
	Rank             int         `json:"rank"`
	ServiceMonths    int         `json:"-"`
	ServiceMarks     []int       `json:"serviceMarks,omitempty"`
	Active           bool        `json:"active"`
	Expired          bool        `json:"expired"`
	Archived         bool        `json:"-"`
	Founding         bool        `json:"founding"`
	Collection       *Collection `json:"collection,omitempty"`
}

// BadgeCase summarizes historical progress across both badge families.
type BadgeCase struct {
	MonthlyLevel     int    `json:"monthlyLevel,omitempty"`
	YearlyLevel      int    `json:"yearlyLevel,omitempty"`
	LivingLevel      int    `json:"livingLevel"`
	PatronLevel      int    `json:"patronLevel"`
	MasterworkLevel  int    `json:"masterworkLevel"`
	Unlocked         int    `json:"unlocked"`
	Total            int    `json:"total"`
	MasterworkName   string `json:"masterworkName"`
	MasterworkEarned bool   `json:"masterworkEarned"`
	MasterworkActive bool   `json:"masterworkActive"`
}

// AppStatus names the application in status documents that use a nested identity.
type AppStatus struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Status is the shared supporter projection returned by all application adapters.
type Status struct {
	Monthly             *Badge     `json:"monthly,omitempty"`
	Yearly              *Badge     `json:"yearly,omitempty"`
	AppID               string     `json:"appId"`
	Tier                string     `json:"tier"`
	Name                string     `json:"name"`
	SupporterID         string     `json:"supporterId,omitempty"`
	SupportedSince      string     `json:"supportedSince,omitempty"`
	ExpiresAt           string     `json:"expiresAt,omitempty"`
	SubscriptionTier    string     `json:"subscriptionTier,omitempty"`
	SubscriptionName    string     `json:"subscriptionName,omitempty"`
	RecognitionName     string     `json:"recognitionName,omitempty"`
	Rank                int        `json:"rank"`
	Active              bool       `json:"active"`
	Expired             bool       `json:"expired"`
	Sustaining          bool       `json:"sustaining"`
	SubscriptionActive  bool       `json:"subscriptionActive"`
	Founding            bool       `json:"founding"`
	CompleteFleetActive bool       `json:"completeFleetActive"`
	ActivationAvailable bool       `json:"-"`
	App                 *AppStatus `json:"-"`
	PatronOrder         *Badge     `json:"patronOrder,omitempty"`
	LivingStandard      *Badge     `json:"livingStandard,omitempty"`
	BadgeCase           BadgeCase  `json:"badgeCase"`
	SupportURL          string     `json:"supportUrl"`
}

// ViewerBadge is the privacy-safe subset exposed outside the owner interface.
type ViewerBadge struct {
	Edition  string `json:"edition,omitempty"`
	Family   string `json:"family"`
	Tier     string `json:"tier"`
	Name     string `json:"name"`
	Rank     int    `json:"rank"`
	Active   bool   `json:"active"`
	Archived bool   `json:"archived"`
}

// ActivationInput contains the only user values sent to the supporter provider.
type ActivationInput struct {
	Key             string  `json:"key"`
	RecognitionName *string `json:"recognitionName,omitempty"`
}

// RotationInput confirms an explicit local public-key rotation.
type RotationInput struct {
	Version   int    `json:"version"`
	PublicKey string `json:"publicKey"`
	Confirm   bool   `json:"confirm"`
}

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

type App struct {
	ID, Name, Audience, MasterworkName                                                                         string
	FamilyPatron, FamilyLiving                                                                                 string
	EmptyBadges, NestedApp, TrackActivations, AllowEmptyRecognitionField, GrantPublicKey                       bool
	RejectPatronDowngrade, ExposeActivationAvailable, AcceptInvalidRequestError, BindSupporterIDToInstallation bool
	Legacy                                                                                                     LegacyPolicy
}

type Config struct {
	App              App
	ActivationURL    string
	SupportURL       string
	TrustedPublicKey string
	HTTPClient       *http.Client
	Now              func() time.Time
}

type Grant struct {
	ActivationID string `json:"activationId,omitempty"`
	Certificate  string `json:"certificate,omitempty"`
	Signature    string `json:"signature,omitempty"`
	PublicKey    string `json:"publicKey,omitempty"`
	KeyHash      string `json:"keyHash,omitempty"`
}

type Activation struct {
	ActivationID string `json:"activationId,omitempty"`
	KeyHash      string `json:"keyHash,omitempty"`
}

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

type Collection struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Edition string   `json:"edition"`
	AppIDs  []string `json:"appIds"`
}

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

type AppStatus struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

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

type ViewerBadge struct {
	Edition  string `json:"edition,omitempty"`
	Family   string `json:"family"`
	Tier     string `json:"tier"`
	Name     string `json:"name"`
	Rank     int    `json:"rank"`
	Active   bool   `json:"active"`
	Archived bool   `json:"archived"`
}

type ActivationInput struct {
	Key             string  `json:"key"`
	RecognitionName *string `json:"recognitionName,omitempty"`
}

type RotationInput struct {
	Version   int    `json:"version"`
	PublicKey string `json:"publicKey"`
	Confirm   bool   `json:"confirm"`
}

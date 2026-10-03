package server

import (
	settingsops "github.com/MikeO7/kinosail/packages/settings"
	"github.com/MikeO7/kinosail/packages/transcodehardware"
)

type installationSettings struct {
	Name                       string   `json:"name"`
	Libraries                  []string `json:"libraries"`
	RequireMFA                 bool     `json:"requireMfa"`
	SessionInactiveHours       float64  `json:"sessionInactiveHours,omitempty"`
	SessionAbsoluteHours       float64  `json:"sessionAbsoluteHours,omitempty"`
	PublicSessionInactiveHours float64  `json:"publicSessionInactiveHours,omitempty"`
	PublicSessionAbsoluteHours float64  `json:"publicSessionAbsoluteHours,omitempty"`
	JellyfinCompatibility      bool     `json:"jellyfinCompatibility"`
	HomeAssistant              bool     `json:"homeAssistant,omitempty"`
	JellyfinID                 string   `json:"jellyfinId,omitempty"`
	settingsops.Playback
	transcodehardware.Selection
	SubtitleLanguage      string          `json:"subtitleLanguage,omitempty"`
	SubtitlePickerLimited bool            `json:"subtitlePickerLimited,omitempty"`
	ScanFrequency         string          `json:"scanFrequency,omitempty"`
	DLNAToken             string          `json:"dlnaToken,omitempty"`
	Navigation            []string        `json:"navigation"`
	OnboardingPending     bool            `json:"onboardingPending,omitempty"`
	UpdateChecks          bool            `json:"updateChecks"`
	SupporterDisplay      string          `json:"supporterDisplay,omitempty"`
	Supporter             *supporterState `json:"supporter,omitempty"`
}

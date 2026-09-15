package scimapp

import (
	"time"

	sharedscim "github.com/MikeO7/kinosail/packages/scim"
)

type (
	// Email is one SCIM email address.
	Email = sharedscim.Email
	// EnterpriseProfile is the SCIM enterprise extension.
	EnterpriseProfile = sharedscim.EnterpriseProfile
	// Name is one SCIM structured name.
	Name = sharedscim.Name
	// Profile is one SCIM resource projection.
	Profile = sharedscim.Profile
	// ProfileInput is one validated SCIM resource replacement.
	ProfileInput = sharedscim.ProfileInput
	// Repository is the protocol repository implemented by RepositoryAdapter.
	Repository = sharedscim.Repository
)

// RepositoryAdapter maps one app's profile store onto the shared SCIM repository.
type RepositoryAdapter[T any] struct {
	ListProfiles  func() []T
	GetProfile    func(string) (T, bool)
	CreateProfile func(ProfileInput) (T, error)
	UpdateProfile func(string, ProfileInput, string) (T, error)
	DeleteProfile func(string, string) error
	Project       func(T) Profile
}

// NewRepository creates one shared repository from app-owned profile operations.
func NewRepository[T any](list func() []T, get func(string) (T, bool), create func(ProfileInput) (T, error), update func(string, ProfileInput, string) (T, error), deleteProfile func(string, string) error, project func(T) Profile) Repository {
	return RepositoryAdapter[T]{ListProfiles: list, GetProfile: get, CreateProfile: create, UpdateProfile: update, DeleteProfile: deleteProfile, Project: project}
}

// ProjectProfile builds one isolated protocol projection from app-owned fields.
func ProjectProfile(id, name, userName, externalID string, nameParts Name, emails []Email, enterprise EnterpriseProfile, disabled bool, createdAt, updatedAt time.Time, revision uint64) Profile {
	return Profile{ID: id, Name: name, UserName: userName, ExternalID: externalID, NameParts: nameParts, Emails: append([]Email(nil), emails...), Enterprise: enterprise, Disabled: disabled, CreatedAt: createdAt, UpdatedAt: updatedAt, Revision: revision}
}

// List returns each provisioned app profile as a protocol profile.
func (adapter RepositoryAdapter[T]) List() []Profile {
	profiles := adapter.ListProfiles()
	result := make([]Profile, 0, len(profiles))
	for _, profile := range profiles {
		result = append(result, adapter.Project(profile))
	}
	return result
}

// Get returns one provisioned app profile as a protocol profile.
func (adapter RepositoryAdapter[T]) Get(id string) (Profile, bool) {
	profile, found := adapter.GetProfile(id)
	if !found {
		return Profile{}, false
	}
	return adapter.Project(profile), true
}

// Create commits and projects one validated profile.
func (adapter RepositoryAdapter[T]) Create(input ProfileInput) (Profile, error) {
	profile, err := adapter.CreateProfile(input)
	return adapter.Project(profile), err
}

// Update commits and projects one validated profile replacement.
func (adapter RepositoryAdapter[T]) Update(id string, input ProfileInput, expected string) (Profile, error) {
	profile, err := adapter.UpdateProfile(id, input, expected)
	return adapter.Project(profile), err
}

// Delete commits one validated profile deletion.
func (adapter RepositoryAdapter[T]) Delete(id, expected string) error {
	return adapter.DeleteProfile(id, expected)
}

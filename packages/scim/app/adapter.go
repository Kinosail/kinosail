package scimapp

import (
	"time"

	sharedscim "github.com/MikeO7/kinosail/packages/scim"
)

type (
	Email             = sharedscim.Email
	EnterpriseProfile = sharedscim.EnterpriseProfile
	Name              = sharedscim.Name
	Profile           = sharedscim.Profile
	ProfileInput      = sharedscim.ProfileInput
	Repository        = sharedscim.Repository
)

type RepositoryAdapter[T any] struct {
	ListProfiles  func() []T
	GetProfile    func(string) (T, bool)
	CreateProfile func(ProfileInput) (T, error)
	UpdateProfile func(string, ProfileInput, string) (T, error)
	DeleteProfile func(string, string) error
	Project       func(T) Profile
}

func NewRepository[T any](list func() []T, get func(string) (T, bool), create func(ProfileInput) (T, error), update func(string, ProfileInput, string) (T, error), deleteProfile func(string, string) error, project func(T) Profile) Repository {
	return RepositoryAdapter[T]{ListProfiles: list, GetProfile: get, CreateProfile: create, UpdateProfile: update, DeleteProfile: deleteProfile, Project: project}
}

func ProjectProfile(id, name, userName, externalID string, nameParts Name, emails []Email, enterprise EnterpriseProfile, disabled bool, createdAt, updatedAt time.Time, revision uint64) Profile {
	return Profile{ID: id, Name: name, UserName: userName, ExternalID: externalID, NameParts: nameParts, Emails: append([]Email(nil), emails...), Enterprise: enterprise, Disabled: disabled, CreatedAt: createdAt, UpdatedAt: updatedAt, Revision: revision}
}

func (adapter RepositoryAdapter[T]) List() []Profile {
	profiles := adapter.ListProfiles()
	result := make([]Profile, 0, len(profiles))
	for _, profile := range profiles {
		result = append(result, adapter.Project(profile))
	}
	return result
}

func (adapter RepositoryAdapter[T]) Get(id string) (Profile, bool) {
	profile, found := adapter.GetProfile(id)
	if !found {
		return Profile{}, false
	}
	return adapter.Project(profile), true
}

func (adapter RepositoryAdapter[T]) Create(input ProfileInput) (Profile, error) {
	profile, err := adapter.CreateProfile(input)
	return adapter.Project(profile), err
}

func (adapter RepositoryAdapter[T]) Update(id string, input ProfileInput, expected string) (Profile, error) {
	profile, err := adapter.UpdateProfile(id, input, expected)
	return adapter.Project(profile), err
}

func (adapter RepositoryAdapter[T]) Delete(id, expected string) error {
	return adapter.DeleteProfile(id, expected)
}

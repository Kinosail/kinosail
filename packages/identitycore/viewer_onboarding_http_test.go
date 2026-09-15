package identitycore

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

type onboardingViewerEffects struct {
	calls          int
	name, password string
	owner          bool
	policy         ProfilePolicy
	err            error
}

func (effects *onboardingViewerEffects) add(name, password string, owner bool, policy ProfilePolicy) (string, error) {
	effects.calls++
	effects.name, effects.password, effects.owner, effects.policy = name, password, owner, policy
	return "viewer", effects.err
}

func validHouseholdForm() url.Values {
	return url.Values{
		"kind": {"child"}, "owner": {"false"}, "rating": {"teen"}, "libraries": {"all"},
		"name": {"Viewer"}, "password": {"windward-viewer-2026"},
	}
}

func TestOnboardingViewerHandlerRejectsInvalidInputBeforeAdd(t *testing.T) {
	t.Parallel()
	valid := validHouseholdForm().Encode()
	tests := map[string]struct {
		path, body, contentType, message string
	}{
		"missing media type":  {"/onboarding/household", valid, "", invalidViewerProfile},
		"wrong media type":    {"/onboarding/household", valid, "text/plain", invalidViewerProfile},
		"query":               {"/onboarding/household?kind=parent", valid, "application/x-www-form-urlencoded", invalidViewerProfile},
		"malformed":           {"/onboarding/household", "kind=%zz", "application/x-www-form-urlencoded", invalidViewerProfile},
		"unknown field":       {"/onboarding/household", valid + "&unexpected=true", "application/x-www-form-urlencoded", invalidViewerProfile},
		"repeated kind":       {"/onboarding/household", valid + "&kind=child", "application/x-www-form-urlencoded", invalidViewerProfile},
		"conflicting kind":    {"/onboarding/household", valid + "&kind=parent", "application/x-www-form-urlencoded", invalidViewerProfile},
		"oversized body":      {"/onboarding/household", "kind=child&" + strings.Repeat("x", viewerMutationFormLimit), "application/x-www-form-urlencoded", invalidViewerProfile},
		"missing kind":        {"/onboarding/household", "owner=false&rating=teen&libraries=all&name=Viewer&password=windward-viewer-2026", "application/x-www-form-urlencoded", householdProfileError},
		"invalid kind":        {"/onboarding/household", strings.Replace(valid, "kind=child", "kind=owner", 1), "application/x-www-form-urlencoded", householdProfileError},
		"missing owner":       {"/onboarding/household", strings.Replace(valid, "owner=false&", "", 1), "application/x-www-form-urlencoded", householdAccessError},
		"owner true":          {"/onboarding/household", strings.Replace(valid, "owner=false", "owner=true", 1), "application/x-www-form-urlencoded", householdAccessError},
		"missing libraries":   {"/onboarding/household", strings.Replace(valid, "libraries=all&", "", 1), "application/x-www-form-urlencoded", householdAccessError},
		"wrong libraries":     {"/onboarding/household", strings.Replace(valid, "libraries=all", "libraries=Movies", 1), "application/x-www-form-urlencoded", householdAccessError},
		"missing rating":      {"/onboarding/household", strings.Replace(valid, "&rating=teen", "", 1), "application/x-www-form-urlencoded", householdRatingError},
		"invalid rating":      {"/onboarding/household", strings.Replace(valid, "rating=teen", "rating=mature", 1), "application/x-www-form-urlencoded", householdRatingError},
		"parent rating":       {"/onboarding/household", strings.NewReplacer("kind=child", "kind=parent", "rating=teen", "rating=family").Replace(valid), "application/x-www-form-urlencoded", parentRatingError},
		"missing name":        {"/onboarding/household", strings.Replace(valid, "name=Viewer&", "", 1), "application/x-www-form-urlencoded", profileInputError},
		"empty name":          {"/onboarding/household", strings.Replace(valid, "name=Viewer", "name=+++", 1), "application/x-www-form-urlencoded", profileInputError},
		"oversized name":      {"/onboarding/household", strings.Replace(valid, "name=Viewer", "name="+strings.Repeat("n", maxNameLength+1), 1), "application/x-www-form-urlencoded", profileInputError},
		"missing password":    {"/onboarding/household", strings.Replace(valid, "&password=windward-viewer-2026", "", 1), "application/x-www-form-urlencoded", profileInputError},
		"weak password":       {"/onboarding/household", strings.Replace(valid, "windward-viewer-2026", "short", 1), "application/x-www-form-urlencoded", profileInputError},
		"oversized password":  {"/onboarding/household", strings.Replace(valid, "windward-viewer-2026", strings.Repeat("p", viewerPasswordLimit+1), 1), "application/x-www-form-urlencoded", profileInputError},
		"invalid flag":        {"/onboarding/household", valid + "&remote=false", "application/x-www-form-urlencoded", invalidViewerProfile},
		"oversized start":     {"/onboarding/household", valid + "&start=009%3A00", "application/x-www-form-urlencoded", invalidViewerProfile},
		"incomplete schedule": {"/onboarding/household", valid + "&start=09%3A00", "application/x-www-form-urlencoded", "both viewing schedule times are required"},
		"malformed schedule":  {"/onboarding/household", valid + "&start=09%3A00&end=bad", "application/x-www-form-urlencoded", "viewing schedule must use HH:MM"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			effects, result := new(onboardingViewerEffects), new(viewerMutationError)
			response := httptest.NewRecorder()
			request := viewerMutationRequest(t, test.path, test.body, test.contentType)
			OnboardingViewerHandler(effects.add, result.write).ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest || result.message != test.message || result.status != http.StatusBadRequest || effects.calls != 0 {
				t.Fatalf("response = %d, error = %#v, effects = %#v", response.Code, result, effects)
			}
		})
	}
}

func TestOnboardingViewerHandlerAcceptsExactFieldLimits(t *testing.T) {
	t.Parallel()
	name, password := strings.Repeat("n", maxNameLength), strings.Repeat("p", viewerPasswordLimit)
	form := url.Values{
		"kind": {"parent"}, "owner": {"false"}, "rating": {"all"}, "libraries": {"all"},
		"name": {" " + name + " "}, "password": {password}, "start": {"09:00"}, "end": {"17:00"},
		"remote": {"true"}, "transcode": {"true"}, "downloads": {"true"},
	}
	effects, result := new(onboardingViewerEffects), new(viewerMutationError)
	response := httptest.NewRecorder()
	OnboardingViewerHandler(effects.add, result.write).ServeHTTP(response, viewerMutationRequest(t, "/onboarding/household", form.Encode(), "application/x-www-form-urlencoded; charset=utf-8"))
	wantPolicy := ProfilePolicy{Downloads: true, Transcode: true, Remote: true, Rating: "all", Libraries: []string{"all"}, AccessStart: "09:00", AccessEnd: "17:00"}
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/onboarding/household" || effects.calls != 1 || effects.name != name || effects.password != password || effects.owner || !reflect.DeepEqual(effects.policy, wantPolicy) || result.status != 0 {
		t.Fatalf("response = %d %q, error = %#v, effects = %#v", response.Code, response.Header().Get("Location"), result, effects)
	}
}

func TestOnboardingViewerHandlerPreservesDomainErrors(t *testing.T) {
	t.Parallel()
	storeError := errors.New("profile transaction failed")
	effects, result := &onboardingViewerEffects{err: storeError}, new(viewerMutationError)
	response := httptest.NewRecorder()
	OnboardingViewerHandler(effects.add, result.write).ServeHTTP(response, viewerMutationRequest(t, "/onboarding/household", validHouseholdForm().Encode(), "application/x-www-form-urlencoded"))
	if response.Code != http.StatusBadRequest || result.message != storeError.Error() || effects.calls != 1 || response.Header().Get("Location") != "" {
		t.Fatalf("response = %d %q, error = %#v, effects = %#v", response.Code, response.Header().Get("Location"), result, effects)
	}
}

func TestOnboardingViewerHandlerFailsClosedWithoutCallbacks(t *testing.T) {
	t.Parallel()
	for name, handler := range map[string]http.HandlerFunc{
		"add":   OnboardingViewerHandler(nil, new(viewerMutationError).write),
		"error": OnboardingViewerHandler(func(string, string, bool, ProfilePolicy) (string, error) { return "", nil }, nil),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/onboarding/household", nil))
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("response = %d", response.Code)
			}
		})
	}
}

package server_test

import "testing"

func TestInvalidLanguageSelectionHasNoSideEffects(t *testing.T) {
	localeWebFixture.InvalidLanguageSelectionHasNoSideEffects(t)
}

func TestExpandedLanguageTagsPreserveRegionalAndRTLSemantics(t *testing.T) {
	localeWebFixture.ExpandedLanguageTagsPreserveRegionalAndRTLSemantics(t)
}

func TestLanguagePickerOffersEveryLocaleOnTheHomePage(t *testing.T) {
	localeWebFixture.LanguagePickerOffersEveryLocaleOnTheHomePage(t)
}

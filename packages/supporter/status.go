package supporter

import "time"

// Status returns a private owner projection without changing state.
func (service *Service) Status(state State) Status { //nolint:cyclop,gocognit // One projection preserves cross-version badge precedence.
	state = service.migrateLegacy(cloneState(state))
	patron := service.badge(service.decodeGrant(state, state.PatronOrder, FamilyPatron, true))
	living := service.badge(service.decodeGrant(state, state.LivingStandard, FamilyLiving, true))
	if legacy := legacyGrant(state); legacy != nil {
		if patron == nil {
			patron = service.badge(service.decodeGrant(state, legacy, FamilyPatron, true))
		}
		if living == nil {
			living = service.badge(service.decodeGrant(state, legacy, FamilyLiving, true))
		}
	}
	status := Status{Tier: "free", Name: "Free", PatronOrder: patron, LivingStandard: living, SupportURL: service.supportURL, ActivationAvailable: service.app.ExposeActivationAvailable && service.ActivationAvailable()}
	if service.app.NestedApp {
		status.App = &AppStatus{ID: service.app.ID, Name: service.app.Name}
	} else {
		status.AppID = service.app.ID
	}
	if service.app.EmptyBadges {
		if status.PatronOrder == nil {
			status.PatronOrder = service.emptyBadge(FamilyPatron)
		}
		if status.LivingStandard == nil {
			status.LivingStandard = service.emptyBadge(FamilyLiving)
		}
	}
	status.Monthly = service.badge(service.decodeGrant(state, state.Monthly, EditionMonthly, true))
	status.Yearly = service.badge(service.decodeGrant(state, state.Yearly, EditionYearly, true))
	status.BadgeCase = service.badgeCase(state, patron, living)
	status.BadgeCase.MonthlyLevel = earnedEditionLevel(state.MonthlyLevel, status.Monthly)
	status.BadgeCase.YearlyLevel = earnedEditionLevel(state.YearlyLevel, status.Yearly)
	if service.app.ID == "kino-player" {
		status.BadgeCase.Total = 30
		if status.BadgeCase.LivingLevel > 0 {
			status.BadgeCase.Total += 10
		}
		status.BadgeCase.Unlocked = status.BadgeCase.PatronLevel + status.BadgeCase.MonthlyLevel + status.BadgeCase.YearlyLevel + status.BadgeCase.LivingLevel
	}
	status.CompleteFleetActive = completeFleetActive(patron, living, status.Monthly, status.Yearly)
	primary := preferredBadge(status.Monthly, status.Yearly, living, patron)
	if primary == nil {
		return status
	}
	status.Tier, status.Name, status.SupporterID = primary.Tier, primary.Name, primary.SupporterID
	status.SupportedSince, status.ExpiresAt, status.Rank = primary.SupportedSince, primary.ExpiresAt, primary.Rank
	status.Active, status.Expired, status.Sustaining = primary.Active, primary.Expired, primary.Family == service.outputFamily(FamilyLiving)
	status.Founding, status.RecognitionName = primary.Founding, primary.RecognitionName
	if living != nil {
		status.SubscriptionTier, status.SubscriptionName = living.SubscriptionTier, living.SubscriptionName
		status.SubscriptionActive = living.Active
	}
	status.SubscriptionActive = living != nil && living.Active || status.Monthly != nil && status.Monthly.Active || status.Yearly != nil && status.Yearly.Active
	return status
}

func (service *Service) badge(decoded decodedGrant) *Badge {
	if !decoded.valid {
		return nil
	}
	certificate := decoded.certificate
	badge := &Badge{
		Edition: certificate.Edition, Family: service.outputFamily(decoded.family), Title: familyTitle(decoded.family), Tier: certificate.Tier, Name: Name(certificate.Tier),
		SupporterID: certificate.SupporterID, SupportedSince: certificate.SupportedSince, ExpiresAt: certificate.ExpiresAt,
		RecognitionName: certificate.RecognitionName, SubscriptionTier: certificate.SubscriptionTier, SubscriptionName: subscriptionName(certificate.SubscriptionTier),
		Rank: Rank(certificate.Tier), Active: !decoded.expired, Expired: decoded.expired, Archived: decoded.expired, Founding: certificate.Founding,
		Collection: cloneCollection(certificate.Collection),
	}
	if decoded.family == FamilyPatron {
		badge.Edition = EditionOnce
	}
	if decoded.family == FamilyLiving {
		badge.ServiceMonths, badge.ServiceMarks = serviceMonths(certificate, service.now().UTC())
	}
	return badge
}

func (service *Service) emptyBadge(family string) *Badge {
	return &Badge{Family: service.outputFamily(family), Title: familyTitle(family), Tier: "free", Name: "Free"}
}

func (service *Service) badgeCase(state State, patron, living *Badge) BadgeCase {
	livingLevel, patronLevel := state.LivingLevel, state.PatronLevel
	if living != nil {
		livingLevel = max(livingLevel, living.Rank)
	}
	if patron != nil {
		patronLevel = max(patronLevel, patron.Rank)
	}
	if livingLevel < 0 || livingLevel > MaximumLevel {
		livingLevel = 0
	}
	if patronLevel < 0 || patronLevel > MaximumLevel {
		patronLevel = 0
	}
	level := min(livingLevel, patronLevel)
	return BadgeCase{
		LivingLevel: livingLevel, PatronLevel: patronLevel, MasterworkLevel: level, Unlocked: livingLevel + patronLevel,
		Total: 20, MasterworkName: service.app.MasterworkName, MasterworkEarned: level > 0, MasterworkActive: level > 0 && living != nil && living.Active,
	}
}

func serviceMonths(certificate Certificate, now time.Time) (int, []int) {
	start, err := time.Parse(time.RFC3339Nano, certificate.SupportedSince)
	if err != nil {
		return 0, nil
	}
	end := now
	if expiry, expiryErr := time.Parse(time.RFC3339Nano, certificate.ExpiresAt); expiryErr == nil && expiry.Before(end) {
		end = expiry
	}
	months := (end.Year()-start.Year())*12 + int(end.Month()-start.Month())
	if start.AddDate(0, months, 0).After(end) {
		months--
	}
	months = max(months, 0)
	marks := make([]int, 0, 6)
	for _, mark := range []int{3, 6, 12, 24, 36, 60} {
		if months >= mark {
			marks = append(marks, mark)
		}
	}
	return months, marks
}

// CertificateForFamily returns verified certificate data for local rendering.
func (service *Service) CertificateForFamily(state State, family string) (Certificate, *Badge, bool) {
	if family != FamilyPatron && family != FamilyLiving && !validEdition(family) {
		return Certificate{}, nil, false
	}
	state = service.migrateLegacy(cloneState(state))
	grant := state.PatronOrder
	if family == FamilyLiving {
		grant = state.LivingStandard
	}
	if family == EditionMonthly {
		grant = state.Monthly
	}
	if family == EditionYearly {
		grant = state.Yearly
	}
	if family == EditionOnce {
		family = FamilyPatron
	}
	decoded := service.decodeGrant(state, grant, family, true)
	badge := service.badge(decoded)
	return decoded.certificate, badge, badge != nil
}

// ViewerStatus removes private certificate and installation data.
func (service *Service) ViewerStatus(state State) []ViewerBadge {
	status := service.Status(state)
	result := make([]ViewerBadge, 0, 2)
	for _, badge := range []*Badge{status.PatronOrder, status.Monthly, status.Yearly, status.LivingStandard} {
		if badge != nil && badge.Rank > 0 {
			result = append(result, ViewerBadge{Edition: badge.Edition, Family: badge.Family, Tier: badge.Tier, Name: badge.Name, Rank: badge.Rank, Active: badge.Active, Archived: badge.Expired})
		}
	}
	return result
}

func (service *Service) outputFamily(family string) string {
	if family == FamilyLiving {
		return service.app.FamilyLiving
	}
	return service.app.FamilyPatron
}

func familyTitle(family string) string {
	if family == FamilyLiving {
		return "Living Standard"
	}
	return "Patron Order"
}

func completeFleetActive(badges ...*Badge) bool {
	for _, badge := range badges {
		if badge != nil && badge.Active && badge.Collection != nil && badge.Collection.ID == completeFleetID {
			return true
		}
	}
	return false
}

func cloneCollection(collection *Collection) *Collection {
	if collection == nil {
		return nil
	}
	cloned := *collection
	cloned.AppIDs = append([]string(nil), collection.AppIDs...)
	return &cloned
}

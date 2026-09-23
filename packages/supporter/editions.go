package supporter

func validEdition(edition string) bool {
	return edition == EditionOnce || edition == EditionMonthly || edition == EditionYearly
}

func earnedEditionLevel(level int, badge *Badge) int {
	if level < 0 || level > MaximumLevel {
		level = 0
	}
	if badge != nil {
		level = max(level, badge.Rank)
	}
	return level
}

func preferredBadge(badges ...*Badge) *Badge {
	for _, badge := range badges {
		if badge != nil && badge.Active {
			return badge
		}
	}
	for _, badge := range badges {
		if badge != nil {
			return badge
		}
	}
	return nil
}

func applyRecurring(state *State, grant *Grant, certificate Certificate) {
	rank := Rank(certificate.Tier)
	switch certificate.Edition {
	case EditionMonthly:
		state.Monthly, state.MonthlyLevel = grant, max(state.MonthlyLevel, rank)
	case EditionYearly:
		state.Yearly, state.YearlyLevel = grant, max(state.YearlyLevel, rank)
	default:
		state.LivingStandard, state.LivingLevel = grant, max(state.LivingLevel, rank)
	}
}

func (service *Service) validEditionGrants(state State) bool {
	return (state.Monthly == nil || service.decodeGrant(state, state.Monthly, EditionMonthly, true).valid) &&
		(state.Yearly == nil || service.decodeGrant(state, state.Yearly, EditionYearly, true).valid)
}

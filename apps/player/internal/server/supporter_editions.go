package server

func editionName(edition string) string {
	switch edition {
	case "one-time":
		return "One-time"
	case "monthly":
		return "Monthly"
	case "yearly":
		return "Yearly"
	}
	return ""
}

type supporterEditionView struct {
	Name    string
	Owned   *supporterOwnedBadgeView
	Preview supporterBadgeArt
}

func supporterEditionViews(status supporterStatus) []supporterEditionView {
	result := make([]supporterEditionView, 0, 3)
	for _, item := range []struct {
		edition string
		badge   *supporterBadgeStatus
	}{{"one-time", status.PatronOrder}, {"monthly", status.Monthly}, {"yearly", status.Yearly}} {
		result = append(result, supporterEditionView{Name: editionName(item.edition), Owned: supporterOwnedBadge(item.badge), Preview: supporterBadgeArt{Family: item.edition, FamilyName: editionName(item.edition), Rank: 1, Name: "Friend"}})
	}
	return result
}

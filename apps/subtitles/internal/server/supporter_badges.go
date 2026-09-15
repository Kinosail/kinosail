package server

type supporterTierView struct {
	Rank                          int
	Tier, Name, BadgeName, Detail string
	Outline, Ornament             string
	Current, Collected, Elite     bool
}

func supporterBadgeDefinition(tier, badgeName, detail, outline, ornament string) supporterTierView {
	return supporterTierView{Tier: tier, BadgeName: badgeName, Detail: detail, Outline: outline, Ornament: ornament}
}

var patronOrderBadges = []supporterTierView{
	{Tier: "friend", BadgeName: "Whisper Glyph", Detail: "A compact caption token marks permanent support.", Outline: "M120 28a92 92 0 1 1 0 184 92 92 0 1 1 0-184Z", Ornament: "M120 44v18M120 178v18"},
	{Tier: "crew", BadgeName: "Cue Pair", Detail: "A six-sided enamel order joins the Subtitles crew.", Outline: "M120 20 202 68v104l-82 48-82-48V68Z", Ornament: "M62 78h116M62 162h116"},
	{Tier: "navigator", BadgeName: "Timing Compass", Detail: "A clipped compass order records precise subtitle navigation.", Outline: "M120 16 192 52 220 120l-28 68-72 36-72-36-28-68 28-68Z", Ornament: "M120 38l15 42 42 15-42 15-15 42-15-42-42-15 42-15Z"},
	{Tier: "patron", BadgeName: "Dialogue Shield", Detail: "A deep enamel shield gives the caption emblem formal weight.", Outline: "M120 18 204 50l-12 108-72 64-72-64L36 50Z", Ornament: "M58 66l62-24 62 24-10 82-52 48-52-48Z"},
	{Tier: "steward", BadgeName: "Caption Medal", Detail: "An eight-sided archive medal preserves a permanent record.", Outline: "M78 20h84l58 58v84l-58 58H78l-58-58V78Z", Ornament: "M78 42h84l36 36v84l-36 36H78l-36-36V78Z"},
	{Tier: "lighthouse", BadgeName: "Linguist Crest", Detail: "A tall beacon crest guards the caption archive.", Outline: "M120 14 194 38l24 78-30 82-68 28-68-28-30-82 24-78Z", Ornament: "M82 62h76l24 58-24 58H82l-24-58Z"},
	{Tier: "commodore", BadgeName: "Polyglot Order", Detail: "Command wings and a gold caption plate define the first elite order.", Outline: "M120 14 156 40l68-16-20 70 18 66-62-10-40 76-40-76-62 10 18-66-20-70 68 16Z", Ornament: "M28 116h54M158 116h54M54 92l-28-20M186 92l28-20"},
	{Tier: "admiral", BadgeName: "Atlas Star", Detail: "An eight-point admiral star carries a double caption frame.", Outline: "M120 8l27 57 57-29-29 57 57 27-57 27 29 57-57-29-27 57-27-57-57 29 29-57-57-27 57-27-29-57 57 29Z", Ornament: "M120 34l20 46 46 20-46 20-20 46-20-46-46-20 46-20Z"},
	{Tier: "northstar", BadgeName: "Rosetta Tablet", Detail: "A celestial crown and compass points make this order unmistakable.", Outline: "M120 6 144 54 190 22 184 78 230 120 184 162 190 218 144 186 120 234 96 186 50 218 56 162 10 120 56 78 50 22 96 54Z", Ornament: "M58 82l30 18 32-58 32 58 30-18-12 76H70Z"},
	{Tier: "legacy", BadgeName: "Universal Voice Seal", Detail: "The sovereign seal combines crown, wings, star, and caption plate.", Outline: "M120 4 142 38 178 18 180 58 222 48 204 88 236 120 204 152 222 192 180 182 178 222 142 202 120 236 98 202 62 222 60 182 18 192 36 152 4 120 36 88 18 48 60 58 62 18 98 38Z", Ornament: "M44 120h34M162 120h34M72 72l24 20M168 72l-24 20M72 168l24-20M168 168l-24-20"},
}

var livingStandardBadges = []supporterTierView{
	supporterBadgeDefinition("friend", "Whisper Pulse", "A live caption pulse confirms current monthly support.", "M120 26c54 0 92 38 92 94s-38 94-92 94-92-38-92-94 38-94 92-94Zm0 18c-42 0-72 31-72 76s30 76 72 76 72-31 72-76-30-76-72-76Z", "M44 120h28l12-18 18 36 18-36 18 36 18-18h40"),
	supporterBadgeDefinition("crew", "Cue Signal", "Twin signal lobes make the active crew badge visible at a glance.", "M120 18c26 26 46 36 78 38-2 32 8 52 34 78-26 26-36 46-38 78-32-2-52 8-78 34-26-26-46-36-78-38 2-32-8-52-34-78 26-26 36-46 38-78 32 2 52-8 78-34Z", "M120 38v42M120 160v42M38 120h42M160 120h42"),
	supporterBadgeDefinition("navigator", "Timing Beat", "An offset orbital standard signals guided subtitle coverage.", "M120 12c32 34 62 42 104 36-6 42 2 72 36 104-34 32-42 62-36 104-42-6-72 2-104 36-32-34-62-42-104-36 6-42-2-72-36-104 34-32 42-62 36-104 42 6 72-2 104-36Z", "M52 92c34-48 102-48 136 0M52 148c34 48 102 48 136 0"),
	supporterBadgeDefinition("patron", "Dialogue Current", "A four-point watch standard frames the active caption field.", "M120 10 158 50 214 46l-4 56 20 18-20 18 4 56-56-4-38 40-38-40-56 4 4-56-20-18 20-18-4-56 56 4Z", "M120 30v44M120 166v44M30 120h44M166 120h44"),
	supporterBadgeDefinition("steward", "Sync Ribbon", "Two opposing beams show continuous stewardship of subtitle work.", "M120 8 154 54 210 30 186 86 232 120 186 154 210 210 154 186 120 232 86 186 30 210 54 154 8 120 54 86 30 30 86 54Z", "M28 78l64 28v28L28 162ZM212 78l-64 28v28l64 28Z"),
	supporterBadgeDefinition("lighthouse", "Caption Beacon", "A radiant standard keeps the current subscription state explicit.", "M120 8 148 48l46-20 2 52 36 40-36 40-2 52-46-20-28 40-28-40-46 20-2-52-36-40 36-40 2-52 46 20Z", "M72 72h96v96H72ZM120 30v34M120 176v34"),
	supporterBadgeDefinition("commodore", "Linguist Standard", "Long signal wings and command bars launch the elite standards.", "M120 6 154 40 226 18 204 92 234 120 204 148 226 222 154 200 120 234 86 200 14 222 36 148 6 120 36 92 14 18 86 40Z", "M22 110h62v20H22M156 110h62v20h-62M92 40h56M92 200h56"),
	supporterBadgeDefinition("admiral", "Polyglot Array", "A sixteen-point signal array surrounds the double caption field.", "M120 4 138 48 174 16 170 64 216 48 190 90 236 120 190 150 216 192 170 176 174 224 138 192 120 236 102 192 66 224 70 176 24 192 50 150 4 120 50 90 24 48 70 64 66 16 102 48Z", "M120 26v40M120 174v40M26 120h40M174 120h40M54 54l28 28M186 54l-28 28M54 186l28-28M186 186l-28-28"),
	supporterBadgeDefinition("northstar", "Atlas Chorus", "A wide constellation field records an exceptional active level.", "M120 2 140 42 178 12 178 58 224 42 198 82 238 120 198 158 224 198 178 182 178 228 140 198 120 238 100 198 62 228 62 182 16 198 42 158 2 120 42 82 16 42 62 58 62 12 100 42Z", "M52 88l28-18 20 18 20-44 20 44 20-18 28 18M52 152l28 18 20-18 20 44 20-44 20 18 28-18"),
	supporterBadgeDefinition("legacy", "Rosetta Crown", "The complete live crown joins array, constellation, wings, and caption emblem.", "M120 2 136 30 164 8 168 42 204 28 194 68 232 62 210 98 240 120 210 142 232 178 194 172 204 212 168 198 164 232 136 210 120 238 104 210 76 232 72 198 36 212 46 172 8 178 30 142 0 120 30 98 8 62 46 68 36 28 72 42 76 8 104 30Z", "M42 110h44M154 110h44M42 130h44M154 130h44M72 62l24 22M168 62l-24 22M72 178l24-22M168 178l-24-22"),
}

var perfectSyncNames = []string{"Joined Cue", "Twin Caption", "Timing Accord", "Dialogue Union", "Synchronized Crest", "Caption Concord", "Polyglot Chorus", "Atlas Harmony", "Rosetta Ascendant", "Sovereign Perfect Sync"}

func supporterBadgeTiers(family string, status supporterGrantStatus, collectedLevel int) []supporterTierView {
	source := patronOrderBadges
	if family == livingStandardFamily {
		source = livingStandardBadges
	}
	tiers := make([]supporterTierView, len(source))
	copy(tiers, source)
	for index := range tiers {
		tiers[index].Rank = index + 1
		tiers[index].Name = supporterName(tiers[index].Tier)
		tiers[index].Current = status.Rank == index+1
		tiers[index].Collected = collectedLevel >= index+1
		tiers[index].Elite = index >= 6
	}
	return tiers
}

func supporterBadge(family string, rank int) supporterTierView {
	if rank < 1 || rank > len(supporterTiers) {
		return supporterTierView{}
	}
	return supporterBadgeTiers(family, supporterGrantStatus{}, 0)[rank-1]
}

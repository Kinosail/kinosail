package server

import "strings"

func directFirstPlaybackCopy(page string) string {
	return strings.NewReplacer(
		"Automatic starts with the original file. If your device cannot play it, Kinosail switches to a compatible version.", "Direct First starts the original file on every connection. Wi-Fi speed, slow startup, and buffering never trigger conversion. After a confirmed format failure, Kinosail uses the smallest compatible change. Video transcoding always asks first.",
		"Automatic · Original first (Recommended)", "Direct First (Recommended)",
		"Original only", "Direct Play only",
		"Compatible version first", "Compatibility first",
	).Replace(page)
}

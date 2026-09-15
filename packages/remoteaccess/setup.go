package remoteaccess

import "strings"

// SetupStep gives Owners the same ordered instructions in the API and UI.
type SetupStep struct {
	Title       string       `json:"title"`
	Instruction string       `json:"instruction"`
	Fields      []SetupField `json:"fields,omitempty"`
	Links       []SetupLink  `json:"links,omitempty"`
}

// SetupField matches a field in a router's port-forwarding form.
type SetupField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// SetupLink points to a maintained manufacturer's guide, never a detected router.
type SetupLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

func remoteSetupSteps(readiness Readiness) []SetupStep {
	if readiness.Stopped {
		return []SetupStep{{Title: "Public access is turned off", Instruction: "Keep the router rule disabled while you investigate. When you are ready, return to Settings → Remote access, choose Allow public access after restart, then restart your Server and check this page again. Local access stays available."}}
	}
	steps := []SetupStep{}
	fixes := map[string]SetupStep{
		"authorization-state": {Title: "Restore Profile access", Instruction: "Your Server could not read its Profiles. Keep remote access off and restore the Server's Profile storage before continuing."},
		"viewer-signin":       {Title: "Prepare a Viewer Profile", Instruction: "At home, sign in as an Owner and create a Viewer Profile in Settings. Allow only the Libraries they need, leave downloads off for playback-only use, and enable that Profile's remote access. Sign in locally as that Viewer and open Account → Set up an authenticator app. Keep this device signed in to approve Quick Connect later. Owner Profiles cannot sign in through the public address."},
		"public-https":        {Title: "Enable public HTTPS", Instruction: "On the computer running your Server, open the Kinosail installation folder in a terminal. Run ./scripts/setup-remote-access.sh, choose https, and follow the prompts to create a free DuckDNS address and restart. Have your router app or router administrator password ready. Trusted HTTPS for local devices alone does not enable remote access."},
		"remote-policy":       {Title: "Apply the remote configuration", Instruction: "Restart using the setup script's HTTPS configuration. If the public access policy still needs attention, leave the router rule off and update your Server before continuing."},
		"listener":            {Title: "Start public HTTPS", Instruction: "Check Settings → Remote access for the error. Confirm your DuckDNS token is correct and no other service uses port 443 on the Server computer. Restart using the HTTPS configuration."},
		"certificate":         {Title: "Finish the certificate check", Instruction: "After saving the router rule, open your public HTTPS address using cellular data. The first connection requests a trusted certificate. If a certificate warning appears, stop and check your DuckDNS address and the port-forwarding rule. Never bypass that warning. Then return here and check again."},
	}
	for _, id := range []string{"authorization-state", "viewer-signin", "public-https", "remote-policy", "listener"} {
		for _, check := range readiness.Checks {
			if check.ID == id && !check.Ready {
				steps = append(steps, fixes[id])
			}
		}
	}
	steps = append(steps,
		SetupStep{Title: "Find your Server in the router", Instruction: "While connected to home Wi-Fi, open your router's app or settings page. Find the computer or NAS running Kinosail in Connected devices. Reserve its local IP address so it stays the same; this may be called Address reservation or DHCP reservation. Use that computer's address below, not a container address."},
		SetupStep{Title: "Add one port-forwarding rule", Instruction: "Find Port forwarding in your router settings (sometimes called Virtual server or NAT forwarding). Add a custom rule using these values, then save it. These instructions do not change your router or firewall automatically.", Fields: []SetupField{
			{Label: "Name / service", Value: "Kinosail HTTPS"},
			{Label: "Device / internal IP", Value: "Your Server computer's reserved local IP"},
			{Label: "Protocol", Value: "TCP only"},
			{Label: "External / public port", Value: "443"},
			{Label: "Internal / private port", Value: "443"},
		}, Links: routerGuides()},
		SetupStep{Title: "Allow that port on the Server", Instruction: "If the Server computer has a firewall, allow inbound TCP 443 only. Keep the firewall on. Do not expose the LAN port, TCP 80, file sharing, or router administration. Do not use DMZ or open a range of ports. If asked for a start and end port, enter 443 in both. With the standard installation, internal port 443 reaches container 8443 automatically; do not forward to host 8443."},
	)
	for _, check := range readiness.Checks {
		if check.ID == "certificate" && !check.Ready {
			steps = append(steps, fixes[check.ID])
		}
	}
	return append(steps, SetupStep{Title: "Try it away from home", Instruction: "Turn Wi-Fi off on your phone. Open your public HTTPS address in a browser and choose Get a sign-in code. In Kinosail or a compatible Jellyfin app, add that address and choose Quick Connect. On your signed-in home device, open Quick Connect as the remote-enabled Viewer, verify the device and code, then approve it. If asked, sign in locally again with your password and authenticator code. Never approve an unexpected code. Play a movie and try seeking. You can also use a passkey already registered for the public address. Home Wi-Fi alone does not prove outside access. Return here and choose Check again to refresh the Server checks."})
}

func routerGuides() []SetupLink {
	return []SetupLink{
		{Label: "TP-Link routers", URL: "https://www.tp-link.com/us/support/faq/1379/"},
		{Label: "ASUS routers", URL: "https://www.asus.com/us/support/faq/1037906/"},
		{Label: "NETGEAR routers", URL: "https://kb.netgear.com/24289/How-do-I-set-up-port-forwarding-to-a-local-server-on-my-NETGEAR-router"},
		{Label: "eero", URL: "https://eero.com/support/articles/how-do-i-set-up-port-forwarding"},
		{Label: "Google Nest Wifi / Google Wifi", URL: "https://support.google.com/googlehome/answer/6274503?hl=en"},
		{Label: "Xfinity Gateway", URL: "https://www.xfinity.com/support/articles/xfi-port-forwarding"},
	}
}

func publicSetupURL(hostname string) string {
	label, found := strings.CutSuffix(hostname, ".duckdns.org")
	if !found || !validDomain(label) {
		return ""
	}
	return "https://" + hostname
}

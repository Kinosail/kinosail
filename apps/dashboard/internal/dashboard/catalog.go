package dashboard

// CatalogEntry provides safe defaults without controlling a user's address.
type CatalogEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Icon        string `json:"icon"`
	Accent      string `json:"accent"`
}

var catalog = []CatalogEntry{
	{"jellyfin", "Jellyfin", "Movies, shows, music, and books", "Media", "play", "lime"},
	{"plex", "Plex", "Personal media server", "Media", "play", "amber"},
	{"jellyseerr", "Jellyseerr", "Media requests and discovery", "Media", "search", "coral"},
	{"emby", "Emby", "Personal media server", "Media", "play", "ocean"},
	{"immich", "Immich", "Private photos and videos", "Photos", "image", "coral"},
	{"audiobookshelf", "Audiobookshelf", "Audiobooks and podcasts", "Media", "headphones", "violet"},
	{"sonarr", "Sonarr", "Television library automation", "Automation", "tv", "sky"},
	{"radarr", "Radarr", "Movie library automation", "Automation", "film", "amber"},
	{"lidarr", "Lidarr", "Music library automation", "Automation", "music", "lime"},
	{"prowlarr", "Prowlarr", "Indexer management", "Automation", "search", "ocean"},
	{"bazarr", "Bazarr", "Subtitle automation", "Automation", "captions", "sky"},
	{"sabnzbd", "SABnzbd", "Usenet download manager", "Downloads", "download", "amber"},
	{"qbittorrent", "qBittorrent", "BitTorrent client", "Downloads", "download", "ocean"},
	{"listenarr", "Listenarr", "Audiobook automation", "Automation", "headphones", "violet"},
	{"profilarr", "Profilarr", "Arr profile management", "Automation", "settings", "slate"},
	{"home-assistant", "Home Assistant", "Household automation", "Home", "home", "sky"},
	{"homebridge", "Homebridge", "Smart-home device bridge", "Home", "home", "amber"},
	{"matter-server", "Matter Server", "Matter device connectivity", "Home", "boxes", "ocean"},
	{"otbr", "OTBR", "Thread border router", "Home", "network", "lime"},
	{"pihole", "Pi-hole", "Network-wide DNS filtering", "Network", "shield", "coral"},
	{"adguard", "AdGuard Home", "Network-wide DNS filtering", "Network", "shield", "lime"},
	{"unifi", "UniFi", "Network management", "Network", "network", "sky"},
	{"protect", "Protect", "Security camera management", "Network", "shield", "coral"},
	{"unifi-site-manager", "UniFi Site Manager", "Remote UniFi management", "Network", "cloud", "ocean"},
	{"proxmox", "Proxmox", "Virtualization environment", "Infrastructure", "server", "amber"},
	{"truenas", "TrueNAS", "Network storage", "Storage", "database", "ocean"},
	{"synology", "Synology", "Network storage", "Storage", "database", "sky"},
	{"nas", "NAS", "Network storage", "Storage", "database", "slate"},
	{"grafana", "Grafana", "Operational dashboards", "Monitoring", "chart", "amber"},
	{"uptime-kuma", "Uptime Kuma", "Service monitoring", "Monitoring", "pulse", "lime"},
	{"portainer", "Portainer", "Container management", "Infrastructure", "boxes", "sky"},
	{"paperless", "Paperless-ngx", "Searchable document archive", "Documents", "file", "slate"},
	{"nextcloud", "Nextcloud", "Files, calendar, and collaboration", "Productivity", "cloud", "ocean"},
	{"vaultwarden", "Vaultwarden", "Password vault", "Security", "lock", "violet"},
	{"arcane", "Arcane", "Container management", "Infrastructure", "boxes", "coral"},
	{"homarr", "Homarr", "Home server dashboard", "Productivity", "grid", "lime"},
	{"hermes", "Hermes", "Household service", "Productivity", "app", "violet"},
	{"kinosail-player", "Kinosail Player", "Household media", "Kinosail", "play", "lime"},
	{"kinosail-subtitles", "Kinosail Subtitles", "Subtitle management", "Kinosail", "captions", "sky"},
	{"kinosail-dashboard", "Kinosail Dashboard", "Household app launcher", "Kinosail", "grid", "coral"},
}

var catalogAliases = map[string][]string{
	"jellyfin": {"media", "movies", "shows", "streaming"}, "plex": {"media", "movies", "shows", "streaming"}, "jellyseerr": {"requests", "media"}, "emby": {"media", "movies", "shows", "streaming"},
	"immich": {"photos", "pictures", "camera"}, "audiobookshelf": {"audiobooks", "podcasts", "books"}, "sonarr": {"tv", "series", "shows"}, "radarr": {"movies", "films"}, "lidarr": {"music", "albums"}, "prowlarr": {"indexers", "search"}, "bazarr": {"subtitles", "captions"}, "sabnzbd": {"downloads", "usenet"}, "qbittorrent": {"downloads", "torrents"},
	"listenarr": {"audiobooks", "books"}, "profilarr": {"profiles", "arr"}, "home-assistant": {"ha", "smart home", "automation"}, "homebridge": {"smart home", "homekit"}, "matter-server": {"smart home", "matter"}, "otbr": {"thread", "router", "smart home"}, "pihole": {"dns", "ad blocking", "network"}, "adguard": {"dns", "ad blocking", "network"},
	"unifi": {"wifi", "router", "network"}, "protect": {"cameras", "security", "unifi"}, "unifi-site-manager": {"unifi", "network", "wifi"}, "proxmox": {"virtual machines", "vms", "containers"}, "truenas": {"nas", "storage", "files"}, "synology": {"nas", "storage", "files"}, "nas": {"storage", "files", "server"}, "grafana": {"metrics", "monitoring", "charts"}, "uptime-kuma": {"monitoring", "uptime", "status"}, "portainer": {"docker", "containers"}, "paperless": {"documents", "scanner", "files"}, "nextcloud": {"files", "calendar", "cloud"}, "vaultwarden": {"passwords", "bitwarden", "security"}, "arcane": {"docker", "containers"}, "homarr": {"dashboard", "home lab"}, "hermes": {"household"},
	"kinosail-player": {"media", "movies", "shows"}, "kinosail-subtitles": {"subtitles", "captions"}, "kinosail-dashboard": {"dashboard", "launcher", "kinosail"},
}

// Catalog returns an isolated copy of the built-in application catalog.
func Catalog() []CatalogEntry { return append([]CatalogEntry(nil), catalog...) }

// CatalogAliases returns an isolated alias index for fast household search.
func CatalogAliases() map[string][]string {
	result := make(map[string][]string, len(catalogAliases))
	for id, aliases := range catalogAliases {
		result[id] = append([]string(nil), aliases...)
	}
	return result
}

func catalogByID(id string) (CatalogEntry, bool) {
	for _, entry := range catalog {
		if entry.ID == id {
			return entry, true
		}
	}
	return CatalogEntry{}, false
}

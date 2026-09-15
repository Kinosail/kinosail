package server

import sharedjellyfin "github.com/MikeO7/kinosail/packages/jellyfincompat"

func jellyfinShowID(name string) string { return sharedjellyfin.ShowID(name) }

func jellyfinRawID(id string) string { return sharedjellyfin.RawID(id) }

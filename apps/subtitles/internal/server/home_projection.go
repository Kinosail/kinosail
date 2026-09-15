package server

import (
	"net/http"

	homeview "github.com/MikeO7/kinosail/packages/home"
)

func (source homeSource) Shell(request *http.Request, view string) homeview.Shell {
	viewer := currentViewer(request)
	primary, more := source.settings.navigationLinks(view, preferredLanguage(request))
	return homeview.NewShell(source.settings.serverName(), viewer.Owner, viewer.Name, viewer.ID, primary, more, source.updates != nil && source.updates.Available())
}

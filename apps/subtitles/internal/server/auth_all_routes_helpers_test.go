package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/scim"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func init() {
	for _, pattern := range scim.Patterns() {
		explicitlyAnonymousRoutes[pattern] = true
	}
}

var (
	newRouteAuthorizationServer = routeAuthorizationFixture.Server
	routeTestTOTP               = servertest.CurrentTOTP
	loginRouteProfile           = servertest.LoginRouteProfile
	loginRouteCookie            = routeAuthorizationFixture.Cookie
	exerciseRouteWithCookie     = routeAuthorizationFixture.ExerciseCookie
	createRouteProfile          = servertest.CreateRouteProfile
	createRouteAPIKey           = servertest.CreateRouteAPIKey
	routeJSON                   = servertest.RouteJSON
	concreteRoutePath           = servertest.ConcreteRoutePath
)

func registeredRouteInventory(t *testing.T) []string {
	t.Helper()
	removed := map[string]bool{}
	for _, route := range removedRoutePatterns(t) {
		removed[route] = true
	}
	var routes []string
	for _, route := range servertest.RegisteredRouteInventory(t) {
		if !removed[route] {
			routes = append(routes, route)
		}
	}
	return routes
}

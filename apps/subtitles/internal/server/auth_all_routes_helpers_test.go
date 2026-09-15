package server

import (
	"github.com/MikeO7/kinosail/packages/scim"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func init() {
	for _, pattern := range scim.Patterns() {
		explicitlyAnonymousRoutes[pattern] = true
	}
}

var (
	registeredRouteInventory    = servertest.RegisteredRouteInventory
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

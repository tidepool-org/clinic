package api

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func RouteSkipper(routes []string) middleware.Skipper {
	routesMap := map[string]struct{}{}
	for _, route := range routes {
		routesMap[route] = struct{}{}
	}

	return func(ec echo.Context) bool {
		_, ok := routesMap[ec.Path()]
		return ok
	}
}

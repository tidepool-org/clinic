package api

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"strings"
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

func PathPrefixSkipper(prefix string) middleware.Skipper {
	return func(ec echo.Context) bool {
		return strings.HasPrefix(ec.Path(), prefix)
	}
}

func PathSuffixSkipper(prefix string) middleware.Skipper {
	return func(ec echo.Context) bool {
		return strings.HasSuffix(ec.Path(), prefix)
	}
}

func AnySkipper(skippers ...middleware.Skipper) middleware.Skipper {
	return func(ec echo.Context) bool {
		for _, skipper := range skippers {
			if skipper(ec) {
				return true
			}
		}
		return false
	}
}

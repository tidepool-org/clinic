package errors

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

// CustomHTTPErrorHandler responds with the status code of an [HttpError], and otherwise
// as echo does by default. A status that doesn't allow a body, such as that of
// [NoChange], is sent without one: echo's default handler would write the message as
// JSON, which net/http refuses, and echo would then log that as an error.
func CustomHTTPErrorHandler(err error, c echo.Context) {
	e := HttpError{}
	if errors.As(err, &e) {
		if !bodyAllowed(e.Code) {
			if !c.Response().Committed {
				if err := c.NoContent(e.Code); err != nil {
					c.Echo().Logger.Error(err)
				}
			}
			return
		}
		c.Echo().DefaultHTTPErrorHandler(echo.NewHTTPError(e.Code, err.Error()), c)
		return
	}
	c.Echo().DefaultHTTPErrorHandler(err, c)
}

// bodyAllowed reports whether a response with the status code may have a body.
func bodyAllowed(code int) bool {
	return code >= http.StatusOK && code != http.StatusNoContent &&
		code != http.StatusNotModified
}

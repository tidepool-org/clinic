package errors

import (
	"errors"
	"github.com/labstack/echo/v4"
)

func CustomHTTPErrorHandler(err error, c echo.Context) {
	e := HttpError{}
	if errors.As(err, &e) {
		c.Echo().DefaultHTTPErrorHandler(echo.NewHTTPError(e.Code, err.Error()), c)
		return
	}
	c.Echo().DefaultHTTPErrorHandler(err, c)
}

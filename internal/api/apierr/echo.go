package apierr

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// JSONInvalidParam writes a 400 INVALID_PARAM response to the client.
func JSONInvalidParam(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, InvalidParam())
}

// JSONInternalError writes a 500 INTERNAL_ERROR response to the client.
func JSONInternalError(c echo.Context) error {
	return c.JSON(http.StatusInternalServerError, InternalError())
}

// JSONNoSuchUser writes a 404 NO_SUCH_USER response to the client.
func JSONNoSuchUser(c echo.Context) error {
	return c.JSON(http.StatusNotFound, NoSuchUser())
}

// JSONNoSuchNote writes a 404 NO_SUCH_NOTE response to the client.
func JSONNoSuchNote(c echo.Context) error {
	return c.JSON(http.StatusNotFound, NoSuchNote())
}

// JSONAccessDenied writes a 403 ACCESS_DENIED response to the client.
func JSONAccessDenied(c echo.Context) error {
	return c.JSON(http.StatusForbidden, AccessDenied())
}

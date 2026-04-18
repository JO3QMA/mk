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

// JSONNoSuchRenoteTarget writes a 404 NO_SUCH_RENOTE_TARGET response to the client.
func JSONNoSuchRenoteTarget(c echo.Context) error {
	return c.JSON(http.StatusNotFound, NoSuchRenoteTarget())
}

// JSONNoSuchReplyTarget writes a 404 NO_SUCH_REPLY_TARGET response to the client.
func JSONNoSuchReplyTarget(c echo.Context) error {
	return c.JSON(http.StatusNotFound, NoSuchReplyTarget())
}

// JSONCannotReplyToAnInvisibleNote writes a 403 CANNOT_REPLY_TO_AN_INVISIBLE_NOTE response.
func JSONCannotReplyToAnInvisibleNote(c echo.Context) error {
	return c.JSON(http.StatusForbidden, CannotReplyToAnInvisibleNote())
}

// JSONCannotRenoteDueToVisibility writes a 403 CANNOT_RENOTE_DUE_TO_VISIBILITY response.
func JSONCannotRenoteDueToVisibility(c echo.Context) error {
	return c.JSON(http.StatusForbidden, CannotRenoteDueToVisibility())
}

// JSONNoSuchChannel writes a 404 NO_SUCH_CHANNEL response to the client.
func JSONNoSuchChannel(c echo.Context) error {
	return c.JSON(http.StatusNotFound, NoSuchChannel())
}

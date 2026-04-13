// Package apierr provides Misskey-compatible error response helpers
// shared across all API handlers.
//
// All error responses follow the format:
//
//	{"error": {"message": ..., "code": ..., "id": ...}}
//
// Frequently used errors have canonical UUIDs to avoid drift between handlers.
package apierr

// Error returns a Misskey-compatible error response map.
// The returned map is safe to pass to echo.Context.JSON.
func Error(code, message, id string) map[string]any {
	return map[string]any{
		"error": map[string]any{
			"message": message,
			"code":    code,
			"id":      id,
		},
	}
}

// Canonical UUIDs for frequently-used error codes.
// Any handler returning these codes should use the constants to prevent drift.
const (
	UUIDInvalidParam  = "3d81ceae-475f-4600-b2a8-2bc116157532"
	UUIDInternalError = "5d37dbcb-891e-41ca-a3d6-e690c97775ac"
	UUIDNoSuchNote    = "24fcbfc6-2e37-42b6-8388-c29b3272571530"
	UUIDNoSuchUser    = "4362f8dc-731f-4ad8-a694-be5a88922a24"
	UUIDAccessDenied  = "1fb7cb09-d46a-4fff-b8df-057708cce513"
)

// InvalidParam returns a 400 INVALID_PARAM error response.
func InvalidParam() map[string]any {
	return Error("INVALID_PARAM", "Invalid param.", UUIDInvalidParam)
}

// InternalError returns a 500 INTERNAL_ERROR error response.
func InternalError() map[string]any {
	return Error("INTERNAL_ERROR", "Internal error.", UUIDInternalError)
}

// NoSuchNote returns a 404 NO_SUCH_NOTE error response.
func NoSuchNote() map[string]any {
	return Error("NO_SUCH_NOTE", "No such note.", UUIDNoSuchNote)
}

// NoSuchUser returns a 404 NO_SUCH_USER error response.
func NoSuchUser() map[string]any {
	return Error("NO_SUCH_USER", "No such user.", UUIDNoSuchUser)
}

// AccessDenied returns a 403 ACCESS_DENIED error response.
func AccessDenied() map[string]any {
	return Error("ACCESS_DENIED", "Access denied.", UUIDAccessDenied)
}

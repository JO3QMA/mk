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
// UUIDs are sourced from third_party/misskey/packages/backend/src/server/api/endpoints/
// and must match the upstream Misskey implementation so that clients can identify
// errors by their `id` field.
const (
	UUIDInvalidParam  = "3d81ceae-475f-4600-b2a8-2bc116157532"
	UUIDInternalError = "5d37dbcb-891e-41ca-a3d6-e690c97775ac"
	UUIDNoSuchNote    = "24fcbfc6-2e37-42b6-8388-c29b3861a08d"
	UUIDNoSuchUser    = "4362f8dc-731f-4ad8-a694-be5a88922a24"
	UUIDAccessDenied  = "1fb7cb09-d46a-4fff-b8df-057708cce513"

	// UUIDs for notes/create errors (third_party/misskey/.../endpoints/notes/create.ts).
	UUIDNoSuchRenoteTarget                                         = "b5c90186-4ab0-49c8-9bba-a1f76c282ba4"
	UUIDCannotRenoteToAPureRenote                                  = "fd4cc33e-2a37-48dd-99cc-9b806eb2031a"
	UUIDCannotRenoteDueToVisibility                                = "be9529e9-fe72-4de0-ae43-0b363c4938af"
	UUIDNoSuchReplyTarget                                          = "749ee0f6-d3da-459a-bf02-282e2da4292c"
	UUIDCannotReplyToAnInvisibleNote                               = "b98980fa-3780-406c-a935-b6d0eeee10d1"
	UUIDCannotReplyToAPureRenote                                   = "3ac74a84-8fd5-4bb0-870f-01804f82ce15"
	UUIDCannotReplyToSpecifiedVisibilityNoteWithExtendedVisibility = "ed940410-535c-4d5e-bfa3-af798671e93c"
	UUIDCannotCreateAlreadyExpiredPoll                             = "04da457d-b083-4055-9082-955525eda5a5"
	UUIDNoSuchChannel                                              = "b1653923-5453-4edc-b786-7c4f39bb0bbb"
	UUIDYouHaveBeenBlocked                                         = "b390d7e1-8a5e-46ed-b625-06271cafd3d3"
	UUIDNoSuchFile                                                 = "b6992544-63e7-67f0-fa7f-32444b1b5306"
	UUIDCannotRenoteOutsideOfChannel                               = "33510210-8452-094c-6227-4a6c05d99f00"
	UUIDContainsProhibitedWords                                    = "aa6e01d3-a85c-669d-758a-76aab43af334"
	UUIDContainsTooManyMentions                                    = "4de0363a-3046-481b-9b0f-feff3e211025"

	// UUID for users/show (third_party/misskey/.../endpoints/users/show.ts).
	UUIDFailedToResolveRemoteUser = "ef7b9be4-9cba-4e6f-ab41-90ed171c7d3c"
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

// NoSuchRenoteTarget returns a 404 NO_SUCH_RENOTE_TARGET error response.
func NoSuchRenoteTarget() map[string]any {
	return Error("NO_SUCH_RENOTE_TARGET", "No such renote target.", UUIDNoSuchRenoteTarget)
}

// NoSuchReplyTarget returns a 404 NO_SUCH_REPLY_TARGET error response.
func NoSuchReplyTarget() map[string]any {
	return Error("NO_SUCH_REPLY_TARGET", "No such reply target.", UUIDNoSuchReplyTarget)
}

// CannotReplyToAnInvisibleNote returns a 403 CANNOT_REPLY_TO_AN_INVISIBLE_NOTE error response.
func CannotReplyToAnInvisibleNote() map[string]any {
	return Error("CANNOT_REPLY_TO_AN_INVISIBLE_NOTE", "You cannot reply to an invisible Note.", UUIDCannotReplyToAnInvisibleNote)
}

// CannotRenoteDueToVisibility returns a 403 CANNOT_RENOTE_DUE_TO_VISIBILITY error response.
func CannotRenoteDueToVisibility() map[string]any {
	return Error("CANNOT_RENOTE_DUE_TO_VISIBILITY", "You can not Renote due to target visibility.", UUIDCannotRenoteDueToVisibility)
}

// NoSuchChannel returns a 404 NO_SUCH_CHANNEL error response.
func NoSuchChannel() map[string]any {
	return Error("NO_SUCH_CHANNEL", "No such channel.", UUIDNoSuchChannel)
}

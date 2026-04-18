package apierr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError_Structure(t *testing.T) {
	result := Error("INVALID_PARAM", "Invalid.", "abc-123")
	errObj, ok := result["error"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "Invalid.", errObj["message"])
	assert.Equal(t, "INVALID_PARAM", errObj["code"])
	assert.Equal(t, "abc-123", errObj["id"])
}

func TestInvalidParam(t *testing.T) {
	result := InvalidParam()
	errObj := result["error"].(map[string]any)
	assert.Equal(t, "INVALID_PARAM", errObj["code"])
	assert.Equal(t, UUIDInvalidParam, errObj["id"])
}

func TestInternalError(t *testing.T) {
	result := InternalError()
	errObj := result["error"].(map[string]any)
	assert.Equal(t, "INTERNAL_ERROR", errObj["code"])
	assert.Equal(t, UUIDInternalError, errObj["id"])
}

func TestNoSuchNote(t *testing.T) {
	result := NoSuchNote()
	errObj := result["error"].(map[string]any)
	assert.Equal(t, "NO_SUCH_NOTE", errObj["code"])
	assert.Equal(t, UUIDNoSuchNote, errObj["id"])
}

func TestNoSuchUser(t *testing.T) {
	result := NoSuchUser()
	errObj := result["error"].(map[string]any)
	assert.Equal(t, "NO_SUCH_USER", errObj["code"])
	assert.Equal(t, UUIDNoSuchUser, errObj["id"])
}

func TestAccessDenied(t *testing.T) {
	result := AccessDenied()
	errObj := result["error"].(map[string]any)
	assert.Equal(t, "ACCESS_DENIED", errObj["code"])
	assert.Equal(t, UUIDAccessDenied, errObj["id"])
}

func TestNoSuchRenoteTarget(t *testing.T) {
	result := NoSuchRenoteTarget()
	errObj := result["error"].(map[string]any)
	assert.Equal(t, "NO_SUCH_RENOTE_TARGET", errObj["code"])
	assert.Equal(t, UUIDNoSuchRenoteTarget, errObj["id"])
}

func TestNoSuchReplyTarget(t *testing.T) {
	result := NoSuchReplyTarget()
	errObj := result["error"].(map[string]any)
	assert.Equal(t, "NO_SUCH_REPLY_TARGET", errObj["code"])
	assert.Equal(t, UUIDNoSuchReplyTarget, errObj["id"])
}

func TestCannotReplyToAnInvisibleNote(t *testing.T) {
	result := CannotReplyToAnInvisibleNote()
	errObj := result["error"].(map[string]any)
	assert.Equal(t, "CANNOT_REPLY_TO_AN_INVISIBLE_NOTE", errObj["code"])
	assert.Equal(t, UUIDCannotReplyToAnInvisibleNote, errObj["id"])
}

func TestCannotRenoteDueToVisibility(t *testing.T) {
	result := CannotRenoteDueToVisibility()
	errObj := result["error"].(map[string]any)
	assert.Equal(t, "CANNOT_RENOTE_DUE_TO_VISIBILITY", errObj["code"])
	assert.Equal(t, UUIDCannotRenoteDueToVisibility, errObj["id"])
}

func TestNoSuchChannel(t *testing.T) {
	result := NoSuchChannel()
	errObj := result["error"].(map[string]any)
	assert.Equal(t, "NO_SUCH_CHANNEL", errObj["code"])
	assert.Equal(t, UUIDNoSuchChannel, errObj["id"])
}

// TestUUIDConstants_MatchUpstream guards against drift between the Go
// constants and the upstream Misskey error UUIDs. The values on the right
// are copied verbatim from third_party/misskey/packages/backend/src/server/
// api/endpoints/.../meta.errors blocks and must never be edited without
// updating the corresponding upstream reference.
func TestUUIDConstants_MatchUpstream(t *testing.T) {
	cases := []struct {
		name     string
		got      string
		expected string
	}{
		{"NoSuchNote", UUIDNoSuchNote, "24fcbfc6-2e37-42b6-8388-c29b3861a08d"},
		{"NoSuchUser", UUIDNoSuchUser, "4362f8dc-731f-4ad8-a694-be5a88922a24"},
		{"NoSuchRenoteTarget", UUIDNoSuchRenoteTarget, "b5c90186-4ab0-49c8-9bba-a1f76c282ba4"},
		{"CannotRenoteToAPureRenote", UUIDCannotRenoteToAPureRenote, "fd4cc33e-2a37-48dd-99cc-9b806eb2031a"},
		{"CannotRenoteDueToVisibility", UUIDCannotRenoteDueToVisibility, "be9529e9-fe72-4de0-ae43-0b363c4938af"},
		{"NoSuchReplyTarget", UUIDNoSuchReplyTarget, "749ee0f6-d3da-459a-bf02-282e2da4292c"},
		{"CannotReplyToAnInvisibleNote", UUIDCannotReplyToAnInvisibleNote, "b98980fa-3780-406c-a935-b6d0eeee10d1"},
		{"CannotReplyToAPureRenote", UUIDCannotReplyToAPureRenote, "3ac74a84-8fd5-4bb0-870f-01804f82ce15"},
		{"CannotReplyToSpecifiedVisibilityNoteWithExtendedVisibility", UUIDCannotReplyToSpecifiedVisibilityNoteWithExtendedVisibility, "ed940410-535c-4d5e-bfa3-af798671e93c"},
		{"CannotCreateAlreadyExpiredPoll", UUIDCannotCreateAlreadyExpiredPoll, "04da457d-b083-4055-9082-955525eda5a5"},
		{"NoSuchChannel", UUIDNoSuchChannel, "b1653923-5453-4edc-b786-7c4f39bb0bbb"},
		{"YouHaveBeenBlocked", UUIDYouHaveBeenBlocked, "b390d7e1-8a5e-46ed-b625-06271cafd3d3"},
		{"NoSuchFile", UUIDNoSuchFile, "b6992544-63e7-67f0-fa7f-32444b1b5306"},
		{"CannotRenoteOutsideOfChannel", UUIDCannotRenoteOutsideOfChannel, "33510210-8452-094c-6227-4a6c05d99f00"},
		{"ContainsProhibitedWords", UUIDContainsProhibitedWords, "aa6e01d3-a85c-669d-758a-76aab43af334"},
		{"ContainsTooManyMentions", UUIDContainsTooManyMentions, "4de0363a-3046-481b-9b0f-feff3e211025"},
		{"FailedToResolveRemoteUser", UUIDFailedToResolveRemoteUser, "ef7b9be4-9cba-4e6f-ab41-90ed171c7d3c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.got)
		})
	}
}

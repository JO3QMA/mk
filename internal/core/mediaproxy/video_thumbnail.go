package mediaproxy

import "strings"

// IsVideoMIME reports whether contentType is a video/* MIME type the proxy
// should attempt to extract a still frame from.
func IsVideoMIME(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(contentType), "video/")
}

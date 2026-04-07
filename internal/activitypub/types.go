// Package activitypub provides ActivityStreams types and the helpers for
// rendering local model entities into AP-compatible JSON-LD documents.
package activitypub

// ContextURL is the standard ActivityStreams 2.0 JSON-LD context.
const ContextURL = "https://www.w3.org/ns/activitystreams"

// SecurityContextURL is the W3C security vocabulary used for HTTP Signatures.
const SecurityContextURL = "https://w3id.org/security/v1"

// Public is the magic IRI used to denote a publicly addressable activity.
const Public = "https://www.w3.org/ns/activitystreams#Public"

// MimeType is the canonical Content-Type for ActivityPub objects.
const MimeType = `application/activity+json`

// LDMimeType is the more specific JSON-LD content type with profile parameter.
const LDMimeType = `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`

// Object is the base type for any AS object. 拡張するときは embed する。
type Object struct {
	Context any    `json:"@context,omitempty"`
	ID      string `json:"id,omitempty"`
	Type    string `json:"type,omitempty"`
	Name    string `json:"name,omitempty"`
}

// PublicKey is the embedded JSON-LD object describing a user's signing key.
type PublicKey struct {
	ID           string `json:"id"`
	Owner        string `json:"owner"`
	PublicKeyPEM string `json:"publicKeyPem"`
}

// Endpoints holds the endpoints sub-object for an Actor.
type Endpoints struct {
	SharedInbox string `json:"sharedInbox,omitempty"`
}

// Image is a generic ActivityStreams Image (used for icon/image fields).
type Image struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Person represents a user actor.
type Person struct {
	Object
	Inbox             string    `json:"inbox"`
	Outbox            string    `json:"outbox"`
	Followers         string    `json:"followers"`
	Following         string    `json:"following"`
	PreferredUsername string    `json:"preferredUsername"`
	Summary           string    `json:"summary,omitempty"`
	URL               string    `json:"url,omitempty"`
	Endpoints         Endpoints `json:"endpoints,omitzero"`
	PublicKey         PublicKey `json:"publicKey"`
	Icon              *Image    `json:"icon,omitempty"`
	ManuallyApproves  bool      `json:"manuallyApprovesFollowers,omitempty"`
	Discoverable      bool      `json:"discoverable,omitempty"`
}

// Note represents a note object (microblog post).
type Note struct {
	Object
	AttributedTo string   `json:"attributedTo"`
	Content      string   `json:"content"`
	Source       *Source  `json:"source,omitempty"`
	Published    string   `json:"published"`
	To           []string `json:"to"`
	CC           []string `json:"cc,omitempty"`
	InReplyTo    string   `json:"inReplyTo,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	Sensitive    bool     `json:"sensitive,omitempty"`
}

// Source represents the original markup of a note (Markdown / MFM).
type Source struct {
	Content   string `json:"content"`
	MediaType string `json:"mediaType"`
}

// Activity is the base type embedded by all activity types.
type Activity struct {
	Object
	Actor     string   `json:"actor"`
	Published string   `json:"published,omitempty"`
	To        []string `json:"to,omitempty"`
	CC        []string `json:"cc,omitempty"`
}

// Create wraps a Note (or other Object) inside a Create activity.
type Create struct {
	Activity
	Object any `json:"object"`
}

// Follow represents a Follow activity.
type Follow struct {
	Activity
	Object string `json:"object"` // followee actor URI
}

// Accept represents an Accept activity.
type Accept struct {
	Activity
	Object any `json:"object"`
}

// Reject represents a Reject activity.
type Reject struct {
	Activity
	Object any `json:"object"`
}

// Undo represents an Undo activity.
type Undo struct {
	Activity
	Object any `json:"object"`
}

// Delete represents a Delete activity.
type Delete struct {
	Activity
	Object any `json:"object"`
}

// Update represents an Update activity.
type Update struct {
	Activity
	Object any `json:"object"`
}

// Like represents a Like (reaction) activity.
type Like struct {
	Activity
	Object  string `json:"object"` // target note URI
	Content string `json:"content,omitempty"`
}

// Tombstone represents a deleted object placeholder used as the object of a
// Delete activity.
type Tombstone struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// Announce represents a Renote (boost) activity.
type Announce struct {
	Activity
	Object string `json:"object"` // target note URI
}

// AddContext attaches the standard AS+security context to any object that
// embeds Object. 配列で持つことで複数 vocabulary を表現する。
func AddContext(o any) {
	type ctxSetter interface {
		setContext(any)
	}
	switch v := o.(type) {
	case *Person:
		v.Context = []any{ContextURL, SecurityContextURL}
	case *Note:
		v.Context = ContextURL
	case *Create:
		v.Context = ContextURL
	case *Follow:
		v.Context = ContextURL
	case *Accept:
		v.Context = ContextURL
	case *Reject:
		v.Context = ContextURL
	case *Undo:
		v.Context = ContextURL
	case *Delete:
		v.Context = ContextURL
	case *Update:
		v.Context = ContextURL
	case *Like:
		v.Context = ContextURL
	case *Announce:
		v.Context = ContextURL
	}
}

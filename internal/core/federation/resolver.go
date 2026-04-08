// Package federation provides services for processing inbound and outbound
// ActivityPub traffic.
package federation

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// HTTPFetcher abstracts the HTTP client used for fetching remote AP objects.
// 実装は activitypub.Client か互換のテストダブル。署名なし fetch でも互換性は
// 保たれるため、ここでは accept ヘッダ指定だけ受け取る。
type HTTPFetcher interface {
	FetchObject(uri string) ([]byte, error)
}

// Errors returned by Resolver.
var (
	// ErrInvalidActor is returned when the fetched JSON cannot be parsed.
	ErrInvalidActor = errors.New("invalid actor document")
	// ErrInvalidNote is returned when a fetched Note cannot be parsed.
	ErrInvalidNote = errors.New("invalid note document")
)

// Resolver fetches remote actors / notes and persists them in the local
// user / note tables.
//
// 公開鍵は別キャッシュ (in-memory map) で actorID → PEM を保持する。永続化は
// 後続フェーズで user_keypair などに移す予定。
type Resolver struct {
	userRepo repository.UserRepository
	noteRepo repository.NoteRepository
	urls     *activitypub.URLBuilder
	fetcher  HTTPFetcher
	idGen    id.Generator
	keys     map[string]string // userID → publicKeyPEM
}

// NewResolver constructs a Resolver.
// noteRepo / urls はリモート Note の解決と取り込みに使用する。リモート Note
// 機能を使わない呼び出し側 (テスト等) では nil を渡してよい。
func NewResolver(
	userRepo repository.UserRepository,
	noteRepo repository.NoteRepository,
	urls *activitypub.URLBuilder,
	fetcher HTTPFetcher,
	idGen id.Generator,
) *Resolver {
	return &Resolver{
		userRepo: userRepo,
		noteRepo: noteRepo,
		urls:     urls,
		fetcher:  fetcher,
		idGen:    idGen,
		keys:     map[string]string{},
	}
}

// PublicKeyForActor returns the cached public key PEM for an actor ID.
func (r *Resolver) PublicKeyForActor(actorID string) (string, error) {
	key, ok := r.keys[actorID]
	if !ok {
		return "", fmt.Errorf("public key for actor %q not cached", actorID)
	}
	return key, nil
}

// ResolveActor returns the local model.User row for a remote actor URI,
// fetching and creating it if necessary.
func (r *Resolver) ResolveActor(uri string) (*model.User, error) {
	if existing, err := r.userRepo.FindByURI(uri); err == nil {
		// 既存ユーザーであっても publicKey キャッシュが空なら fetch しなおす。
		if _, cached := r.keys[existing.ID]; !cached {
			r.refreshPublicKey(existing.ID, uri)
		}
		return existing, nil
	}

	actor, err := r.fetchActor(uri)
	if err != nil {
		return nil, err
	}

	host, err := hostFromURI(actor.ID)
	if err != nil {
		return nil, ErrInvalidActor
	}

	now := time.Now()
	user := &model.User{
		ID:            r.idGen.Generate(now),
		Username:      actor.PreferredUsername,
		UsernameLower: strings.ToLower(actor.PreferredUsername),
		Host:          &host,
		URI:           &actor.ID,
		Inbox:         &actor.Inbox,
		LastFetchedAt: &now,
	}
	if name := actor.Name; name != "" {
		user.Name = &name
	}
	if actor.Endpoints.SharedInbox != "" {
		user.SharedInbox = &actor.Endpoints.SharedInbox
	}
	if err := r.userRepo.Create(user); err != nil {
		return nil, err
	}
	r.keys[user.ID] = actor.PublicKey.PublicKeyPEM
	return user, nil
}

// fetchActor fetches and decodes a remote actor document.
func (r *Resolver) fetchActor(uri string) (*activitypub.Person, error) {
	body, err := r.fetcher.FetchObject(uri)
	if err != nil {
		return nil, err
	}
	var actor activitypub.Person
	if err := json.Unmarshal(body, &actor); err != nil {
		return nil, ErrInvalidActor
	}
	if actor.ID == "" || actor.PreferredUsername == "" {
		return nil, ErrInvalidActor
	}
	return &actor, nil
}

// refreshPublicKey fetches the actor again and caches its public key. エラー
// は呼び出し側でログするだけで上には伝搬しない。
func (r *Resolver) refreshPublicKey(userID, uri string) {
	actor, err := r.fetchActor(uri)
	if err != nil {
		return
	}
	r.keys[userID] = actor.PublicKey.PublicKeyPEM
}

// ResolveActorByKeyID resolves an actor based on the keyId fragment URI.
func (r *Resolver) ResolveActorByKeyID(keyID string) (*model.User, error) {
	return r.ResolveActor(activitypub.ResolveKeyURL(keyID))
}

// ResolveNote returns the local model.Note row for a remote (or local) note
// URI, fetching and creating it if necessary.
//
//   - ローカル URI (urls.NoteURI のプレフィックスに一致) の場合は ID を抽出して
//     noteRepo.FindByID を試みる
//   - リモート URI で既に取り込み済みなら noteRepo.FindByURI で返す
//   - 未知のリモート URI なら fetcher で取得して IngestNote で永続化
func (r *Resolver) ResolveNote(uri string) (*model.Note, error) {
	if r.noteRepo == nil {
		return nil, ErrInvalidNote
	}
	if id := r.extractLocalNoteID(uri); id != "" {
		if existing, err := r.noteRepo.FindByID(id); err == nil {
			return existing, nil
		}
		// ローカル URI なのにDBに無ければ、これ以上 fetch しても意味がない
		return nil, ErrInvalidNote
	}
	if existing, err := r.noteRepo.FindByURI(uri); err == nil {
		return existing, nil
	}
	body, err := r.fetcher.FetchObject(uri)
	if err != nil {
		return nil, err
	}
	return r.IngestNote(body)
}

// IngestNote parses an ActivityStreams Note JSON and persists it as a local
// row authored by the (resolved) attributedTo actor. 既に同じ URI の note を
// 取り込み済みなら既存レコードを返す。
func (r *Resolver) IngestNote(body []byte) (*model.Note, error) {
	if r.noteRepo == nil {
		return nil, ErrInvalidNote
	}
	var apNote activitypub.Note
	if err := json.Unmarshal(body, &apNote); err != nil {
		return nil, ErrInvalidNote
	}
	if apNote.ID == "" || apNote.AttributedTo == "" {
		return nil, ErrInvalidNote
	}
	if existing, err := r.noteRepo.FindByURI(apNote.ID); err == nil {
		return existing, nil
	}
	actor, err := r.ResolveActor(apNote.AttributedTo)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	noteURI := apNote.ID
	note := &model.Note{
		ID:         r.idGen.Generate(now),
		UserID:     actor.ID,
		UserHost:   actor.Host,
		URI:        &noteURI,
		Visibility: deriveVisibility(apNote.To, apNote.CC),
	}
	if apNote.Content != "" {
		text := apNote.Content
		note.Text = &text
	}
	if apNote.Summary != "" {
		summary := apNote.Summary
		note.CW = &summary
	}
	if apNote.Sensitive && note.CW == nil {
		// Sensitive かつ Summary が無いケース: 空文字 CW を設定して NSFW を表現
		empty := ""
		note.CW = &empty
	}
	// 返信先がローカルに存在すれば紐付ける。リモート返信先の解決は後続 phase で
	// 対応するため、現状では nil のままにする。
	if apNote.InReplyTo != "" {
		if id := r.extractLocalNoteID(apNote.InReplyTo); id != "" {
			if reply, err := r.noteRepo.FindByID(id); err == nil {
				note.ReplyID = &reply.ID
				note.ReplyUserID = &reply.UserID
				note.ReplyUserHost = reply.UserHost
			}
		} else if reply, err := r.noteRepo.FindByURI(apNote.InReplyTo); err == nil {
			note.ReplyID = &reply.ID
			note.ReplyUserID = &reply.UserID
			note.ReplyUserHost = reply.UserHost
		}
	}
	if err := r.noteRepo.Create(note); err != nil {
		return nil, err
	}
	return note, nil
}

// extractLocalNoteID returns the trailing note ID for a URI rooted at the
// local instance, or "" if the URI does not match the local pattern.
func (r *Resolver) extractLocalNoteID(uri string) string {
	if r.urls == nil {
		return ""
	}
	prefix := r.urls.NoteURI("")
	if !strings.HasPrefix(uri, prefix) {
		return ""
	}
	rest := uri[len(prefix):]
	// /notes/{id} の "/{id}" 部分しか持たないように、追加のスラッシュは無視する
	if i := strings.Index(rest, "/"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

// deriveVisibility maps an AS to/cc audience pair to a Misskey visibility.
//
//   - to に Public があれば public
//   - cc に Public があり to に followers があれば home
//   - to に followers のみ (Public 無し) なら followers
//   - それ以外 (specific actor 列挙) は specified
func deriveVisibility(to, cc []string) model.NoteVisibility {
	hasFollowers := func(list []string) bool {
		for _, v := range list {
			if strings.HasSuffix(v, "/followers") {
				return true
			}
		}
		return false
	}
	if slices.Contains(to, activitypub.Public) {
		return model.NoteVisibilityPublic
	}
	if slices.Contains(cc, activitypub.Public) && hasFollowers(to) {
		return model.NoteVisibilityHome
	}
	if hasFollowers(to) {
		return model.NoteVisibilityFollowers
	}
	return model.NoteVisibilitySpecified
}

// hostFromURI extracts the host portion from an actor URI.
func hostFromURI(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", fmt.Errorf("missing host in %q", uri)
	}
	return u.Host, nil
}

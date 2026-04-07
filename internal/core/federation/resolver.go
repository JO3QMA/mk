// Package federation provides services for processing inbound and outbound
// ActivityPub traffic.
package federation

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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
	FetchActor(uri string) ([]byte, error)
}

// Errors returned by Resolver.
var (
	// ErrInvalidActor is returned when the fetched JSON cannot be parsed.
	ErrInvalidActor = errors.New("invalid actor document")
)

// Resolver fetches remote actors and persists them in the local user table.
//
// 公開鍵は別キャッシュ (in-memory map) で actorID → PEM を保持する。永続化は
// 後続フェーズで user_keypair などに移す予定。
type Resolver struct {
	userRepo repository.UserRepository
	fetcher  HTTPFetcher
	idGen    id.Generator
	keys     map[string]string // userID → publicKeyPEM
}

// NewResolver constructs a Resolver.
func NewResolver(userRepo repository.UserRepository, fetcher HTTPFetcher, idGen id.Generator) *Resolver {
	return &Resolver{userRepo: userRepo, fetcher: fetcher, idGen: idGen, keys: map[string]string{}}
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
	body, err := r.fetcher.FetchActor(uri)
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

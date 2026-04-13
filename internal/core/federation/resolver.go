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

	"log/slog"

	"github.com/shiroha-a/mk/internal/activitypub"
	corenote "github.com/shiroha-a/mk/internal/core/note"
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

// InstanceTracker is an interface used by Resolver to register hosts as soon
// as a remote user is fetched. パッケージ間の循環依存を避けるため interface で
// 受け取る (実装は core/instance.Service)。
type InstanceTracker interface {
	RegisterFromHost(host string) (*model.Instance, error)
}

// ChartHook is invoked after a remote user has been freshly created so
// the chart subsystem can record the new-user event into UsersChart and
// InstanceChart. パッケージ間の循環依存を避けるため interface で受け取る
// (実装は core/chart/charthook)。
type ChartHook interface {
	OnRemoteUserCreated(user *model.User)
}

// Errors returned by Resolver.
var (
	// ErrInvalidActor is returned when the fetched JSON cannot be parsed.
	ErrInvalidActor = errors.New("invalid actor document")
	// ErrInvalidNote is returned when a fetched Note cannot be parsed.
	ErrInvalidNote = errors.New("invalid note document")
)

// DefaultActorTTL is the default duration after which a cached actor (and its
// public key) is considered stale and refetched on next access.
const DefaultActorTTL = 24 * time.Hour

// publicKeyEntry stores a cached PEM together with the time it was fetched so
// the resolver can detect TTL expiry.
type publicKeyEntry struct {
	pem       string
	fetchedAt time.Time
}

// PublickeyStore abstracts persistence of remote actor public keys. Resolver
// uses this to fall back to the database when the in-memory cache misses.
type PublickeyStore interface {
	Upsert(pk *model.UserPublickey) error
	FindByUserID(userID string) (*model.UserPublickey, error)
}

// Resolver fetches remote actors / notes and persists them in the local
// user / note tables.
//
// 公開鍵は in-memory + DB (user_publickey テーブル) の二段キャッシュで管理
// する。エントリは actorTTL を超えると miss として扱い、次回 ResolveActor 時
// にリフレッシュされる。
type Resolver struct {
	userRepo        repository.UserRepository
	noteRepo        repository.NoteRepository
	urls            *activitypub.URLBuilder
	fetcher         HTTPFetcher
	idGen           id.Generator
	keys            map[string]publicKeyEntry // userID → publicKey + fetchedAt
	clock           func() time.Time          // テストで差し替える時計
	actorTTL        time.Duration             // アクター情報の最大寿命
	instanceTracker InstanceTracker           // optional: ホスト発見を通知
	chartHook       ChartHook                 // optional: 新規 remote user の集計
	publickeyRepo   PublickeyStore            // optional: 公開鍵の永続化
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
		keys:     map[string]publicKeyEntry{},
		clock:    time.Now,
		actorTTL: DefaultActorTTL,
	}
}

// SetClock replaces the clock used for TTL checks. Intended for tests.
func (r *Resolver) SetClock(now func() time.Time) {
	if now != nil {
		r.clock = now
	}
}

// SetActorTTL overrides the actor TTL used for cache freshness checks.
// 0 以下を渡した場合は変更しない。
func (r *Resolver) SetActorTTL(d time.Duration) {
	if d > 0 {
		r.actorTTL = d
	}
}

// SetInstanceTracker attaches an InstanceTracker that will be notified each
// time a remote actor is fetched (created or refreshed). nil 渡しは無効化と
// 同義。
func (r *Resolver) SetInstanceTracker(t InstanceTracker) {
	r.instanceTracker = t
}

// SetChartHook attaches a ChartHook invoked after a remote user has
// been freshly created. nil 渡しは無効化と同義。
func (r *Resolver) SetChartHook(h ChartHook) {
	r.chartHook = h
}

// SetPublickeyRepo attaches a PublickeyStore for persistent public key
// storage. nil 渡しは無効化と同義 (in-memory only に戻る)。
func (r *Resolver) SetPublickeyRepo(repo PublickeyStore) {
	r.publickeyRepo = repo
}

// PublicKeyForActor returns the cached public key PEM for an actor ID.
// in-memory → DB → miss の順で探索する。TTL超過は miss として扱い、呼び出し
// 側が ResolveActor を再実行することで refresh をトリガできる。
func (r *Resolver) PublicKeyForActor(actorID string) (string, error) {
	// 1. in-memory cache (TTL内)
	if entry, ok := r.keys[actorID]; ok {
		if r.clock().Sub(entry.fetchedAt) <= r.actorTTL {
			return entry.pem, nil
		}
		delete(r.keys, actorID)
	}
	// 2. DB fallback
	if r.publickeyRepo != nil {
		if pk, err := r.publickeyRepo.FindByUserID(actorID); err == nil {
			r.keys[actorID] = publicKeyEntry{pem: pk.KeyPEM, fetchedAt: r.clock()}
			return pk.KeyPEM, nil
		}
	}
	return "", fmt.Errorf("public key for actor %q not cached", actorID)
}

// ResolveActor returns the local model.User row for a remote actor URI,
// fetching and creating it if necessary. 既存ユーザーであっても LastFetchedAt
// が actorTTL を超えていたら fetch しなおして name / inbox / sharedInbox /
// publicKey を更新する。fetch 失敗時はベストエフォートで既存値を返す。
func (r *Resolver) ResolveActor(uri string) (*model.User, error) {
	if existing, err := r.userRepo.FindByURI(uri); err == nil {
		if r.shouldRefreshActor(existing) {
			r.refreshActor(existing, uri)
		} else if _, cached := r.keys[existing.ID]; !cached {
			// TTL 内であっても publicKey キャッシュが空 (再起動直後など) なら
			// 取り直す。
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

	now := r.clock()
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
	if actor.Featured != "" {
		user.Featured = &actor.Featured
	}
	if actor.MovedTo != "" {
		user.MovedToURI = &actor.MovedTo
		user.MovedAt = &now
	}
	if len(actor.AlsoKnownAs) > 0 {
		aka := strings.Join(actor.AlsoKnownAs, ",")
		user.AlsoKnownAs = &aka
	}
	if err := r.userRepo.Create(user); err != nil {
		return nil, err
	}
	r.cachePublicKey(user.ID, actor.PublicKey.ID, actor.PublicKey.PublicKeyPEM)
	r.notifyInstance(host)
	if r.chartHook != nil {
		r.chartHook.OnRemoteUserCreated(user)
	}
	return user, nil
}

// ForceResolveActor resolves an actor and always re-fetches the profile,
// bypassing the TTL cache. Move activityなどプロフィール更新が確実に必要な場合に使う。
func (r *Resolver) ForceResolveActor(uri string) (*model.User, error) {
	if existing, err := r.userRepo.FindByURI(uri); err == nil {
		r.refreshActor(existing, uri)
		return existing, nil
	}
	return r.ResolveActor(uri)
}

// notifyInstance is a best-effort hook into the instance tracker. ベスト
// エフォートのため、失敗してもエラーは伝搬しない。
func (r *Resolver) notifyInstance(host string) {
	if r.instanceTracker == nil || host == "" {
		return
	}
	_, _ = r.instanceTracker.RegisterFromHost(host)
}

// shouldRefreshActor reports whether a cached user row is past its TTL and
// should be refetched.
func (r *Resolver) shouldRefreshActor(u *model.User) bool {
	if u == nil || u.LastFetchedAt == nil {
		return true
	}
	return r.clock().Sub(*u.LastFetchedAt) > r.actorTTL
}

// refreshActor refetches the remote actor document and updates mutable fields
// on the local user row. 失敗してもエラーは返さず (呼び出し側はベストエフォート
// で既存値を使う)、ログは呼び出し元側で残す。
func (r *Resolver) refreshActor(existing *model.User, uri string) {
	actor, err := r.fetchActor(uri)
	if err != nil {
		return
	}
	now := r.clock()
	fields := map[string]any{
		"lastFetchedAt": &now,
	}
	if actor.Name != "" {
		name := actor.Name
		fields["name"] = &name
		existing.Name = &name
	}
	if actor.Inbox != "" {
		inbox := actor.Inbox
		fields["inbox"] = &inbox
		existing.Inbox = &inbox
	}
	if actor.Endpoints.SharedInbox != "" {
		shared := actor.Endpoints.SharedInbox
		fields["sharedInbox"] = &shared
		existing.SharedInbox = &shared
	}
	if actor.Featured != "" {
		featured := actor.Featured
		fields["featured"] = &featured
		existing.Featured = &featured
	}
	if actor.MovedTo != "" {
		movedTo := actor.MovedTo
		fields["movedToUri"] = &movedTo
		fields["movedAt"] = &now
		existing.MovedToURI = &movedTo
		existing.MovedAt = &now
	}
	if len(actor.AlsoKnownAs) > 0 {
		akaStr := strings.Join(actor.AlsoKnownAs, ",")
		fields["alsoKnownAs"] = &akaStr
		existing.AlsoKnownAs = &akaStr
	}
	existing.LastFetchedAt = &now
	// UpdateUser エラーはベストエフォートで無視 (次回再試行される)
	_ = r.userRepo.UpdateUser(existing.ID, fields)
	r.cachePublicKey(existing.ID, actor.PublicKey.ID, actor.PublicKey.PublicKeyPEM)
	if existing.Host != nil {
		r.notifyInstance(*existing.Host)
	}
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
	r.cachePublicKey(userID, actor.PublicKey.ID, actor.PublicKey.PublicKeyPEM)
}

// cachePublicKey stores a PEM in the in-memory cache and optionally persists
// it to the user_publickey table.
func (r *Resolver) cachePublicKey(userID, keyID, pem string) {
	r.keys[userID] = publicKeyEntry{pem: pem, fetchedAt: r.clock()}
	if r.publickeyRepo != nil && keyID != "" {
		if err := r.publickeyRepo.Upsert(&model.UserPublickey{
			UserID: userID,
			KeyID:  keyID,
			KeyPEM: pem,
		}); err != nil {
			slog.Warn("failed to persist public key", "userId", userID, "err", err)
		}
	}
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
	if apNote.Content != "" {
		note.Mentions = corenote.ExtractMentions(apNote.Content)
	}
	if err := r.noteRepo.Create(note); err != nil {
		return nil, err
	}
	// ローカルノートへの返信の場合、repliesCount を増やす。
	// これにより timeline や API 上の「返信数」表示が federated reply も
	// 含むようになる。失敗はベストエフォートで無視。
	if note.ReplyID != nil {
		_ = r.noteRepo.IncrementCount(*note.ReplyID, "repliesCount", 1)
	}
	return note, nil
}

// UpdateRemoteNote applies an inbound Update Note activity body to an existing
// remote-authored note row. Misskey 本家にはノート編集機能が無いためローカル
// note は不変だが、Mastodon 等から push されてくる Update activity は受信
// するだけ受信して反映する。
//
// 動作:
//   - URI でノートを引いて見つからなければ何もせず nil
//   - 見つかったが著者がローカルなら何もしない (ローカルノートは変更不可)
//   - 著者がリモートなら text/cw/sensitive/mentions を更新
func (r *Resolver) UpdateRemoteNote(body []byte) (*model.Note, error) {
	if r.noteRepo == nil {
		return nil, ErrInvalidNote
	}
	var apNote activitypub.Note
	if err := json.Unmarshal(body, &apNote); err != nil {
		return nil, ErrInvalidNote
	}
	if apNote.ID == "" {
		return nil, ErrInvalidNote
	}
	existing, err := r.noteRepo.FindByURI(apNote.ID)
	if err != nil {
		// 未取得のリモート Note は無視 (こちらに該当データが無いものを編集する
		// 通知が来ても反映先が無いため)。
		return nil, nil
	}
	if existing.UserHost == nil {
		// ローカルノートに対する Update は無視 (Misskey は編集機能を持たない)。
		return existing, nil
	}
	fields := map[string]any{}
	if apNote.Content != "" {
		text := apNote.Content
		fields["text"] = &text
		existing.Text = &text
		fields["mentions"] = corenote.ExtractMentions(apNote.Content)
		existing.Mentions = corenote.ExtractMentions(apNote.Content)
	} else {
		// content が空文字に変わるケース (text 削除) はサポート対象外
		// (空配信はメモリ攻撃の温床になりやすいので意図的に何もしない)。
	}
	if apNote.Summary != "" {
		summary := apNote.Summary
		fields["cw"] = &summary
		existing.CW = &summary
	} else if apNote.Sensitive {
		// Summary が空でも sensitive なら空 CW を保つ (IngestNote と対称)。
		empty := ""
		fields["cw"] = &empty
		existing.CW = &empty
	}
	if len(fields) == 0 {
		return existing, nil
	}
	if err := r.noteRepo.UpdateFields(existing.ID, fields); err != nil {
		return nil, err
	}
	return existing, nil
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

package federation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shiroha-a/mk/internal/activitypub"
	corefollowing "github.com/shiroha-a/mk/internal/core/following"
	corenote "github.com/shiroha-a/mk/internal/core/note"
	corereaction "github.com/shiroha-a/mk/internal/core/reaction"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// nowFn is the time source used by Processor when generating new note IDs.
// テストでの差し替えを容易にするため変数として保持する。
var nowFn = time.Now

// Errors returned by Processor.
var (
	// ErrUnsupportedActivity is returned when an activity type cannot be handled.
	ErrUnsupportedActivity = errors.New("unsupported activity type")
)

// Processor dispatches inbound activities to the right handler.
type Processor struct {
	resolver         *Resolver
	followingService *corefollowing.Service
	reactionService  *corereaction.Service
	noteDeleteSvc    *corenote.DeleteService
	userRepo         repository.UserRepository
	noteRepo         repository.NoteRepository
}

// NewProcessor constructs a Processor. reactionService / noteDeleteSvc は省略
// 可能 (nil)。nil の場合、対応する activity 種別は ErrUnsupportedActivity を返す。
func NewProcessor(
	resolver *Resolver,
	followingService *corefollowing.Service,
	reactionService *corereaction.Service,
	noteDeleteSvc *corenote.DeleteService,
	userRepo repository.UserRepository,
	noteRepo repository.NoteRepository,
) *Processor {
	return &Processor{
		resolver:         resolver,
		followingService: followingService,
		reactionService:  reactionService,
		noteDeleteSvc:    noteDeleteSvc,
		userRepo:         userRepo,
		noteRepo:         noteRepo,
	}
}

// genericActivity is the minimum struct used by the dispatcher to read the
// activity type and actor.
type genericActivity struct {
	Type   string          `json:"type"`
	Actor  string          `json:"actor"`
	Object json.RawMessage `json:"object"`
	ID     string          `json:"id"`
}

// Process consumes a JSON activity body received in an inbox. Returns nil if
// the activity is accepted (whether or not it produced side effects). Returns
// ErrUnsupportedActivity for types that are accepted but currently not
// processed (callers should still acknowledge the request to the sender).
//
// 入力 JSON は最初に activitypub.Normalize を通してキー名を canonical な短形式
// に揃えてから dispatch する。これにより `as:type` / `https://www.w3.org/ns/
// activitystreams#type` / `@type` のいずれでも同じ struct フィールドにマップ
// される。
func (p *Processor) Process(body []byte) error {
	normalized, err := activitypub.Normalize(body)
	if err != nil {
		return fmt.Errorf("invalid activity json: %w", err)
	}
	body = normalized

	// Normalize は内部で json.Marshal して再エンコードするため、ここでの
	// Unmarshal は構文エラーで失敗しないことが保証される。
	var act genericActivity
	_ = json.Unmarshal(body, &act)
	if act.Actor == "" {
		return errors.New("activity missing actor")
	}

	switch strings.ToLower(act.Type) {
	case "follow":
		return p.handleFollow(act)
	case "undo":
		return p.handleUndo(act)
	case "accept":
		return p.handleAccept(act)
	case "create":
		return p.handleCreate(act)
	case "like":
		return p.handleLike(act)
	case "announce":
		return p.handleAnnounce(act)
	case "delete":
		return p.handleDelete(act)
	case "update":
		return p.handleUpdate(act)
	case "reject":
		return p.handleReject(act)
	}
	return ErrUnsupportedActivity
}

// handleFollow processes an inbound Follow activity.
func (p *Processor) handleFollow(act genericActivity) error {
	follower, err := p.resolver.ResolveActor(act.Actor)
	if err != nil {
		return err
	}
	followeeURI, err := readObjectString(act.Object)
	if err != nil {
		return err
	}
	followee, err := p.userRepo.FindByURI(followeeURI)
	if err != nil {
		return errors.New("unknown followee")
	}
	if _, err := p.followingService.Follow(follower.ID, followee.ID); err != nil {
		// 既にフォロー済みは許容
		if errors.Is(err, corefollowing.ErrAlreadyFollowing) {
			return nil
		}
		return err
	}
	return nil
}

// handleUndo processes an Undo activity wrapping a Follow / Like / Announce.
func (p *Processor) handleUndo(act genericActivity) error {
	var inner genericActivity
	if err := json.Unmarshal(act.Object, &inner); err != nil {
		return fmt.Errorf("invalid undo object: %w", err)
	}
	switch strings.ToLower(inner.Type) {
	case "follow":
		return p.handleUndoFollow(act, inner)
	case "like":
		return p.handleUndoLike(act, inner)
	case "announce":
		return p.handleUndoAnnounce(act, inner)
	}
	return ErrUnsupportedActivity
}

// handleUndoFollow undoes a previously created Follow.
func (p *Processor) handleUndoFollow(act genericActivity, inner genericActivity) error {
	follower, err := p.resolver.ResolveActor(act.Actor)
	if err != nil {
		return err
	}
	followeeURI, err := readObjectString(inner.Object)
	if err != nil {
		return err
	}
	followee, err := p.userRepo.FindByURI(followeeURI)
	if err != nil {
		return errors.New("unknown followee")
	}
	if err := p.followingService.Unfollow(follower.ID, followee.ID); err != nil {
		if errors.Is(err, corefollowing.ErrNotFollowing) {
			return nil
		}
		return err
	}
	return nil
}

// handleUndoLike removes a reaction previously added via Like.
func (p *Processor) handleUndoLike(act genericActivity, inner genericActivity) error {
	if p.reactionService == nil {
		return ErrUnsupportedActivity
	}
	reactor, err := p.resolver.ResolveActor(act.Actor)
	if err != nil {
		return err
	}
	targetURI, err := readObjectString(inner.Object)
	if err != nil {
		return err
	}
	target, err := p.resolver.ResolveNote(targetURI)
	if err != nil {
		return err
	}
	if err := p.reactionService.Delete(reactor, target.ID); err != nil {
		// 既に削除済みは許容
		if errors.Is(err, corereaction.ErrReactionNotFound) {
			return nil
		}
		return err
	}
	return nil
}

// handleUndoAnnounce removes a previously created Announce (renote).
// inner.Object には元ノートの URI を持つことを想定する。
func (p *Processor) handleUndoAnnounce(act genericActivity, inner genericActivity) error {
	announcer, err := p.resolver.ResolveActor(act.Actor)
	if err != nil {
		return err
	}
	targetURI, err := readObjectString(inner.Object)
	if err != nil {
		return err
	}
	target, err := p.resolver.ResolveNote(targetURI)
	if err != nil {
		return err
	}
	// announcer が pure renote を 1 件でも持っていれば削除する。複数あった場合
	// は最新の 1 件のみで十分 (本家 misskey の挙動と同じ)。
	renotes, err := p.noteRepo.ListRenotesOf(target.ID, "", "", 50)
	if err != nil {
		return err
	}
	for _, n := range renotes {
		if n.UserID != announcer.ID {
			continue
		}
		if !corenote.IsPureRenote(n) {
			continue
		}
		if err := p.noteRepo.Delete(n); err != nil {
			return err
		}
		_ = p.noteRepo.IncrementCount(target.ID, "renoteCount", -1)
		return nil
	}
	return nil
}

// handleAccept currently only logs that an Accept was received. リモートからの
// Acceptは主に follow request の承認なので、Step E 以降で完全実装する。
func (p *Processor) handleAccept(_ genericActivity) error {
	return nil
}

// handleCreate persists an inbound Note carried by a Create activity.
func (p *Processor) handleCreate(act genericActivity) error {
	if _, err := p.resolver.ResolveActor(act.Actor); err != nil {
		return err
	}
	if _, err := p.resolver.IngestNote(act.Object); err != nil {
		return err
	}
	return nil
}

// handleLike attaches a reaction to a local note based on a Like activity.
func (p *Processor) handleLike(act genericActivity) error {
	if p.reactionService == nil {
		return ErrUnsupportedActivity
	}
	reactor, err := p.resolver.ResolveActor(act.Actor)
	if err != nil {
		return err
	}
	var like activitypub.Like
	if err := json.Unmarshal(act.Object, &like); err == nil && like.Object != "" {
		// 完全な Like オブジェクトを object として持つケース
	} else {
		// object が単なる URI 文字列のケース
		uri, rerr := readObjectString(act.Object)
		if rerr != nil {
			return rerr
		}
		like.Object = uri
	}
	target, err := p.resolver.ResolveNote(like.Object)
	if err != nil {
		return err
	}
	reaction := like.Content
	if _, err := p.reactionService.Create(reactor, target.ID, reaction); err != nil {
		if errors.Is(err, corereaction.ErrAlreadyReacted) {
			return nil
		}
		return err
	}
	return nil
}

// handleAnnounce creates a renote pointing at the announced note.
func (p *Processor) handleAnnounce(act genericActivity) error {
	announcer, err := p.resolver.ResolveActor(act.Actor)
	if err != nil {
		return err
	}
	targetURI, err := readObjectString(act.Object)
	if err != nil {
		return err
	}
	target, err := p.resolver.ResolveNote(targetURI)
	if err != nil {
		return err
	}
	// announce 自身に id があれば URI として保存し、重複検出にも使う
	if act.ID != "" {
		if _, err := p.noteRepo.FindByURI(act.ID); err == nil {
			return nil
		}
	}
	now := nowFn()
	renote := &model.Note{
		ID:         p.resolver.idGen.Generate(now),
		UserID:     announcer.ID,
		UserHost:   announcer.Host,
		RenoteID:   &target.ID,
		Visibility: model.NoteVisibilityPublic,
	}
	if act.ID != "" {
		uri := act.ID
		renote.URI = &uri
	}
	renote.RenoteUserID = &target.UserID
	renote.RenoteUserHost = target.UserHost
	if err := p.noteRepo.Create(renote); err != nil {
		return err
	}
	_ = p.noteRepo.IncrementCount(target.ID, "renoteCount", 1)
	return nil
}

// handleDelete removes a remote note (or actor) referenced by a Delete activity.
func (p *Processor) handleDelete(act genericActivity) error {
	author, err := p.resolver.ResolveActor(act.Actor)
	if err != nil {
		return err
	}
	targetURI, err := readObjectString(act.Object)
	if err != nil {
		return err
	}
	// Actor 自身の Delete (アカウント削除) は現在未対応 — 受信は許容して no-op。
	if author.URI != nil && *author.URI == targetURI {
		return nil
	}
	note, err := p.noteRepo.FindByURI(targetURI)
	if err != nil {
		// 既に存在しないなら成功扱い
		return nil
	}
	if note.UserID != author.ID {
		return errors.New("delete from non-author")
	}
	if p.noteDeleteSvc != nil {
		return p.noteDeleteSvc.Delete(author, note.ID)
	}
	return p.noteRepo.Delete(note)
}

// handleUpdate refreshes a remote actor's stored profile fields, or applies
// an inbound Update Note activity from a federated peer.
//
// Misskey 本家にはノート編集 API が無いが、Mastodon 系の Update Note は受信
// するだけ受信して反映する (`Resolver.UpdateRemoteNote` 経由)。それ以外の
// type (Question / Article 等) は no-op。
func (p *Processor) handleUpdate(act genericActivity) error {
	// 先に object の type を覗いて Note / Person の判定を行う。
	objectType := peekObjectType(act.Object)
	if strings.EqualFold(objectType, "note") {
		_, err := p.resolver.UpdateRemoteNote(act.Object)
		// ErrInvalidNote は受信側の不備として skip 扱い (200 を返す)。
		if errors.Is(err, ErrInvalidNote) {
			return nil
		}
		return err
	}

	// object が単なる URI なら fetch、object 内に Person があればそれを使う
	var person activitypub.Person
	if err := json.Unmarshal(act.Object, &person); err != nil || person.ID == "" {
		// object が文字列 URI のケース
		uri, rerr := readObjectString(act.Object)
		if rerr != nil {
			return rerr
		}
		person.ID = uri
	}
	if person.Type != "" && !strings.EqualFold(person.Type, "person") {
		// Question / Article 等は未対応
		return nil
	}
	user, err := p.userRepo.FindByURI(person.ID)
	if err != nil {
		// 未取得のリモートユーザーなら無視 (次回 follow/inbox などで取り込まれる)
		return nil
	}
	fields := map[string]any{}
	if person.Name != "" {
		name := person.Name
		fields["name"] = &name
	}
	if len(fields) == 0 {
		return nil
	}
	return p.userRepo.UpdateUser(user.ID, fields)
}

// peekObjectType reads only the "type" field from a JSON object body. パース
// 不能・object でない・type フィールド無しの場合は空文字を返す。
func peekObjectType(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var obj struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	return obj.Type
}

// handleReject undoes a Follow that was rejected by the followee.
//
// 想定: 自分 (ローカル follower) が remote followee に対して Follow を送ったが
// 拒否された場合に呼ばれる。inner.Follow.actor がローカル follower、object が
// remote followee。
func (p *Processor) handleReject(act genericActivity) error {
	var inner genericActivity
	if err := json.Unmarshal(act.Object, &inner); err != nil {
		return fmt.Errorf("invalid reject object: %w", err)
	}
	if !strings.EqualFold(inner.Type, "follow") {
		return ErrUnsupportedActivity
	}
	followee, err := p.resolver.ResolveActor(act.Actor)
	if err != nil {
		return err
	}
	followerURI, err := readActorString(inner)
	if err != nil {
		return err
	}
	follower, err := p.userRepo.FindByURI(followerURI)
	if err != nil {
		return nil
	}
	// 既存のフォローがあれば解除する。pending な follow request も同様。
	if err := p.followingService.Unfollow(follower.ID, followee.ID); err != nil &&
		!errors.Is(err, corefollowing.ErrNotFollowing) {
		return err
	}
	if err := p.followingService.CancelRequest(follower.ID, followee.ID); err != nil &&
		!errors.Is(err, corefollowing.ErrRequestNotFound) {
		return err
	}
	return nil
}

// readObjectString reads an activity Object field that is either a plain
// string IRI or a nested object with an "id" field.
func readObjectString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", errors.New("missing object")
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return s, nil
	}
	var obj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", err
	}
	if obj.ID == "" {
		return "", errors.New("object missing id")
	}
	return obj.ID, nil
}

// readActorString reads an activity's actor field, supporting both string and
// object forms (mirrors readObjectString but on the actor field of an inner
// activity such as the Follow inside a Reject).
func readActorString(act genericActivity) (string, error) {
	if act.Actor != "" {
		return act.Actor, nil
	}
	return "", errors.New("inner activity missing actor")
}

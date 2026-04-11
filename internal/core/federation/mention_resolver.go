package federation

import (
	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/repository"
)

// UserMentionResolver implements activitypub.MentionResolver by looking up
// users via UserRepository and building the AS Mention tag data (name + href).
// ローカルユーザーは urls.UserURI を返し、リモートユーザーは user.URI を返す。
type UserMentionResolver struct {
	userRepo repository.UserRepository
	urls     *activitypub.URLBuilder
}

// NewUserMentionResolver constructs a UserMentionResolver.
func NewUserMentionResolver(userRepo repository.UserRepository, urls *activitypub.URLBuilder) *UserMentionResolver {
	return &UserMentionResolver{userRepo: userRepo, urls: urls}
}

// ResolveMention looks up the user by ID and returns the mention name and URI.
// 未知のユーザー (削除済み等) は ok=false を返しスキップ。
func (r *UserMentionResolver) ResolveMention(userID string) (name, uri string, ok bool) {
	u, err := r.userRepo.FindByID(userID)
	if err != nil || u == nil {
		return "", "", false
	}
	if u.IsLocal() {
		return "@" + u.Username, r.urls.UserURI(u.ID), true
	}
	if u.URI == nil || *u.URI == "" {
		return "", "", false
	}
	// IsLocal() returns false here, so Host is non-nil and non-empty.
	return "@" + u.Username + "@" + *u.Host, *u.URI, true
}

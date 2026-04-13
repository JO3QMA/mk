package entity

import (
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/datatypes"
)

// UserLite is the minimal user representation returned by most API endpoints.
type UserLite struct {
	ID                string            `json:"id"`
	Name              *string           `json:"name"`
	Username          string            `json:"username"`
	Host              *string           `json:"host"`
	AvatarURL         *string           `json:"avatarUrl"`
	AvatarBlurhash    *string           `json:"avatarBlurhash"`
	AvatarDecorations datatypes.JSON    `json:"avatarDecorations"`
	IsBot             bool              `json:"isBot"`
	IsCat             bool              `json:"isCat"`
	Emojis            map[string]string `json:"emojis"`
	OnlineStatus      string            `json:"onlineStatus"`
	BadgeRoles        []any             `json:"badgeRoles"`
}

// UserDetailed includes additional fields for detailed user views.
type UserDetailed struct {
	UserLite
	BannerURL           *string        `json:"bannerUrl"`
	BannerBlurhash      *string        `json:"bannerBlurhash"`
	IsLocked            bool           `json:"isLocked"`
	IsSilenced          bool           `json:"isSilenced"`
	IsSuspended         bool           `json:"isSuspended"`
	Description         *string        `json:"description"`
	Location            *string        `json:"location"`
	Birthday            *string        `json:"birthday"`
	Lang                *string        `json:"lang"`
	Fields              datatypes.JSON `json:"fields"`
	VerifiedLinks       []string       `json:"verifiedLinks"`
	FollowersCount      int            `json:"followersCount"`
	FollowingCount      int            `json:"followingCount"`
	NotesCount          int            `json:"notesCount"`
	FollowersVisibility string         `json:"followersVisibility"`
	FollowingVisibility string         `json:"followingVisibility"`
	CreatedAt           string         `json:"createdAt"`
	UpdatedAt           *string        `json:"updatedAt"`
	URI                 *string        `json:"uri"`
	URL                 *string        `json:"url"`
	PinnedNoteIDs       []string       `json:"pinnedNoteIds"`
	PinnedNotes         []any          `json:"pinnedNotes"`
	Roles               []any          `json:"roles"`
	Instance            *InstanceLite  `json:"instance,omitempty"`
	// viewer依存フィールド (ハンドラ側でセット)
	IsFollowing                    *bool   `json:"isFollowing"`
	IsFollowed                     *bool   `json:"isFollowed"`
	IsBlocking                     *bool   `json:"isBlocking,omitempty"`
	IsBlocked                      *bool   `json:"isBlocked,omitempty"`
	IsMuted                        *bool   `json:"isMuted,omitempty"`
	IsRenoteMuted                  *bool   `json:"isRenoteMuted,omitempty"`
	HasPendingFollowRequestFromYou *bool   `json:"hasPendingFollowRequestFromYou,omitempty"`
	HasPendingFollowRequestToYou   *bool   `json:"hasPendingFollowRequestToYou,omitempty"`
	Notify                         *string `json:"notify,omitempty"`
	WithReplies                    *bool   `json:"withReplies,omitempty"`
	Memo                           *string `json:"memo,omitempty"`
}

// InstanceLite is the minimal instance info embedded in UserDetailed for remote users.
type InstanceLite struct {
	Name            *string `json:"name"`
	SoftwareName    *string `json:"softwareName"`
	SoftwareVersion *string `json:"softwareVersion"`
	IconURL         *string `json:"iconUrl"`
	FaviconURL      *string `json:"faviconUrl"`
	ThemeColor      *string `json:"themeColor"`
}

// PackUserLite converts a model.User to a UserLite DTO.
func PackUserLite(u *model.User) UserLite {
	avatarURL := u.AvatarURL
	// avatarUrlがnullの場合、identiconを生成
	if avatarURL == nil || *avatarURL == "" {
		host := ""
		if u.Host != nil {
			host = "@" + *u.Host
		}
		identicon := "/identicon/" + u.Username + host
		avatarURL = &identicon
	}
	return UserLite{
		ID:                u.ID,
		Name:              u.Name,
		Username:          u.Username,
		Host:              u.Host,
		AvatarURL:         avatarURL,
		AvatarBlurhash:    u.AvatarBlurhash,
		AvatarDecorations: u.AvatarDecorations,
		IsBot:             u.IsBot,
		IsCat:             u.IsCat,
		Emojis:            make(map[string]string),
		OnlineStatus:      "unknown",
		BadgeRoles:        []any{},
	}
}

// PackUserDetailed converts a model.User and optional profile to UserDetailed.
func PackUserDetailed(u *model.User, profile *model.UserProfile, idGens ...id.Generator) UserDetailed {
	d := UserDetailed{
		UserLite:            PackUserLite(u),
		BannerURL:           u.BannerURL,
		BannerBlurhash:      u.BannerBlurhash,
		IsLocked:            u.IsLocked,
		IsSuspended:         u.IsSuspended,
		FollowersCount:      u.FollowersCount,
		FollowingCount:      u.FollowingCount,
		NotesCount:          u.NotesCount,
		VerifiedLinks:       []string{},
		FollowersVisibility: "public",
		FollowingVisibility: "public",
		URI:                 u.URI,
		PinnedNoteIDs:       []string{},
		PinnedNotes:         []any{},
		Roles:               []any{},
	}

	// IDからcreatedAtを抽出
	if len(idGens) > 0 && idGens[0] != nil {
		if t, err := idGens[0].ParseTime(u.ID); err == nil {
			d.CreatedAt = t.UTC().Format("2006-01-02T15:04:05.000Z")
		}
	}

	if profile != nil {
		d.Description = profile.Description
		d.Location = profile.Location
		d.Birthday = profile.Birthday
		d.Lang = profile.Lang
		d.Fields = profile.Fields
		if len(profile.VerifiedLinks) > 0 {
			d.VerifiedLinks = []string(profile.VerifiedLinks)
		}
		d.FollowersVisibility = string(profile.FollowersVisibility)
		d.FollowingVisibility = string(profile.FollowingVisibility)
	}

	return d
}

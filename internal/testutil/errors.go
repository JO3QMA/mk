package testutil

import "gorm.io/gorm"

// ErrNotFound is returned by mock repositories when an entity is not found.
//
// production の GORM repositories は record-not-found 時に
// gorm.ErrRecordNotFound を返すので、mock 側もこれと同一 sentinel に
// 揃える。これにより上位 layer で `errors.Is(err, gorm.ErrRecordNotFound)`
// で分岐するコード (例: activitypub.UserMentionResolver) が mock テスト
// 経路でも production と同じ挙動になる (#572 item 2)。
var ErrNotFound = gorm.ErrRecordNotFound

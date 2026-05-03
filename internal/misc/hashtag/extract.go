// Package hashtag provides hashtag extraction from note text.
//
// Misskey TS (mfm-js) は MFM パーサで text を構造化してから hashtag
// ノードを取り出すが、mk-go では現状軽量な正規表現ベースの抽出に
// 留めている。連合経由で受け取る ActivityPub Object には別途 tag 配列
// が付くので、本パッケージは local note 作成時の text/cw 由来 hashtag
// だけを担当する。
package hashtag

import (
	"regexp"
	"strings"
)

// MaxTagLength は note.tags 列の varchar(128) 制約に合わせた tag 長
// 上限。これを超える tag は truncate される (drop ではなく trim にする
// のは Misskey TS の挙動互換)。
const MaxTagLength = 128

// hashtagRe は #tag を抽出する正規表現。
//
//   - 直前は行頭 / 空白 / 一部の記号 (引用符・閉じ括弧・改行) の
//     「単語境界」とみなせる文字。`#one#two` のような連続は upstream
//     Misskey でも `#one` のみが拾われるため境界に `#` は含めない。
//   - tag 本体は Unicode の Letter / Number / アンダースコア / ハイフン。
//     MFM の hashtag 文字種より狭めだが、よく使われる範囲はカバーする。
//
// FindAllStringSubmatch でキャプチャ #1 が tag 本体になる。
var hashtagRe = regexp.MustCompile(`(?:^|[\s>"'` + "`" + `(\[{])#([\p{L}\p{N}_-]+)`)

// Extract pulls hashtag names out of text fragments (typically the note's
// body and CW). Order is preserved by first occurrence; case-insensitive
// duplicates collapse to a single entry. The original case is preserved
// for the kept tag (Misskey TS も同じ挙動)。
//
// Empty / whitespace-only inputs return nil. Tags longer than
// MaxTagLength are truncated.
func Extract(parts ...string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, text := range parts {
		if text == "" {
			continue
		}
		matches := hashtagRe.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			tag := m[1]
			if len(tag) > MaxTagLength {
				tag = tag[:MaxTagLength]
			}
			key := strings.ToLower(tag)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, tag)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

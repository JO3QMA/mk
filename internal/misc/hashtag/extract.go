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

// codeBlockRe / urlRe は MFM パーサ未導入の代替として、hashtag 抽出
// 前に「ハッシュタグとして拾うべきでない領域」を空白に置換するための
// 正規表現。
//
//   - codeBlockRe: \`\`\`fence\`\`\` / inline \`code\`。インライン版は同行
//     内のみで閉じる (改行をまたぐ \`...\` は意図しない大量マッチを生む)。
//   - urlRe: http(s) URL の末尾に付く #fragment が hashtag 扱いされて
//     しまう問題への対処 (e.g. https://example.com/path#anchor)。
//
// 真の MFM パーサ統合 (#上流 mfm-js 相当) は別 issue 扱いで、本
// 実装は最も多い誤検知 2 種を遮断するライトウェイト版。
var (
	codeBlockRe = regexp.MustCompile("```[\\s\\S]*?```|`[^`\n]*`")
	urlRe       = regexp.MustCompile(`https?://[^\s]+`)
)

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
		// code block / URL の中に現れる # は hashtag として扱わない。
		// 元 text の長さを保つために空白置換する (位置情報は使わないが、
		// 隣接トークンの境界が壊れないようにする保険)。
		cleaned := codeBlockRe.ReplaceAllString(text, " ")
		cleaned = urlRe.ReplaceAllString(cleaned, " ")
		matches := hashtagRe.FindAllStringSubmatch(cleaned, -1)
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

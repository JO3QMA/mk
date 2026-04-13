package mfm

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Parse tokenizes and parses the input MFM string into an AST.
func Parse(input string) []*Node {
	if input == "" {
		return nil
	}
	s := &state{src: input, depth: 0, nestLimit: 20}
	nodes := s.parseNodes(false)
	return mergeText(nodes)
}

// ParseSimple parses with limited syntax: only text, unicodeEmoji, emojiCode, plain.
func ParseSimple(input string) []*Node {
	if input == "" {
		return nil
	}
	s := &state{src: input, depth: 0, nestLimit: 20, simple: true}
	nodes := s.parseNodes(false)
	return mergeText(nodes)
}

// state tracks parser position and nesting.
type state struct {
	src       string
	pos       int
	depth     int
	nestLimit int
	inLink    bool
	simple    bool
}

func (s *state) remaining() string { return s.src[s.pos:] }
func (s *state) eof() bool         { return s.pos >= len(s.src) }

func (s *state) peek() rune {
	if s.eof() {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(s.src[s.pos:])
	return r
}

func (s *state) advance(n int) { s.pos += n }

func (s *state) hasPrefix(prefix string) bool {
	return strings.HasPrefix(s.remaining(), prefix)
}

// parseNodes parses until EOF or (if inQuote) until a line doesn't start with >.
func (s *state) parseNodes(inQuote bool) []*Node {
	var nodes []*Node
	for !s.eof() {
		if inQuote {
			// quoteの中で行頭が>でなくなったら終了
			if s.pos > 0 && s.src[s.pos-1] == '\n' && !s.hasPrefix(">") {
				break
			}
		}
		node := s.parseOne()
		if node == nil {
			break
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func (s *state) parseOne() *Node {
	if s.eof() {
		return nil
	}
	if s.simple {
		return s.parseSimpleOne()
	}
	return s.parseFullOne()
}

func (s *state) parseSimpleOne() *Node {
	if n := s.tryUnicodeEmoji(); n != nil {
		return n
	}
	if n := s.tryEmojiCode(); n != nil {
		return n
	}
	if n := s.tryPlainTag(); n != nil {
		return n
	}
	return s.consumeChar()
}

func (s *state) parseFullOne() *Node {
	// mfm-js の alt() 順序に従う
	if n := s.tryUnicodeEmoji(); n != nil {
		return n
	}
	if n := s.tryCenterTag(); n != nil {
		return n
	}
	if n := s.trySmallTag(); n != nil {
		return n
	}
	if n := s.tryPlainTag(); n != nil {
		return n
	}
	if n := s.tryBoldTag(); n != nil {
		return n
	}
	if n := s.tryItalicTag(); n != nil {
		return n
	}
	if n := s.tryStrikeTag(); n != nil {
		return n
	}
	if n := s.tryBoldAsta(); n != nil {
		return n
	}
	if n := s.tryItalicAsta(); n != nil {
		return n
	}
	if n := s.tryBoldUnder(); n != nil {
		return n
	}
	if n := s.tryItalicUnder(); n != nil {
		return n
	}
	if n := s.tryCodeBlock(); n != nil {
		return n
	}
	if n := s.tryInlineCode(); n != nil {
		return n
	}
	if n := s.tryQuote(); n != nil {
		return n
	}
	if n := s.tryMathBlock(); n != nil {
		return n
	}
	if n := s.tryMathInline(); n != nil {
		return n
	}
	if n := s.tryStrikeWave(); n != nil {
		return n
	}
	if n := s.tryFn(); n != nil {
		return n
	}
	if n := s.tryMention(); n != nil {
		return n
	}
	if n := s.tryHashtag(); n != nil {
		return n
	}
	if n := s.tryEmojiCode(); n != nil {
		return n
	}
	if !s.inLink {
		if n := s.tryLink(); n != nil {
			return n
		}
	}
	if n := s.tryURL(); n != nil {
		return n
	}
	if n := s.trySearch(); n != nil {
		return n
	}
	return s.consumeChar()
}

// consumeChar takes one rune and returns it as a text node.
func (s *state) consumeChar() *Node {
	r, size := utf8.DecodeRuneInString(s.remaining())
	s.advance(size)
	return Text(string(r))
}

// nest runs parser function with incremented depth. nil on limit.
func (s *state) nest(fn func() []*Node) []*Node {
	s.depth++
	if s.depth > s.nestLimit {
		s.depth--
		return nil
	}
	result := fn()
	s.depth--
	return result
}

// --- Block-level parsers ---

func (s *state) tryQuote() *Node {
	// 行頭もしくはテキスト先頭のみ
	if s.pos > 0 && s.src[s.pos-1] != '\n' {
		return nil
	}
	if !s.hasPrefix(">") {
		return nil
	}
	save := s.pos
	var lines []string
	for !s.eof() && s.hasPrefix(">") {
		s.advance(1) // skip >
		if !s.eof() && (s.peek() == ' ' || s.peek() == '\t') {
			s.advance(1) // skip optional space
		}
		lineStart := s.pos
		for !s.eof() && s.peek() != '\n' {
			s.advance(utf8.RuneLen(s.peek()))
		}
		lines = append(lines, s.src[lineStart:s.pos])
		if !s.eof() {
			s.advance(1) // skip \n
		}
	}
	if len(lines) == 0 {
		s.pos = save
		return nil
	}
	inner := strings.Join(lines, "\n")
	children := s.nest(func() []*Node {
		sub := &state{src: inner, depth: s.depth, nestLimit: s.nestLimit}
		return sub.parseNodes(false)
	})
	if children == nil {
		s.pos = save
		return nil
	}
	return withChildren(NodeQuote, mergeText(children))
}

func (s *state) tryCodeBlock() *Node {
	if s.pos > 0 && s.src[s.pos-1] != '\n' {
		return nil
	}
	if !s.hasPrefix("```") {
		return nil
	}
	save := s.pos
	s.advance(3) // skip ```
	// optional lang
	langStart := s.pos
	for !s.eof() && s.peek() != '\n' {
		s.advance(utf8.RuneLen(s.peek()))
	}
	lang := strings.TrimSpace(s.src[langStart:s.pos])
	if s.eof() {
		s.pos = save
		return nil
	}
	s.advance(1) // skip \n

	codeStart := s.pos
	for !s.eof() {
		if s.hasPrefix("\n```") {
			code := s.src[codeStart:s.pos]
			s.advance(4) // skip \n```
			// 行末まで消費 (改行 or EOF)
			for !s.eof() && s.peek() != '\n' {
				s.advance(1)
			}
			if !s.eof() {
				s.advance(1)
			}
			props := map[string]any{"code": code}
			if lang != "" {
				props["lang"] = lang
			}
			return &Node{Type: NodeBlockCode, Props: props}
		}
		s.advance(utf8.RuneLen(s.peek()))
	}
	s.pos = save
	return nil
}

func (s *state) tryMathBlock() *Node {
	if !s.hasPrefix("\\[") {
		return nil
	}
	save := s.pos
	s.advance(2)
	start := s.pos
	for !s.eof() {
		if s.hasPrefix("\\]") {
			formula := strings.TrimSpace(s.src[start:s.pos])
			if formula == "" {
				break
			}
			s.advance(2)
			return withProp(NodeMathBlock, "formula", formula)
		}
		s.advance(utf8.RuneLen(s.peek()))
	}
	s.pos = save
	return nil
}

func (s *state) tryCenterTag() *Node {
	return s.tryHTMLTag("center", NodeCenter, true)
}

func (s *state) trySmallTag() *Node {
	return s.tryHTMLTag("small", NodeSmall, true)
}

func (s *state) tryPlainTag() *Node {
	if !s.hasPrefix("<plain>") {
		return nil
	}
	save := s.pos
	s.advance(7) // <plain>
	start := s.pos
	for !s.eof() {
		if s.hasPrefix("</plain>") {
			text := s.src[start:s.pos]
			s.advance(8) // </plain>
			return &Node{Type: NodePlain, Children: []*Node{Text(text)}}
		}
		s.advance(utf8.RuneLen(s.peek()))
	}
	s.pos = save
	return nil
}

func (s *state) tryBoldTag() *Node {
	return s.tryHTMLTag("b", NodeBold, true)
}

func (s *state) tryItalicTag() *Node {
	return s.tryHTMLTag("i", NodeItalic, true)
}

func (s *state) tryStrikeTag() *Node {
	return s.tryHTMLTag("s", NodeStrike, true)
}

// tryHTMLTag は <tag>...</tag> 形式のパースを試みる。
// recurse=true で children を再帰パースする。
func (s *state) tryHTMLTag(tag string, nodeType NodeType, recurse bool) *Node {
	open := "<" + tag + ">"
	close := "</" + tag + ">"
	if !s.hasPrefix(open) {
		return nil
	}
	save := s.pos
	s.advance(len(open))

	if recurse {
		children := s.nest(func() []*Node {
			var nodes []*Node
			for !s.eof() && !s.hasPrefix(close) {
				n := s.parseOne()
				if n == nil {
					break
				}
				nodes = append(nodes, n)
			}
			return nodes
		})
		if s.hasPrefix(close) {
			s.advance(len(close))
			return withChildren(nodeType, mergeText(children))
		}
	}
	s.pos = save
	return nil
}

// --- Inline parsers ---

func (s *state) tryBoldAsta() *Node {
	if !s.hasPrefix("**") {
		return nil
	}
	return s.tryWrapped("**", "**", NodeBold)
}

func (s *state) tryItalicAsta() *Node {
	if !s.hasPrefix("*") || s.hasPrefix("**") {
		return nil
	}
	// 直前が英数字なら失敗
	if s.pos > 0 && isAlphanumeric(s.prevRune()) {
		return nil
	}
	return s.tryWrappedAlphaSpace("*", "*", NodeItalic)
}

func (s *state) tryBoldUnder() *Node {
	if !s.hasPrefix("__") {
		return nil
	}
	return s.tryWrappedAlphaSpace("__", "__", NodeBold)
}

func (s *state) tryItalicUnder() *Node {
	if !s.hasPrefix("_") || s.hasPrefix("__") {
		return nil
	}
	if s.pos > 0 && isAlphanumeric(s.prevRune()) {
		return nil
	}
	return s.tryWrappedAlphaSpace("_", "_", NodeItalic)
}

func (s *state) tryStrikeWave() *Node {
	if !s.hasPrefix("~~") {
		return nil
	}
	return s.tryWrapped("~~", "~~", NodeStrike)
}

// tryWrapped は open...close で囲まれた部分を再帰パースする。
func (s *state) tryWrapped(open, close string, nodeType NodeType) *Node {
	save := s.pos
	s.advance(len(open))
	if s.eof() || s.hasPrefix(close) {
		s.pos = save
		return nil
	}
	children := s.nest(func() []*Node {
		var nodes []*Node
		for !s.eof() && !s.hasPrefix(close) {
			if s.peek() == '\n' {
				s.pos = save
				return nil
			}
			n := s.parseOne()
			if n == nil {
				break
			}
			nodes = append(nodes, n)
		}
		return nodes
	})
	if children == nil || !s.hasPrefix(close) {
		s.pos = save
		return nil
	}
	s.advance(len(close))
	return withChildren(nodeType, mergeText(children))
}

// tryWrappedAlphaSpace は英数字+空白のみを含むラップされた部分をパースする。
func (s *state) tryWrappedAlphaSpace(open, close string, nodeType NodeType) *Node {
	save := s.pos
	s.advance(len(open))
	start := s.pos
	for !s.eof() {
		if s.hasPrefix(close) {
			content := s.src[start:s.pos]
			if content == "" {
				break
			}
			// 英数字+空白のみか確認
			if !isAlphaSpaceOnly(content) {
				break
			}
			s.advance(len(close))
			return withChildren(nodeType, []*Node{Text(content)})
		}
		if s.peek() == '\n' {
			break
		}
		s.advance(utf8.RuneLen(s.peek()))
	}
	s.pos = save
	return nil
}

func (s *state) tryInlineCode() *Node {
	if s.peek() != '`' || s.hasPrefix("```") {
		return nil
	}
	save := s.pos
	s.advance(1)
	start := s.pos
	for !s.eof() {
		ch := s.peek()
		if ch == '`' {
			code := s.src[start:s.pos]
			if code == "" {
				break
			}
			s.advance(1)
			return withProp(NodeInlineCode, "code", code)
		}
		if ch == '\n' || ch == 0xb4 { // ´ acute accent
			break
		}
		s.advance(utf8.RuneLen(ch))
	}
	s.pos = save
	return nil
}

func (s *state) tryMathInline() *Node {
	if !s.hasPrefix("\\(") {
		return nil
	}
	save := s.pos
	s.advance(2)
	start := s.pos
	for !s.eof() {
		if s.hasPrefix("\\)") {
			formula := s.src[start:s.pos]
			if formula == "" {
				break
			}
			s.advance(2)
			return withProp(NodeMathInline, "formula", formula)
		}
		if s.peek() == '\n' {
			break
		}
		s.advance(utf8.RuneLen(s.peek()))
	}
	s.pos = save
	return nil
}

func (s *state) tryFn() *Node {
	if !s.hasPrefix("$[") {
		return nil
	}
	save := s.pos
	s.advance(2)
	// 関数名をパース
	nameStart := s.pos
	for !s.eof() {
		ch := s.peek()
		if isASCIIAlphanumeric(ch) || ch == '_' {
			s.advance(1)
			continue
		}
		break
	}
	name := s.src[nameStart:s.pos]
	if name == "" {
		s.pos = save
		return nil
	}

	// 引数をパース (ドットで始まる)
	var args map[string]any
	if !s.eof() && s.peek() == '.' {
		s.advance(1)
		args = s.parseFnArgs()
	}

	// スペース区切り
	if s.eof() || s.peek() != ' ' {
		s.pos = save
		return nil
	}
	s.advance(1)

	// children (] まで)
	children := s.nest(func() []*Node {
		var nodes []*Node
		for !s.eof() && s.peek() != ']' {
			n := s.parseOne()
			if n == nil {
				break
			}
			nodes = append(nodes, n)
		}
		return nodes
	})
	if !s.hasPrefix("]") {
		s.pos = save
		return nil
	}
	s.advance(1)

	props := map[string]any{"name": name}
	if args != nil {
		props["args"] = args
	}
	return &Node{Type: NodeFn, Props: props, Children: mergeText(children)}
}

func (s *state) parseFnArgs() map[string]any {
	args := map[string]any{}
	for !s.eof() {
		keyStart := s.pos
		for !s.eof() {
			ch := s.peek()
			if isASCIIAlphanumeric(ch) || ch == '_' {
				s.advance(1)
				continue
			}
			break
		}
		key := s.src[keyStart:s.pos]
		if key == "" {
			break
		}
		if !s.eof() && s.peek() == '=' {
			s.advance(1)
			valStart := s.pos
			for !s.eof() {
				ch := s.peek()
				if ch == ',' || ch == ' ' || ch == ']' {
					break
				}
				s.advance(utf8.RuneLen(ch))
			}
			args[key] = s.src[valStart:s.pos]
		} else {
			args[key] = true
		}
		if !s.eof() && s.peek() == ',' {
			s.advance(1)
			continue
		}
		break
	}
	return args
}

func (s *state) tryMention() *Node {
	if s.peek() != '@' {
		return nil
	}
	// 直前が英数字なら失敗
	if s.pos > 0 && isAlphanumeric(s.prevRune()) {
		return nil
	}
	save := s.pos
	s.advance(1) // skip @
	username := s.consumeIdent()
	if username == "" {
		s.pos = save
		return nil
	}
	var host string
	if !s.eof() && s.peek() == '@' {
		s.advance(1)
		host = s.consumeIdent()
		if host == "" {
			// @user@ のようなパターンはホスト無しに戻す
			s.pos -= 1 // @ を戻す
		}
	}
	// 末尾のドット・ハイフンを削る
	username = strings.TrimRight(username, ".-")
	host = strings.TrimRight(host, ".-")
	if username == "" {
		s.pos = save
		return nil
	}

	props := map[string]any{"username": username}
	acct := "@" + username
	if host != "" {
		props["host"] = host
		acct += "@" + host
	}
	props["acct"] = acct
	return &Node{Type: NodeMention, Props: props}
}

func (s *state) consumeIdent() string {
	start := s.pos
	for !s.eof() {
		ch := s.peek()
		if isASCIIAlphanumeric(ch) || ch == '_' || ch == '-' || ch == '.' {
			s.advance(1)
			continue
		}
		break
	}
	return s.src[start:s.pos]
}

func (s *state) tryHashtag() *Node {
	if s.peek() != '#' {
		return nil
	}
	if s.pos > 0 && isAlphanumeric(s.prevRune()) {
		return nil
	}
	save := s.pos
	s.advance(1) // skip #

	start := s.pos
	s.consumeHashtagContent()
	tag := s.src[start:s.pos]
	if tag == "" {
		s.pos = save
		return nil
	}
	// 数字だけのハッシュタグは無効
	if isDigitsOnly(tag) {
		s.pos = save
		return nil
	}
	return &Node{Type: NodeHashtag, Props: map[string]any{"hashtag": tag}}
}

func (s *state) consumeHashtagContent() {
	for !s.eof() {
		ch := s.peek()
		// ハッシュタグで無効な文字
		if ch == '#' || unicode.IsSpace(ch) || ch == '.' || ch == ',' || ch == '!' ||
			ch == '?' || ch == '\'' || ch == '"' || ch == ':' || ch == '<' || ch == '>' {
			return
		}
		// 括弧のネスト
		switch ch {
		case '(':
			s.advance(1)
			s.consumeHashtagContent()
			if !s.eof() && s.peek() == ')' {
				s.advance(1)
			}
			continue
		case '[':
			s.advance(1)
			s.consumeHashtagContent()
			if !s.eof() && s.peek() == ']' {
				s.advance(1)
			}
			continue
		case '「':
			s.advance(utf8.RuneLen(ch))
			s.consumeHashtagContent()
			if !s.eof() && s.peek() == '」' {
				s.advance(utf8.RuneLen('」'))
			}
			continue
		case '（':
			s.advance(utf8.RuneLen(ch))
			s.consumeHashtagContent()
			if !s.eof() && s.peek() == '）' {
				s.advance(utf8.RuneLen('）'))
			}
			continue
		case ')', ']', '」', '）':
			return
		}
		s.advance(utf8.RuneLen(ch))
	}
}

func (s *state) tryEmojiCode() *Node {
	if s.peek() != ':' {
		return nil
	}
	// 境界チェック
	if s.pos > 0 && isAlphanumeric(s.prevRune()) {
		return nil
	}
	save := s.pos
	s.advance(1) // skip :
	start := s.pos
	for !s.eof() {
		ch := s.peek()
		if ch == ':' {
			name := s.src[start:s.pos]
			if name == "" {
				break
			}
			// 英数字+_+-のみ
			if !isEmojiName(name) {
				break
			}
			s.advance(1)
			return withProp(NodeEmojiCode, "name", name)
		}
		if ch == '\n' || unicode.IsSpace(ch) {
			break
		}
		s.advance(utf8.RuneLen(ch))
	}
	s.pos = save
	return nil
}

func (s *state) tryUnicodeEmoji() *Node {
	rest := s.remaining()
	if len(rest) == 0 {
		return nil
	}
	r, size := utf8.DecodeRuneInString(rest)
	if r == utf8.RuneError {
		return nil
	}

	// 絵文字判定: Emoji/Symbol カテゴリ + FE0F (variation selector) の組み合わせ
	if !isEmojiStart(r) {
		return nil
	}

	// 連続する絵文字関連コードポイントを消費
	end := size
	for end < len(rest) {
		nextR, nextSize := utf8.DecodeRuneInString(rest[end:])
		if nextR == utf8.RuneError {
			break
		}
		if isEmojiContinuation(nextR) {
			end += nextSize
			continue
		}
		break
	}

	emoji := rest[:end]
	// FE0F 単体は絵文字ではない
	if emoji == "\ufe0f" {
		return nil
	}
	s.advance(end)
	return withProp(NodeUnicodeEmoji, "emoji", emoji)
}

func (s *state) tryURL() *Node {
	if !(s.hasPrefix("https://") || s.hasPrefix("http://")) {
		return nil
	}
	save := s.pos
	start := s.pos

	// プロトコル部分を消費
	if s.hasPrefix("https://") {
		s.advance(8)
	} else {
		s.advance(7)
	}
	if s.eof() {
		s.pos = save
		return nil
	}

	// URL文字を消費 (括弧ネスト対応)
	s.consumeURLChars()
	url := s.src[start:s.pos]
	// 末尾の句読点を削る
	url = strings.TrimRight(url, ".,")
	s.pos = start + len(url)

	if len(url) <= 8 { // プロトコルのみ
		s.pos = save
		return nil
	}
	return withProp(NodeURL, "url", url)
}

func (s *state) consumeURLChars() {
	for !s.eof() {
		ch := s.peek()
		if unicode.IsSpace(ch) || ch == '<' || ch == '>' || ch == '"' || ch == '\'' {
			return
		}
		switch ch {
		case '(':
			s.advance(1)
			s.consumeURLChars()
			if !s.eof() && s.peek() == ')' {
				s.advance(1)
			}
			continue
		case '[':
			s.advance(1)
			s.consumeURLChars()
			if !s.eof() && s.peek() == ']' {
				s.advance(1)
			}
			continue
		case ')', ']':
			return
		}
		s.advance(utf8.RuneLen(ch))
	}
}

func (s *state) tryLink() *Node {
	// ?[label](url) or [label](url)
	silent := false
	if s.hasPrefix("?[") {
		silent = true
	} else if s.peek() != '[' {
		return nil
	}
	save := s.pos
	if silent {
		s.advance(2)
	} else {
		s.advance(1)
	}

	// label
	oldInLink := s.inLink
	s.inLink = true
	var labelNodes []*Node
	for !s.eof() && s.peek() != ']' {
		if s.peek() == '\n' {
			s.inLink = oldInLink
			s.pos = save
			return nil
		}
		n := s.parseOne()
		if n == nil {
			break
		}
		labelNodes = append(labelNodes, n)
	}
	s.inLink = oldInLink

	if !s.hasPrefix("](") {
		s.pos = save
		return nil
	}
	s.advance(2) // skip ](

	urlStart := s.pos
	parenDepth := 1
	for !s.eof() && parenDepth > 0 {
		ch := s.peek()
		if ch == '(' {
			parenDepth++
		} else if ch == ')' {
			parenDepth--
			if parenDepth == 0 {
				break
			}
		} else if ch == '\n' || unicode.IsSpace(ch) {
			s.pos = save
			return nil
		}
		s.advance(utf8.RuneLen(ch))
	}
	url := s.src[urlStart:s.pos]
	if url == "" || !s.hasPrefix(")") {
		s.pos = save
		return nil
	}
	s.advance(1) // skip )

	props := map[string]any{"url": url, "silent": silent}
	return &Node{Type: NodeLink, Props: props, Children: mergeText(labelNodes)}
}

func (s *state) trySearch() *Node {
	// 行頭から: "query 検索" / "query search" / "query [検索]" / "query [search]"
	if s.pos > 0 && s.src[s.pos-1] != '\n' {
		return nil
	}
	save := s.pos
	// 行末まで読む
	lineStart := s.pos
	for !s.eof() && s.peek() != '\n' {
		s.advance(utf8.RuneLen(s.peek()))
	}
	line := s.src[lineStart:s.pos]

	// 検索キーワードで終わるか確認
	trimmedLine := strings.TrimSpace(line)
	for _, suffix := range []string{" 検索", " search", " [検索]", " [search]", " Search", " SEARCH"} {
		trimmedSuffix := strings.TrimSpace(suffix)
		if strings.HasSuffix(trimmedLine, trimmedSuffix) {
			query := strings.TrimSpace(trimmedLine[:len(trimmedLine)-len(trimmedSuffix)])
			if query == "" {
				continue
			}
			if !s.eof() {
				s.advance(1) // skip \n
			}
			return &Node{Type: NodeSearch, Props: map[string]any{"query": query}}
		}
	}
	s.pos = save
	return nil
}

// --- Helpers ---

func (s *state) prevRune() rune {
	if s.pos <= 0 {
		return 0
	}
	r, _ := utf8.DecodeLastRuneInString(s.src[:s.pos])
	return r
}

func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func isASCIIAlphanumeric(r rune) bool {
	return isAlphanumeric(r)
}

func isAlphaSpaceOnly(s string) bool {
	for _, r := range s {
		if !isAlphanumeric(r) && !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func isDigitsOnly(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

func isEmojiName(s string) bool {
	for _, r := range s {
		if !isASCIIAlphanumeric(r) && r != '_' && r != '+' && r != '-' {
			return false
		}
	}
	return len(s) > 0
}

// isEmojiStart は絵文字シーケンスの先頭文字として妥当か判定する。
// Unicode Emoji カテゴリの文字や、一般的な絵文字開始コードポイントを対象。
func isEmojiStart(r rune) bool {
	// 基本的な絵文字範囲
	if r >= 0x1F600 && r <= 0x1F64F { // Emoticons
		return true
	}
	if r >= 0x1F300 && r <= 0x1F5FF { // Misc Symbols and Pictographs
		return true
	}
	if r >= 0x1F680 && r <= 0x1F6FF { // Transport and Map
		return true
	}
	if r >= 0x1F700 && r <= 0x1F77F { // Alchemical Symbols
		return true
	}
	if r >= 0x1F780 && r <= 0x1F7FF { // Geometric Shapes Extended
		return true
	}
	if r >= 0x1F800 && r <= 0x1F8FF { // Supplemental Arrows-C
		return true
	}
	if r >= 0x1F900 && r <= 0x1F9FF { // Supplemental Symbols and Pictographs
		return true
	}
	if r >= 0x1FA00 && r <= 0x1FA6F { // Chess Symbols
		return true
	}
	if r >= 0x1FA70 && r <= 0x1FAFF { // Symbols and Pictographs Extended-A
		return true
	}
	if r >= 0x2600 && r <= 0x26FF { // Misc symbols
		return true
	}
	if r >= 0x2700 && r <= 0x27BF { // Dingbats
		return true
	}
	if r >= 0x231A && r <= 0x23F3 { // Misc Technical (watch, hourglass)
		return true
	}
	if r >= 0x2300 && r <= 0x23FF { // Misc Technical
		return true
	}
	if r >= 0x2B05 && r <= 0x2B55 { // Arrows, geometric
		return true
	}
	if r >= 0x200D && r <= 0x200D { // ZWJ
		return true
	}
	if r >= 0xFE00 && r <= 0xFE0F { // Variation selectors
		return true
	}
	if r == 0x203C || r == 0x2049 { // ‼ ⁉
		return true
	}
	if r == 0x20E3 { // Combining enclosing keycap
		return true
	}
	if r >= 0x2100 && r <= 0x214F { // Letterlike symbols (™ etc)
		return true
	}
	if r >= 0x2190 && r <= 0x21FF { // Arrows
		return true
	}
	// keycap base digits 0-9, *, #
	if r == '#' || r == '*' || (r >= '0' && r <= '9') {
		// keycapとして使われる可能性があるが、FE0Fが続かないと絵文字にならない
		return false
	}
	// Regional indicators
	if r >= 0x1F1E0 && r <= 0x1F1FF {
		return true
	}
	// © ®
	if r == 0xA9 || r == 0xAE {
		return true
	}
	return false
}

// isEmojiContinuation は絵文字シーケンスの継続文字として妥当か判定する。
func isEmojiContinuation(r rune) bool {
	if isEmojiStart(r) {
		return true
	}
	// ZWJ (Zero Width Joiner)
	if r == 0x200D {
		return true
	}
	// Variation selectors
	if r >= 0xFE00 && r <= 0xFE0F {
		return true
	}
	// Combining enclosing keycap
	if r == 0x20E3 {
		return true
	}
	// Skin tone modifiers
	if r >= 0x1F3FB && r <= 0x1F3FF {
		return true
	}
	// Tag characters (used in flag sequences)
	if r >= 0xE0020 && r <= 0xE007F {
		return true
	}
	return false
}

// mergeText combines adjacent text nodes.
func mergeText(nodes []*Node) []*Node {
	if len(nodes) == 0 {
		return nil
	}
	var result []*Node
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if n.Type == NodeText && len(result) > 0 && result[len(result)-1].Type == NodeText {
			// 前のテキストノードと結合
			prev := result[len(result)-1]
			prev.Props["text"] = prev.textValue() + n.textValue()
		} else {
			result = append(result, n)
		}
	}
	return result
}

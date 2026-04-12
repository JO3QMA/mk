// Package mfm implements a parser and HTML renderer for Misskey Flavored
// Markdown (MFM). The parser is a recursive-descent port of mfm-js that
// produces an AST of Node values; ToHTML converts that AST to sanitised
// HTML suitable for ActivityPub Note.content.
package mfm

// NodeType identifies the kind of a parsed MFM node.
type NodeType string

const (
	NodeText         NodeType = "text"
	NodeBold         NodeType = "bold"
	NodeItalic       NodeType = "italic"
	NodeStrike       NodeType = "strike"
	NodeSmall        NodeType = "small"
	NodeCenter       NodeType = "center"
	NodePlain        NodeType = "plain"
	NodeInlineCode   NodeType = "inlineCode"
	NodeBlockCode    NodeType = "blockCode"
	NodeMathInline   NodeType = "mathInline"
	NodeMathBlock    NodeType = "mathBlock"
	NodeQuote        NodeType = "quote"
	NodeSearch       NodeType = "search"
	NodeURL          NodeType = "url"
	NodeLink         NodeType = "link"
	NodeMention      NodeType = "mention"
	NodeHashtag      NodeType = "hashtag"
	NodeUnicodeEmoji NodeType = "unicodeEmoji"
	NodeEmojiCode    NodeType = "emojiCode"
	NodeFn           NodeType = "fn"
)

// Node is an element in the MFM abstract syntax tree.
type Node struct {
	Type     NodeType
	Props    map[string]any
	Children []*Node
}

// Text creates a text node.
func Text(s string) *Node {
	return &Node{Type: NodeText, Props: map[string]any{"text": s}}
}

// textValue returns the text property if present.
func (n *Node) textValue() string {
	if n.Props == nil {
		return ""
	}
	v, _ := n.Props["text"].(string)
	return v
}

// withChildren returns a new node with the given children.
func withChildren(t NodeType, children []*Node) *Node {
	return &Node{Type: t, Children: children}
}

// withProp returns a new node with a single named property.
func withProp(t NodeType, key string, val any) *Node {
	return &Node{Type: t, Props: map[string]any{key: val}}
}

package hashtag

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtract(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "empty input",
			in:   []string{""},
			want: nil,
		},
		{
			name: "no hashtags",
			in:   []string{"hello world"},
			want: nil,
		},
		{
			name: "single tag",
			in:   []string{"hello #golang"},
			want: []string{"golang"},
		},
		{
			name: "tag at start",
			in:   []string{"#golang is fast"},
			want: []string{"golang"},
		},
		{
			name: "multiple tags",
			in:   []string{"learn #go and #rust today"},
			want: []string{"go", "rust"},
		},
		{
			name: "duplicate tags collapse",
			in:   []string{"#go again #go and #GO"},
			want: []string{"go"},
		},
		{
			name: "japanese tag",
			in:   []string{"おはよう #朝 みなさん"},
			want: []string{"朝"},
		},
		{
			name: "underscore and hyphen",
			in:   []string{"#hello_world #foo-bar"},
			want: []string{"hello_world", "foo-bar"},
		},
		{
			name: "no space before #",
			in:   []string{"foo#bar"},
			want: nil, // word boundary 必須
		},
		{
			name: "consecutive hashtags pick first only",
			in:   []string{"#one#two"},
			want: []string{"one"}, // upstream Misskey と同じく #two は拾わない
		},
		{
			name: "after quote",
			in:   []string{`"#quoted"`},
			want: []string{"quoted"},
		},
		{
			name: "hashtag-only line",
			in:   []string{"#alpha\n#beta"},
			want: []string{"alpha", "beta"},
		},
		{
			name: "case preserved on first occurrence",
			in:   []string{"#Golang and #golang"},
			want: []string{"Golang"},
		},
		{
			name: "text + cw",
			in:   []string{"hello #foo", "warning #bar"},
			want: []string{"foo", "bar"},
		},
		{
			name: "long tag truncated",
			in:   []string{"#" + strings.Repeat("a", MaxTagLength+50)},
			want: []string{strings.Repeat("a", MaxTagLength)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Extract(tc.in...)
			assert.Equal(t, tc.want, got)
		})
	}
}

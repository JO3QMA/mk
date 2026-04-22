package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPaginationOrder(t *testing.T) {
	cases := []struct {
		name           string
		sinceID        string
		untilID        string
		idColumn       string
		wantOrderByStr string
	}{
		{"sinceID only flips to ASC", "X", "", "id", "id ASC"},
		{"untilID only stays DESC", "", "Y", "id", "id DESC"},
		{"both specified stays DESC", "X", "Y", "id", "id DESC"},
		{"neither specified stays DESC", "", "", "id", "id DESC"},
		{"qualified column preserved in ASC", "X", "", `"note"."id"`, `"note"."id" ASC`},
		{"qualified column preserved in DESC", "", "", `"note"."id"`, `"note"."id" DESC`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantOrderByStr, paginationOrder(tc.sinceID, tc.untilID, tc.idColumn))
		})
	}
}

package chart

import "testing"

func TestColumnNameEncoding(t *testing.T) {
	cases := []struct {
		key, want string
	}{
		{"local.total", "___local_total"},
		{"local.diffs.withFile", "___local_diffs_withFile"},
		{"plain", "___plain"},
	}
	for _, c := range cases {
		if got := toColumnName(c.key); got != c.want {
			t.Errorf("toColumnName(%q) = %q, want %q", c.key, got, c.want)
		}
		// round-trip
		back, ok := fromColumnName(c.want)
		if !ok || back != c.key {
			t.Errorf("fromColumnName(%q) = %q, %v, want %q", c.want, back, ok, c.key)
		}
	}
}

func TestUniqueTempColumnNameEncoding(t *testing.T) {
	got := toUniqueTempColumnName("upv.user")
	if got != "unique_temp___upv_user" {
		t.Errorf("got %q", got)
	}
}

func TestFromColumnName_RejectsBadPrefix(t *testing.T) {
	if _, ok := fromColumnName("date"); ok {
		t.Errorf("expected fromColumnName(date) to fail")
	}
}

func TestTableNameTransform(t *testing.T) {
	cases := []struct {
		name, hour, day string
	}{
		{"notes", "__chart__notes", "__chart_day__notes"},
		{"perUserNotes", "__chart__per_user_notes", "__chart_day__per_user_notes"},
		{"apRequest", "__chart__ap_request", "__chart_day__ap_request"},
		{"activeUsers", "__chart__active_users", "__chart_day__active_users"},
	}
	for _, c := range cases {
		if got := HourTableName(c.name); got != c.hour {
			t.Errorf("HourTableName(%q) = %q, want %q", c.name, got, c.hour)
		}
		if got := DayTableName(c.name); got != c.day {
			t.Errorf("DayTableName(%q) = %q, want %q", c.name, got, c.day)
		}
	}
}

func TestSQLTypeOf(t *testing.T) {
	cases := []struct {
		r    ColRange
		want string
	}{
		{RangeMedium, "integer"},
		{RangeBig, "bigint"},
		{RangeSmall, "smallint"},
	}
	for _, c := range cases {
		if got := SQLTypeOf(c.r); got != c.want {
			t.Errorf("SQLTypeOf(%q) = %q, want %q", c.r, got, c.want)
		}
	}
}

func TestSchema_ColumnByName(t *testing.T) {
	s := Schema{
		Name: "x",
		Columns: []ColumnDef{
			{Name: "a"},
			{Name: "b", Accumulate: true},
		},
	}
	if c, ok := s.columnByName("b"); !ok || !c.Accumulate {
		t.Fatalf("lookup b failed: %v %v", c, ok)
	}
	if _, ok := s.columnByName("missing"); ok {
		t.Fatalf("expected missing to be absent")
	}
}

func TestSchema_UniqueColumnNames(t *testing.T) {
	s := Schema{
		Columns: []ColumnDef{
			{Name: "a"},
			{Name: "b", UniqueIncrement: true},
			{Name: "c", UniqueIncrement: true},
		},
	}
	got := s.uniqueColumnNames()
	if len(got) != 2 || got[0] != "b" || got[1] != "c" {
		t.Fatalf("got %v", got)
	}
}

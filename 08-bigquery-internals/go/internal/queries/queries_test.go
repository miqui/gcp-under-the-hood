package queries

import (
	"strings"
	"testing"
)

func TestTable(t *testing.T) {
	got, err := Table("my-proj", "ds", "t1")
	if err != nil || got != "`my-proj.ds.t1`" {
		t.Fatalf("got %q err %v", got, err)
	}
	for _, bad := range []string{"a`b", "a b", "", "a;drop"} {
		if _, err := Table(bad, "ds", "t"); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestLoadSQL(t *testing.T) {
	s := LoadSQL("`p.d.t`", 5000)
	for _, want := range []string{"INSERT INTO `p.d.t`", PublicSource, "LIMIT 5000"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
}

func TestScenarios(t *testing.T) {
	sc := Scenarios("`f`", "`p`", "`c`")
	if len(sc) != 6 {
		t.Fatalf("len %d", len(sc))
	}
	if !strings.Contains(sc[3].SQL, "`p`") || !strings.Contains(sc[4].SQL, "`c`") {
		t.Error("wrong tables")
	}
	if sc[0].Label == "" {
		t.Error("empty label")
	}
}

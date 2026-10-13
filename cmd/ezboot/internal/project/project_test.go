package project

import (
	"testing"
)

func TestPruneDBBlocks(t *testing.T) {
	template := "# header\n" +
		"# ezboot:with-db:begin\n" +
		"  db_migration:\n" +
		"    auto_migrate: true\n" +
		"# ezboot:with-db:end\n" +
		"tail\n"

	cases := []struct {
		name   string
		withDB bool
		want   string
	}{
		{
			name:   "keep block",
			withDB: true,
			want:   "# header\n  db_migration:\n    auto_migrate: true\ntail\n",
		},
		{
			name:   "drop block",
			withDB: false,
			want:   "# header\ntail\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(pruneDBBlocks([]byte(template), c.withDB)); got != c.want {
				t.Fatalf("pruneDBBlocks() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestPruneDBBlocksWithoutMarkers(t *testing.T) {
	data := []byte("name: myna\nver: 1.0.0\n")
	if got := pruneDBBlocks(data, false); string(got) != string(data) {
		t.Fatalf("pruneDBBlocks() should not change data without markers")
	}
}

func TestIsDBOnlyPath(t *testing.T) {
	db := []string{"dbgen/mods/myna/mod.go", "schemas/000001_example.up.sql", "internal/dao/example.dao.go"}
	for _, p := range db {
		if !isDBOnlyPath(p) {
			t.Errorf("isDBOnlyPath(%q) = false, want true", p)
		}
	}

	keep := []string{"internal/cli/cli.go", "cmd/myna/main.go", "dbgenx/file", "internal/database/x.go"}
	for _, p := range keep {
		if isDBOnlyPath(p) {
			t.Errorf("isDBOnlyPath(%q) = true, want false", p)
		}
	}
}

func TestRenderRenamesDBPaths(t *testing.T) {
	params := buildParams("demo")
	got := string(render([]byte("dbgen/mods/myna/mod.go"), params))
	if got != "dbgen/mods/demo/mod.go" {
		t.Fatalf("render() = %q, want %q", got, "dbgen/mods/demo/mod.go")
	}
}

package project

import (
	"testing"
)

func TestRenderRenamesDBPaths(t *testing.T) {
	params := buildParams("demo")
	got := string(render([]byte("dbgen/mods/myna/mod.go"), params))
	if got != "dbgen/mods/demo/mod.go" {
		t.Fatalf("render() = %q, want %q", got, "dbgen/mods/demo/mod.go")
	}
}

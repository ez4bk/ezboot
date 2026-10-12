package rbac

import (
	"net/http"
	"slices"
	"testing"

	"github.com/casbin/casbin/v3/model"
	stringadapter "github.com/casbin/casbin/v3/persist/string-adapter"
)

func TestEnforcerWithV3Adapter(t *testing.T) {
	adapter := stringadapter.NewAdapter(
		"p, Role:Anonymous, /public, GET, allow\n" +
			"g, alice, Role:Superuser",
	)
	enforcer, err := DefaultModel(adapter)
	if err != nil {
		t.Fatal(err)
	}

	loadPolicy := func(m model.Model) []*Def {
		m.AddPolicy("p", "p", []string{
			"Role:Operator", "/api/items", http.MethodGet, "allow",
		})
		m.AddPolicy("g", "g", []string{"bob", "Role:Editor"})
		m.AddPolicy("g", "g", []string{"Role:Editor", "Role:Operator"})
		return []*Def{
			{
				Name: "Role:Editor",
				Desc: "Edits items",
				Api: []*Api{
					{Method: http.MethodGet, Uri: "/api/items"},
				},
			},
			{
				Name: "Role:Operator",
				Desc: "Operates items",
				Api: []*Api{
					{Method: http.MethodGet, Uri: "/api/items"},
				},
			},
		}
	}

	if err := enforcer.LoadSysPolicy(loadPolicy); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name    string
		subject string
		object  string
		action  string
		allowed bool
	}{
		{name: "inheritance", subject: "bob", object: "/api/items", action: http.MethodGet, allowed: true},
		{name: "anonymous", subject: "guest", object: "/public", action: http.MethodGet, allowed: true},
		{name: "superuser", subject: "alice", object: "/anywhere", action: http.MethodDelete, allowed: true},
		{name: "missing policy", subject: "bob", object: "/api/items", action: http.MethodDelete, allowed: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			allowed, matched, err := enforcer.EnforceEx(
				testCase.subject,
				testCase.object,
				testCase.action,
			)
			if err != nil {
				t.Fatal(err)
			}
			if allowed != testCase.allowed {
				t.Fatalf("allowed = %v, want %v", allowed, testCase.allowed)
			}
			if len(matched) == 0 && testCase.allowed {
				t.Fatal("allowed decision has no matched rule")
			}
		})
	}

	defs, err := enforcer.GetRoleDefForUser("bob")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	slices.Sort(names)
	wantNames := []string{"Role:Editor", "Role:Operator"}
	if !slices.Equal(names, wantNames) {
		t.Fatalf("role definitions = %v, want %v", names, wantNames)
	}
}

package enforce

import (
	"net/http"
	"testing"

	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"

	bar "github.com/ez4bk/ezboot/cmd/protoc-gen-go-rbac/testdata/rbac"
	"github.com/ez4bk/ezboot/rbac"
)

var methods = []string{
	http.MethodGet,
	http.MethodPut,
	http.MethodPost,
	http.MethodDelete,
	http.MethodPatch,
}

var apis = []string{
	"/api/v1/resource",
	"/api/v1/resource/qqq",
	"/api/v1/resource/123/types",
}

var users = []string{
	"alice",
	"bob",
	"root",
	"guest",
}

func Test_Enforcer(t *testing.T) {
	e, err := rbac.DefaultModel(fileadapter.NewAdapter("policy.csv"))
	if err != nil {
		t.Fatal(err)
	}

	if err = e.LoadSysPolicy(bar.LoadSimpleServicePolicy); err != nil {
		t.Fatal(err)
	}

	for _, user := range users {
		for _, api := range apis {
			for _, method := range methods {
				if ok, err := e.Enforce(user, api, method); err == nil {
					t.Logf("sub = %-10q, obj = %-25q, act = %-8q => %v", user, api, method, ok)
				} else {
					t.Fatal(err)
				}
			}
		}
		t.Log()
	}
}

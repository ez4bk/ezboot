package gengo

import (
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/ez4bk/ezboot/rbac"
)

func ShouldGenerate(file *protogen.File) bool {
	for _, srv := range file.Services {
		if ServiceHierarchy(srv) != nil {
			return true
		}
		for _, method := range srv.Methods {
			if MethodHTTPRule(method) != nil && MethodRBACRole(method) != nil {
				return true
			}
		}
	}
	return false
}

func MethodHTTPRule(m *protogen.Method) *annotations.HttpRule {
	return AsExtension[*annotations.HttpRule](m.Desc, annotations.E_Http)
}

func MethodRBACRole(m *protogen.Method) *rbac.Role {
	return AsExtension[*rbac.Role](m.Desc, rbac.E_Role)
}

func ServiceHierarchy(srv *protogen.Service) *rbac.Hierarchy {
	return AsExtension[*rbac.Hierarchy](srv.Desc, rbac.E_Hierarchy)
}

func AsExtension[T any](desc protoreflect.Descriptor, ext protoreflect.ExtensionType) T {
	if opt := desc.Options(); opt != nil {
		if ans, ok := proto.GetExtension(opt, ext).(T); ok {
			return ans
		}
	}

	var zero T
	return zero
}

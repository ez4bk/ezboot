{{ $pkgModel := .Pkg.model }}
{{ $pkgRbacDef := .Pkg.rbacDef }}
{{ $pkgRbacApi := .Pkg.rbacApi }}

{{- range .Services }}
{{ $policy := .Policy}}

var _{{.Name}}RoleDef = []*{{$pkgRbacDef}}{
{{- range $name, $desc := .RoleDef }}
    {
        Name: "{{$name}}",
        Desc: "{{$desc}}",
        Api: []*{{$pkgRbacApi}}{
            {{- range $policy }}
            {{- if and (eq $name .Role) (not .Deny) }}
            {Method: "{{.Method}}", Uri: "{{.Uri}}"},
            {{- end }}
            {{- end }}
        },
    },
{{- end }}
}

func Load{{- .Name -}}Policy(m {{$pkgModel}}) []*{{$pkgRbacDef}} {
    // Role binding
    {{- range .Policy }}
    m.AddPolicy("p", "p", []string{"{{.Role}}", "{{.Uri}}", "{{.Method}}", "{{if .Deny}}deny{{else}}allow{{end}}" })
    {{- end }}

    // Role hierarchy
    {{- range .Hierarchy }}
        {{- $role := .Role }}

        {{- range .Extends }}
        m.AddPolicy("g", "g", []string{"{{$role}}", "{{.}}"})
        {{- end }}
    {{- end }}

    return _{{.Name}}RoleDef
}
{{- end }}

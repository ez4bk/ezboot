package template

import (
	"embed"
	"io"
	"text/template"
)

//go:embed *.tpl
var fs embed.FS

type Template = template.Template

func LoadTemplate() (*Template, error) {
	fp, err := fs.Open("loader.tpl")
	if err != nil {
		return nil, err
	}
	defer func() { _ = fp.Close() }()

	content, err := io.ReadAll(fp)
	if err != nil {
		return nil, err
	}

	return template.New("loader").Parse(string(content))
}

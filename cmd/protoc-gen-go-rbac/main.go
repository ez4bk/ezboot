package main

import (
	"flag"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/ez4bk/ezboot/cmd/protoc-gen-go-rbac/internal/gengo"
)

func main() {
	protogen.Options{
		ParamFunc: flag.CommandLine.Set,
	}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = gengo.SupportedFeatures
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}

			gengo.GenerateFile(gen, f)
		}

		return nil
	})
}

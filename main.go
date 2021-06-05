package main

import (
	"fmt"
	"go/types"
	"os"
	"strings"
	"sync"
	"text/template"

	"golang.org/x/tools/go/packages"
)

func main() {
	if err := func() error {
		var (
			path string
		)
		// if len(os.Args) < 2 && os.Getenv("GOPACKAGE") == "" {
		// 	return fmt.Errorf("specify package path")
		// }
		// if p := os.Getenv("GOPACKAGE"); p != "" {
		// 	path = p
		// }
		if len(os.Args) > 1 {
			path = os.Args[1]
		}
		if path == "" {
			return fmt.Errorf("specify package path")
		}
		fmt.Printf("package: %q\n", path)
		pkg, err := loadPkg(path)
		if err != nil {
			return err
		}
		fmt.Printf("type info: %v\n", pkg.TypesInfo)
		for ident, def := range pkg.TypesInfo.Defs {
			if !ident.IsExported() {
				continue
			}
			if !strings.HasSuffix(ident.Name, "Style") {
				continue
			}
			obj, ok := def.Type().Underlying().(*types.Struct)
			if !ok {
				continue
			}
			for ii := 0; ii < obj.NumFields(); ii++ {
				var (
					field = obj.Field(ii)
				)
				if !field.Exported() {
					continue
				}
				fmt.Printf("%s\n", &Setter{
					StructType: ident.Name,
					FieldName:  field.Name(),
					FieldType:  field.Type().String(),
					Receiver:   "style",
					Argument:   string(strings.ToLower(field.Name())[0]),
				})
			}
		}
		return nil
	}(); err != nil {
		fmt.Printf("error: %v", err)
	}
}

// Setter generates a fluent method.
type Setter struct {
	StructType string
	FieldName  string
	FieldType  string
	Receiver   string
	Argument   string

	init sync.Once
	t    *template.Template
	err  error
}

func (s *Setter) String() string {
	s.init.Do(func() {
		s.t, s.err = template.New("").Parse(strings.TrimSpace(`
			func ({{.Receiver}} {{.StructType}}) With{{.FieldName}}({{.Argument}} {{.FieldType}}) {{.StructType}} {
				{{.Receiver}}.{{.FieldName}} = {{.Argument}}
				return {{.Receiver}}
			}
		`))
	})
	if s.t == nil {
		return ""
	}
	b := strings.Builder{}
	s.err = s.t.Execute(&b, s)
	return b.String()
}

func (s *Setter) Err() error {
	return s.err
}

func loadPkg(path string) (*packages.Package, error) {
	pkgs, err := packages.Load(
		&packages.Config{
			Mode: packages.NeedTypes | packages.NeedImports | packages.NeedSyntax | packages.NeedDeps,
		},
		path,
	)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		os.Exit(1)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("package not found")
	}
	return pkgs[0], nil
}

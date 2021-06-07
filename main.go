package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func main() {
	if err := func() error {
		var (
			// path to directory containing go files.
			path string
			// out path to generated file eg "fluent.go".
			out string
			// w is the output writer. Points to stdout when no output file is specified.
			w io.Writer
		)
		if len(os.Args) > 1 {
			path = os.Args[1]
		}
		if len(os.Args) > 2 {
			out = os.Args[2]
		}
		if path == "" {
			return fmt.Errorf("specify directory path")
		}
		if out == "" {
			w = os.Stdout
		} else {
			outf, err := os.Create(out)
			if err != nil {
				return fmt.Errorf("creating output file: %w", err)
			}
			defer outf.Close()
			w = outf
		}
		// Note(jfm): Should we assume the directory matches the package name? This is common
		// convention, but is not guaranteed.
		fmt.Fprintf(w, "package %s\n\n", strings.Split(filepath.Base(path), ".")[0])
		entries, err := ioutil.ReadDir(path)
		if err != nil {
			return fmt.Errorf("reading dir: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if filepath.Ext(entry.Name()) != ".go" {
				continue
			}
			if strings.Contains(entry.Name(), "fluent") {
				continue
			}
			if err := func() error {
				srcf, err := os.Open(filepath.Join(path, entry.Name()))
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				defer srcf.Close()
				fragments, err := Process(srcf)
				if err != nil {
					return fmt.Errorf("generating fluent methods: %w", err)
				}
				for _, fragment := range fragments {
					if _, err := fmt.Fprint(w, fragment); err != nil {
						return fmt.Errorf("writing source code fragments to file: %w", err)
					}
				}
				return nil
			}(); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		fmt.Printf("error: %v", err)
	}
}

// MethodTemplate used to generate fluent setter methods.
var MethodTemplate = template.Must(template.New("").Parse(strings.TrimSpace(`
{{.Doc}}
func ({{.Receiver}} {{.StructType}}) With{{.FieldIdent}}({{.Argument}} {{.FieldType}}) {{.StructType}} {
	{{.Receiver}}.{{.FieldIdent}} = {{.Argument}}
	return {{.Receiver}}
}`)))

// Process parses source code from `r` and returns source code fragments implementing fluent setter
// methods for each viable struct definition.
func Process(r io.Reader) (list []string, err error) {
	src, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing file: %w", err)
	}
	ast.Walk(StructVisitor{Suffix: "Style", Callback: func(s Struct) {
		if err != nil {
			return
		}
		fragment, e := s.GenerateFluentMethods()
		if e != nil {
			err = fmt.Errorf("generating method for struct %q: %w", s.Type, e)
		}
		if fragment != "" {
			list = append(list, fragment)
		}
	}}, f)
	return list, err
}

// StructVisitor visits struct definitions on an ast.
type StructVisitor struct {
	// Suffix to match against. Could use regex for more generic approach.
	Suffix string
	// Callback is invoked for each struct definition.
	Callback func(Struct)
}

func (sv StructVisitor) Visit(n ast.Node) ast.Visitor {
	if sv.Callback == nil {
		return nil
	}
	t, ok := n.(*ast.TypeSpec)
	if !ok {
		return sv
	}
	if !strings.HasSuffix(t.Name.String(), sv.Suffix) {
		return sv
	}
	obj, ok := t.Type.(*ast.StructType)
	if !ok {
		return sv
	}
	var fields []Field
	for _, field := range obj.Fields.List {
		if isEmbedded := len(field.Names) == 0; isEmbedded {
			continue
		}
		for _, ident := range field.Names {
			if !ident.IsExported() {
				continue
			}
			var ft string
			if ptr, ok := field.Type.(*ast.StarExpr); ok {
				ft = "*"
				field.Type = ptr.X
			}
			switch n := field.Type.(type) {
			case *ast.Ident:
				ft += n.Name
			case *ast.SelectorExpr:
				if pkg, ok := n.X.(*ast.Ident); ok {
					ft += fmt.Sprintf("%s.%s", pkg.Name, n.Sel.Name)
				}
			}
			fields = append(fields, Field{
				Type:       ft,
				Identifer:  ident.Name,
				DocComment: strings.TrimSpace(field.Doc.Text()),
			})
		}
	}
	sv.Callback(Struct{
		Type:   t.Name.String(),
		Fields: fields,
	})
	return sv
}

// Struct specifies a struct definition.
type Struct struct {
	Type   string
	Fields []Field
}

// Field specifies a struct field definition.
type Field struct {
	Type       string
	Identifer  string
	DocComment string
}

// GenerateFluentMethods produces source code fragments that implement fluent setter methods for
// the struct definition.
func (s Struct) GenerateFluentMethods() (string, error) {
	var b strings.Builder
	for _, field := range s.Fields {
		if err := MethodTemplate.Execute(&b, struct {
			StructType string
			FieldType  string
			FieldIdent string
			Receiver   string
			Argument   string
			Doc        string
		}{
			StructType: s.Type,
			FieldType:  field.Type,
			FieldIdent: field.Identifer,
			Receiver:   "style",
			Argument: func() string {
				if isPointer := field.Type[0] == '*'; isPointer {
					return strings.ToLower(string(field.Type[1]))
				}
				return strings.ToLower(string(field.Type[0]))
			}(),
			Doc: func() string {
				if field.DocComment != "" {
					return fmt.Sprintf("// With%s", strings.Join(strings.Split(field.DocComment, "\n"), "\n// "))
				}
				return ""
			}(),
		}); err != nil {
			return "", fmt.Errorf("executing template: %w", err)
		}
		if _, err := fmt.Fprint(&b, "\n"); err != nil {
			return "", fmt.Errorf("adding spacer: %w", err)
		}
	}
	return b.String(), nil
}

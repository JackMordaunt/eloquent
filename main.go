package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"text/template"
	"unicode"

	"github.com/davecgh/go-spew/spew"
)

func init() {
	spew.Config = spew.ConfigState{
		Indent:            "    ",
		DisableMethods:    true,
		DisableCapacities: true,
	}
}

func main() {
	if err := func() error {
		var (
			path string
		)
		if len(os.Args) > 1 {
			path = os.Args[1]
		}
		if path == "" {
			return fmt.Errorf("specify package path")
		}
		src, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}
		out, err := Generate(string(src))
		if err != nil {
			return fmt.Errorf("generating fluent methods: %w", err)
		}
		return ioutil.WriteFile(fmt.Sprintf("%s_fluent.go", path), []byte(out), 0665)
	}(); err != nil {
		fmt.Printf("error: %v", err)
	}
}

// Generate fluent methods for struct definitions in the given source.
func Generate(src string) (string, error) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "", src, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parsing file: %w", err)
	}
	var b strings.Builder
	ast.Walk(StructVisitor{Suffix: "Style", Wr: &b}, f)
	return b.String(), nil
}

// Setter generates a fluent method.
type Setter struct {
	StructType string
	FieldName  string
	FieldType  string
	Receiver   string
	Argument   string
	Doc        string

	init sync.Once
	t    *template.Template
	err  error
}

func (s *Setter) String() string {
	s.init.Do(func() {
		s.t, s.err = template.New("").Parse(strings.TrimSpace(`
// {{.Doc}}
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

// StructVisitor visits struct definitions and generates Fluent method stubs.
type StructVisitor struct {
	// Suffix to match against. Could use regex for more generic approach.
	Suffix string
	// Wr to write generated methods into. Typically a file handle.
	Wr io.Writer
}

func (sv StructVisitor) Visit(n ast.Node) ast.Visitor {
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
	for _, field := range obj.Fields.List {
		if isEmbedded := len(field.Names) == 0; isEmbedded {
			continue
		}
		var (
			structType   = t.Name.String()
			fieldName    = field.Names[0].String()
			fieldType    = fmt.Sprintf("%s", field.Type)
			fieldComment = strings.TrimSpace(field.Doc.Text())
		)
		if isExported := unicode.IsUpper(rune(fieldName[0])); !isExported {
			continue
		}
		fmt.Fprint(sv.Wr, &Setter{
			StructType: structType,
			FieldName:  fieldName,
			FieldType:  fieldType,
			Receiver:   "style",
			Argument:   string(strings.ToLower(fieldType)[0]),
			Doc:        fmt.Sprintf("With%s", fieldComment),
		})
		fmt.Fprint(sv.Wr, "\n\n")
	}
	return sv
}

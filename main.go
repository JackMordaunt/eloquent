package main

import (
	"fmt"
	"go/ast"
	"go/parser"
)

func main() {
	var (
		g Generator
	)
	if err := func() error {
		f, err := parser.ParseFile(nil, "", nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parsing file: %w", err)
		}
		ast.Walk(g, f)
		return nil
	}(); err != nil {
		fmt.Printf("error: %v", err)
	}
}

// Generator visits ast nodes and creates a fluent method for each public struct
// field.
type Generator struct {
}

func (g Generator) Visit(n ast.Node) ast.Visitor {
	return g
}

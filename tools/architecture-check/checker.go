package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

func checkFile(path string) []string {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return []string{err.Error()}
	}
	var problems []string
	types := 0
	namedStructs := map[*ast.StructType]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if t, ok := n.(*ast.TypeSpec); ok {
			types++
			if st, ok := t.Type.(*ast.StructType); ok {
				namedStructs[st] = true
			}
		}
		return true
	})
	if types > 1 {
		problems = append(problems, fmt.Sprintf("%s: at most one named type per file (found %d)", path, types))
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if st, ok := n.(*ast.StructType); ok && len(st.Fields.List) > 0 && !namedStructs[st] {
			problems = append(problems, path+": anonymous model struct")
		}
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Body != nil {
			ast.Inspect(fn.Body, func(inner ast.Node) bool {
				if _, ok := inner.(*ast.TypeSpec); ok {
					problems = append(problems, path+": local named type")
				}
				return true
			})
			return true
		}
		return true
	})
	for _, imp := range f.Imports {
		name, _ := strconv.Unquote(imp.Path.Value)
		if forbiddenImport(path, name) {
			problems = append(problems, fmt.Sprintf("%s: forbidden import %s", path, name))
		}
	}
	return problems
}

func forbiddenImport(path, name string) bool {
	if strings.Contains(path, "/domain/") {
		if name == "database/sql" || name == "net/http" || strings.Contains(name, "/infrastructure/") || strings.Contains(name, "sqlite") || strings.Contains(name, "redis") || strings.Contains(name, "singleflight") {
			return true
		}
	}
	if strings.Contains(path, "/application/") && strings.Contains(name, "/infrastructure/") {
		return true
	}
	if strings.HasPrefix(path, "internal/api/") && strings.Contains(name, "/internal/worker") {
		return true
	}
	if strings.HasPrefix(path, "internal/worker/") && strings.Contains(name, "/internal/api/") {
		return true
	}
	return strings.HasPrefix(path, "internal/fetch/") && strings.Contains(name, "/internal/notification/")
}

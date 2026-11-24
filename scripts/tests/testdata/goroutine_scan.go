// Package main is an AST scanner for scripts/check-goroutine-safety.sh.
// It lives under testdata/ so `go build ./...` and `go test ./...` ignore it.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: goroutine_scan.go <dir-or-file>...")
		os.Exit(2)
	}
	for _, arg := range os.Args[1:] {
		info, err := os.Stat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "stat %s: %v\n", arg, err)
			os.Exit(2)
		}
		if !info.IsDir() {
			scanFile(arg)
			continue
		}
		err = filepath.Walk(arg, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				name := info.Name()
				if path != arg && (name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
				scanFile(path)
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "walk %s: %v\n", arg, err)
			os.Exit(2)
		}
	}
}

func scanFile(path string) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse %s: %v\n", path, err)
		os.Exit(2)
	}
	source, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
		os.Exit(2)
	}
	lines := strings.Split(string(source), "\n")
	ast.Inspect(file, func(node ast.Node) bool {
		stmt, ok := node.(*ast.GoStmt)
		if !ok {
			return true
		}
		pos := fset.Position(stmt.Pos())
		fun := stmt.Call.Fun
		for {
			paren, ok := fun.(*ast.ParenExpr)
			if !ok {
				break
			}
			fun = paren.X
		}
		kind := "call"
		switch fun.(type) {
		case *ast.FuncLit:
			kind = "func"
		case *ast.SelectorExpr:
			kind = "selector"
		}
		text := ""
		if pos.Line > 0 && pos.Line <= len(lines) {
			text = strings.TrimSpace(lines[pos.Line-1])
		}
		fmt.Printf("%s:%d:%d\t%s\t%s\n", filepath.ToSlash(path), pos.Line, pos.Column, kind, text)
		return true
	})
}

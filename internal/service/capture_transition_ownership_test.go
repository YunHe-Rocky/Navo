package service

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func TestPrepareTUNLetsCoreOwnAdapterLifecycle(t *testing.T) {
	path := filepath.Join("capture_transition.go")
	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	var function *ast.FuncDecl
	for _, declaration := range parsed.Decls {
		candidate, ok := declaration.(*ast.FuncDecl)
		if ok && candidate.Name.Name == "prepareTUNLocked" {
			function = candidate
			break
		}
	}
	if function == nil {
		t.Fatal("prepareTUNLocked was not found")
	}

	var startPosition, waitPosition token.Pos
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if selector.Sel.Name == "startCoreForCapture" {
			startPosition = selector.Pos()
		}
		if selector.Sel.Name == "WaitForAdapterState" {
			waitPosition = selector.Pos()
		}
		owner, ok := selector.X.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		receiver, ok := owner.X.(*ast.Ident)
		if receiver != nil && ok &&
			receiver.Name == "s" &&
			owner.Sel.Name == "tunManager" &&
			(selector.Sel.Name == "Create" || selector.Sel.Name == "Configure") {
			t.Errorf("prepareTUNLocked must not call tunManager.%s before sing-tun starts", selector.Sel.Name)
		}
		return true
	})

	if startPosition == token.NoPos || waitPosition == token.NoPos {
		t.Fatalf("startup order is incomplete: start=%v wait=%v", startPosition, waitPosition)
	}
	if startPosition >= waitPosition {
		t.Fatal("sing-box must start and create the adapter before readiness is checked")
	}
}

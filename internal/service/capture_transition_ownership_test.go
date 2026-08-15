package service

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func TestPrepareTUNOrdersOwnershipActivationVerificationAndCommit(t *testing.T) {
	path := filepath.Join("capture_transition.go")
	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var function *ast.FuncDecl
	for _, declaration := range parsed.Decls {
		if candidate, ok := declaration.(*ast.FuncDecl); ok && candidate.Name.Name == "prepareTUNLocked" {
			function = candidate
			break
		}
	}
	if function == nil {
		t.Fatal("prepareTUNLocked was not found")
	}

	var ownerPosition, startPosition, activatePosition, verifyPosition, commitPosition token.Pos
	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.AssignStmt:
			for index, left := range value.Lhs {
				selector, ok := left.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "networkManager" {
					continue
				}
				receiver, _ := selector.X.(*ast.Ident)
				if receiver == nil || receiver.Name != "s" || index >= len(value.Rhs) {
					continue
				}
				if identifier, ok := value.Rhs[index].(*ast.Ident); ok && identifier.Name == "manager" {
					ownerPosition = value.Pos()
				}
			}
		case *ast.CallExpr:
			selector, ok := value.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch selector.Sel.Name {
			case "startCoreForCapture":
				startPosition = selector.Pos()
			case "Activate":
				activatePosition = selector.Pos()
			case "Verify":
				verifyPosition = selector.Pos()
			case "commitHealthyRuntime":
				commitPosition = selector.Pos()
			}
		}
		return true
	})
	for name, position := range map[string]token.Pos{"owner": ownerPosition, "start": startPosition, "activate": activatePosition, "verify": verifyPosition, "commit": commitPosition} {
		if position == token.NoPos {
			t.Fatalf("%s step is missing", name)
		}
	}
	if !(ownerPosition < startPosition && startPosition < activatePosition && activatePosition < verifyPosition && verifyPosition < commitPosition) {
		t.Fatalf("invalid TUN order: owner=%d start=%d activate=%d verify=%d commit=%d", ownerPosition, startPosition, activatePosition, verifyPosition, commitPosition)
	}
}

func TestStartCoreForCaptureDoesNotCommitHealth(t *testing.T) {
	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, "capture_transition.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "startCoreForCapture" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if ok && selector.Sel.Name == "commitHealthyRuntime" {
				t.Error("startCoreForCapture still commits health before TUN verification")
			}
			return true
		})
		return
	}
	t.Fatal("startCoreForCapture was not found")
}

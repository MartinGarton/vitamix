// Copyright 2012 Petar Maymounkov. All rights reserved.
// Use of this source code is governed by a 
// license that can be found in the LICENSE file.

package vrewrite

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
)

func rewriteChanOps(fset *token.FileSet, file *ast.File) bool {
	needVtime, err := rewrite(fset, file)
	if err != nil {
		//fmt.Fprintf(os.Stderr, "Rewrite errors parsing '%s':\n%s\n", file.Name.Name, err)
		fmt.Fprintf(os.Stderr, "—— Encountered errors while parsing\n")
	}
	return needVtime
}

// Rewrite creates a new rewriting frame
func rewrite(fset *token.FileSet, node ast.Node) (bool, error) {
	rwv := &rewriteVisitor{}
	rwv.frame.Init(fset)
	ast.Walk(rwv, node)
	return rwv.NeedPkgVtime, rwv.Error()
}


// recurseRewrite creates a new rewriting frame as a callee from the frame caller
func recurseRewrite(caller framed, node ast.Node) (bool, error) {
	rwv := &rewriteVisitor{}
	rwv.frame.InitRecurse(caller)
	ast.Walk(rwv, node)
	return rwv.NeedPkgVtime, rwv.Error()
}

// rewriteVisitor is an AST frame that traverses down the AST until it hits a block
// statement, within which it rewrites the statement-level channel operations. 
// This visitor itself does not traverse below the statements of the block statement.
// It does however call another visitor type to continue below those statements.
type rewriteVisitor struct {
	NeedPkgVtime bool
	frame
}

// Frame implements framed.Frame
func (t *rewriteVisitor) Frame() *frame {
	return &t.frame
}

// Visit implements ast.Visitor's Visit method
func (t *rewriteVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return t
	}
	bstmt, ok := node.(*ast.BlockStmt)
	// If node is not a block statement, it means we are recursing down the
	// AST and we haven't hit a block statement yet. 
	if !ok {
		// Keep recursing
		return t
	}

	// Rewrite each statement of a block statement and stop the recursion of this visitor
	var list []ast.Stmt
	for _, stmt := range bstmt.List {
		// Continue the walk recursively below this stmt
		needVtime, err := recurseRewrite(t, stmt)
		if err != nil {
			t.errs.Add(err)
		}
		t.NeedPkgVtime = t.NeedPkgVtime || needVtime
		list = append(list, stmt)
	}
	bstmt.List = list

	// Do not continue the parent walk recursively
	return nil
}


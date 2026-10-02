// Package main implements droppedvalue, a pure-AST check that fails the build
// on values assigned to blank identifiers.
//
// Every blank in the left-hand side of an assignment or short-variable
// declaration is flagged, including in if/switch/for initializers. Two
// patterns are deliberately exempt because they are structural, not value
// drops: the key (and key/value) bindings of a for-range statement, and
// compile-time assertions written as var declarations (var _ T = ...).
// Generated files are skipped entirely.
//
// A deliberate drop can be justified with a reason:
//
//	_, _ = w.Write(b) //nolint:droppedvalue -- Write returns count and error, http.Error reports failure
//
// The marker must carry a reason; a bare //nolint:droppedvalue is itself a
// diagnostic, so dropping the reason fails the build.
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Marker is the comment directive that whitelists a dropped value, and must be
// followed by a "-- reason".
const Marker = "//nolint:droppedvalue"

// Analyzer reports values assigned to blank identifiers.
var Analyzer = &analysis.Analyzer{
	Name: "droppedvalue",
	Doc:  "report values assigned to blank identifiers (dropped values)",
	Run:  run,
}

var reasonRe = regexp.MustCompile(`^//nolint:droppedvalue(?:,\s*\w+)*\s+--\s+(.+)$`)

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		if ast.IsGenerated(file) {
			continue
		}
		comments := collectComments(pass, file)
		for _, assign := range findAssignments(file) {
			line := pass.Fset.Position(assign.Pos()).Line
			if n := droppedBlanks(assign); n > 0 {
				reportDrop(pass, comments, assign, line, n)
			}
		}
	}
	return nil, nil
}

// reportDrop reports an assignment that drops n values. A marker on the same
// line or the line directly above suppresses the diagnostic only when it
// carries a reason; a bare marker is additionally reported as requiring one and
// does not suppress the drop, so it cannot be used as a silent bypass.
func reportDrop(pass *analysis.Pass, comments []commentLine, assign *ast.AssignStmt, line, n int) {
	marker := markerNear(comments, line)
	if marker == nil {
		pass.Report(analysis.Diagnostic{
			Pos:     assign.Pos(),
			Message: fmt.Sprintf("assigned value is dropped (%d blank identifier(s)); assign it to a named identifier or justify with %s -- <reason>", n, Marker),
		})
		return
	}
	if reasonRe.MatchString(strings.TrimSpace(marker.text)) {
		return // justified drop
	}
	pass.Report(analysis.Diagnostic{
		Pos:     marker.pos,
		Message: fmt.Sprintf("%s requires a reason: %s -- <reason>", Marker, Marker),
	})
	pass.Report(analysis.Diagnostic{
		Pos:     assign.Pos(),
		Message: fmt.Sprintf("assigned value is dropped (%d blank identifier(s)); assign it to a named identifier or justify with %s -- <reason>", n, Marker),
	})
}

// markerNear returns the droppedvalue marker closest to the given source line,
// if any, looking at the same line and the line directly above it.
func markerNear(comments []commentLine, line int) *commentLine {
	var best *commentLine
	for i := range comments {
		c := &comments[i]
		if !strings.Contains(c.text, "nolint:droppedvalue") {
			continue
		}
		if c.line != line && c.line != line-1 {
			continue
		}
		if best == nil || c.line > best.line {
			best = c
		}
	}
	return best
}

// commentLine is a single comment line with its source position.
type commentLine struct {
	line int
	pos  token.Pos
	text string
}

// collectComments returns every comment line in the file with its source line
// number and position, expanding multi-line /* */ comments line by line.
func collectComments(pass *analysis.Pass, file *ast.File) []commentLine {
	var out []commentLine
	for _, group := range file.Comments {
		for _, c := range group.List {
			base := pass.Fset.Position(c.Pos())
			for i, part := range strings.Split(c.Text, "\n") {
				pos := token.Pos(int(c.Pos()) + i)
				out = append(out, commentLine{line: base.Line + i, pos: pos, text: strings.TrimSpace(part)})
			}
		}
	}
	return out
}

// findAssignments returns every assignment or short-variable declaration in the
// file, including the initializers of if/switch/for statements, which are also
// *ast.AssignStmt nodes.
func findAssignments(file *ast.File) []*ast.AssignStmt {
	var out []*ast.AssignStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			out = append(out, assign)
		}
		return true
	})
	return out
}

// droppedBlanks returns the number of blank identifiers on the left-hand side
// of assign that consume a freshly computed value. Assigning an existing
// variable to a blank (_, x = a, b) computes nothing and only keeps the
// variable alive, so it is not a dropped value.
func droppedBlanks(assign *ast.AssignStmt) int {
	n := 0
	for i, lhs := range assign.Lhs {
		id, ok := lhs.(*ast.Ident)
		if !ok || id.Name != "_" {
			continue
		}
		if computesValue(assign, i) {
			n++
		}
	}
	return n
}

// computesValue reports whether the right-hand side consumed by the i-th
// left-hand side evaluates a new value (a call or operation) rather than merely
// referencing an already-bound variable (ident, selector, index, dereference).
func computesValue(assign *ast.AssignStmt, i int) bool {
	var r ast.Expr
	if len(assign.Rhs) == 1 && len(assign.Lhs) > 1 {
		r = assign.Rhs[0] // a call with multiple results distributes to all LHS
	} else {
		r = assign.Rhs[i]
	}
	switch r.(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr, *ast.StarExpr:
		return false
	default:
		return true
	}
}

// Package main implements droppedvalue, a check that fails on values assigned
// to blank identifiers. A deliberate drop needs a reason:
//
//	_, _ = w.Write(b) //nolint:droppedvalue -- count and error are uninteresting
//
// Range bindings, var _ assertions, generated files, and keep-alive _ = x are exempt.
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Marker justifies a dropped value and must carry a "-- reason".
const Marker = "//nolint:droppedvalue"

// Analyzer reports values assigned to blank identifiers.
var Analyzer = &analysis.Analyzer{
	Name: "droppedvalue",
	Doc:  "report values assigned to blank identifiers",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		if ast.IsGenerated(file) {
			continue
		}
		comments := collectComments(pass, file)
		assigns := findAssignments(file)
		assignLines := make(map[int]bool, len(assigns))
		for _, assign := range assigns {
			start := pass.Fset.Position(assign.Pos()).Line
			end := pass.Fset.Position(assign.End()).Line
			for line := start; line <= end; line++ {
				assignLines[line] = true
			}
		}
		for _, assign := range assigns {
			if n := droppedBlanks(assign); n > 0 {
				reportDrop(pass, comments, assign, assignLines, n)
			}
		}
	}
	return nil, nil
}

// reportDrop flags a drop unless an applicable marker justifies it. A bare
// marker is also reported, so it cannot silently bypass the check.
func reportDrop(pass *analysis.Pass, comments []commentLine, assign *ast.AssignStmt, assignLines map[int]bool, n int) {
	marker := markerApplies(pass, comments, assign, assignLines)
	if marker != nil && parseMarker(marker.text) == justified {
		return
	}
	if marker != nil {
		pass.Report(analysis.Diagnostic{
			Pos:     marker.pos,
			Message: fmt.Sprintf("droppedvalue marker requires a reason: use %q -- <reason>", Marker),
		})
	}
	pass.Report(analysis.Diagnostic{
		Pos:     assign.Pos(),
		Message: dropMessage(n),
	})
}

// dropMessage reports that n blank identifiers dropped a computed value.
func dropMessage(n int) string {
	var blanks string
	if n == 1 {
		blanks = "1 blank identifier"
	} else {
		blanks = fmt.Sprintf("%d blank identifiers", n)
	}
	return fmt.Sprintf("assigned value is dropped (%s); assign it to a named identifier or justify with %s -- <reason>", blanks, Marker)
}

// markerApplies returns the marker covering assign: on the assignment's line(s)
// or as a standalone leading comment on the line above, never a trailing
// comment of a prior statement.
func markerApplies(pass *analysis.Pass, comments []commentLine, assign *ast.AssignStmt, assignLines map[int]bool) *commentLine {
	startLine := pass.Fset.Position(assign.Pos()).Line
	endLine := pass.Fset.Position(assign.End()).Line
	var best *commentLine
	for i := range comments {
		c := &comments[i]
		if parseMarker(c.text) == notAMarker {
			continue
		}
		inRange := c.line >= startLine && c.line <= endLine
		leading := c.line == startLine-1 && !assignLines[c.line]
		if !inRange && !leading {
			continue
		}
		if best == nil || c.line > best.line {
			best = c
		}
	}
	return best
}

// markerKind classifies a comment's relationship to the droppedvalue linter.
type markerKind int

const (
	// notAMarker: not a droppedvalue directive (incl. other linters' nolints).
	notAMarker markerKind = iota
	// justified: a droppedvalue directive with a "-- reason".
	justified
	// missingReason: a droppedvalue directive without a reason.
	missingReason
)

// parseMarker classifies a comment; "droppedvalue" may appear anywhere in the
// //nolint linter-name list.
func parseMarker(text string) markerKind {
	t := strings.TrimSpace(text)
	rest, ok := strings.CutPrefix(t, "//nolint:")
	if !ok {
		return notAMarker
	}
	head := rest
	if i := strings.Index(rest, "--"); i >= 0 {
		head = rest[:i] // linter-name list
	}
	if i := strings.Index(head, "//"); i >= 0 {
		head = head[:i]
	}
	hasDropped := false
	for _, name := range strings.Split(head, ",") {
		if strings.TrimSpace(name) == "droppedvalue" {
			hasDropped = true
			break
		}
	}
	if !hasDropped {
		return notAMarker
	}
	reason := ""
	if i := strings.Index(rest, "--"); i >= 0 {
		reason = rest[i+2:]
	}
	if strings.TrimSpace(reason) == "" {
		return missingReason
	}
	return justified
}

// commentLine is a single line comment with its source position.
type commentLine struct {
	line int
	pos  token.Pos
	text string
}

// collectComments returns the line comments with their source position.
// Block comments are not valid markers.
func collectComments(pass *analysis.Pass, file *ast.File) []commentLine {
	var out []commentLine
	for _, group := range file.Comments {
		for _, c := range group.List {
			if !strings.HasPrefix(c.Text, "//") {
				continue
			}
			out = append(out, commentLine{line: pass.Fset.Position(c.Pos()).Line, pos: c.Pos(), text: strings.TrimSpace(c.Text)})
		}
	}
	return out
}

// findAssignments returns every assignment, including if/switch/for initializers.
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

// droppedBlanks counts blanks that consume a computed value; assigning an
// already-bound variable to a blank (_ = x) is a keep-alive, not a drop.
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

// computesValue reports whether the i-th LHS consumes a computed value rather
// than a plain identifier reference (a keep-alive).
func computesValue(assign *ast.AssignStmt, i int) bool {
	var r ast.Expr
	if len(assign.Rhs) == 1 && len(assign.Lhs) > 1 {
		r = assign.Rhs[0] // a call with multiple results distributes to all LHS
	} else {
		r = assign.Rhs[i]
	}
	switch r.(type) {
	case *ast.Ident:
		return false // plain identifier reference, a keep-alive
	default:
		return true
	}
}

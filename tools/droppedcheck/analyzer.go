// Package main implements droppedvalue, a pure-AST check that fails the build
// on values assigned to blank identifiers.
//
// Every blank in the left-hand side of an assignment or short-variable
// declaration is flagged, including in if/switch/for initializers. Three
// patterns are deliberately exempt because they are structural, not value
// drops: the key (and key/value) bindings of a for-range statement,
// compile-time assertions written as var declarations (var _ T = ...), and
// keep-alive assignments of an already-bound identifier to a blank (_ = x).
// Generated files are skipped entirely.
//
// A deliberate drop can be justified with a reason:
//
//	_, _ = w.Write(b) //nolint:droppedvalue -- Write returns count and error, http.Error reports failure
//
// The marker must carry a reason; a bare //nolint:droppedvalue is itself a
// diagnostic, so dropping the reason fails the build. The marker may be a
// trailing comment on the assignment's line, on any line of a multi-line
// assignment, or a standalone comment on the line directly above (but not a
// trailing comment of a preceding statement), and it may appear anywhere in a
// //nolint linter-name list.
package main

import (
	"fmt"
	"go/ast"
	"go/token"
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

// reportDrop reports an assignment that drops n values. An applicable marker
// suppresses the diagnostic only when it carries a reason; a bare marker is
// additionally reported as requiring one and does not suppress the drop, so it
// cannot be used as a silent bypass.
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

// markerApplies returns the droppedvalue marker that applies to assign, if any.
// A marker on the same line as the assignment or anywhere within a multi-line
// assignment's line range always applies; a marker on the line directly above
// applies only when it is a standalone leading comment, i.e. no assignment
// occupies that marker's line, so a trailing comment of a prior statement can
// never suppress a later drop.
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
	// notAMarker is a comment that is not a droppedvalue directive (including
	// nolint directives for other linters), and is ignored entirely.
	notAMarker markerKind = iota
	// justified is a droppedvalue directive carrying a non-empty "-- reason".
	justified
	// missingReason is a droppedvalue directive without the required reason.
	missingReason
)

// parseMarker classifies a comment as a droppedvalue nolint directive. The
// linter-name list runs from "//nolint:" up to the "-- reason" separator (or
// to a nested "//", e.g. an analysistest "// want" expectation); "droppedvalue"
// may appear anywhere in the comma-separated list.
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

// commentLine is a single comment line with its source position.
type commentLine struct {
	line int
	pos  token.Pos
	text string
}

// collectComments returns every line comment in the file with its source line
// number and position. Block comments are not valid marker carriers and are
// ignored entirely.
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
// left-hand side computes a new value. Any expression other than a bare
// reference to an already-bound identifier computes a value and is a dropped
// value when assigned to a blank; only a plain identifier (a keep-alive like
// _ = x) is exempt.
func computesValue(assign *ast.AssignStmt, i int) bool {
	var r ast.Expr
	if len(assign.Rhs) == 1 && len(assign.Lhs) > 1 {
		r = assign.Rhs[0] // a call with multiple results distributes to all LHS
	} else {
		r = assign.Rhs[i]
	}
	switch r.(type) {
	case *ast.Ident:
		return false // plain identifier reference, a keep-alive, not a computed value
	default:
		return true
	}
}

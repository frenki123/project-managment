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

const Marker = "//nolint:droppedvalue"

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

func dropMessage(n int) string {
	var blanks string
	if n == 1 {
		blanks = "1 blank identifier"
	} else {
		blanks = fmt.Sprintf("%d blank identifiers", n)
	}
	return fmt.Sprintf("assigned value is dropped (%s); assign it to a named identifier or justify with %s -- <reason>", blanks, Marker)
}

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

type markerKind int

const (
	notAMarker markerKind = iota
	justified
	missingReason
)

func parseMarker(text string) markerKind {
	t := strings.TrimSpace(text)
	rest, ok := strings.CutPrefix(t, "//nolint:")
	if !ok {
		return notAMarker
	}
	head := rest
	if i := strings.Index(rest, "--"); i >= 0 {
		head = rest[:i]
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

type commentLine struct {
	line int
	pos  token.Pos
	text string
}

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

func computesValue(assign *ast.AssignStmt, i int) bool {
	var r ast.Expr
	if len(assign.Rhs) == 1 && len(assign.Lhs) > 1 {
		r = assign.Rhs[0] // a call with multiple results distributes to all LHS
	} else {
		r = assign.Rhs[i]
	}
	switch r.(type) {
	case *ast.Ident:
		return false
	default:
		return true
	}
}

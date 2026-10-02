// Command droppedcheck runs the droppedvalue analyzer as a standalone tool.
//
// Usage:
//
//	droppedcheck ./...
//
// A dropped value can be justified with a reason:
//
//	_, _ = w.Write(b) //nolint:droppedvalue -- Write returns count and error, http.Error reports failure
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(Analyzer)
}

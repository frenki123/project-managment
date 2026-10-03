// Command droppedcheck runs the droppedvalue analyzer.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(Analyzer)
}

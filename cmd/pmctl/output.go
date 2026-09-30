package main

import (
	json "encoding/json/v2"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

const maxTableRows = 50

type printer struct {
	json     io.Writer
	table    io.Writer
	useTable bool
}

func (p printer) print(value any, table func(io.Writer) error) error {
	if p.useTable {
		return table(p.table)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	_, err = fmt.Fprintln(p.json, string(data))
	return err
}

func (p printer) deleted(kind string, id int64) error {
	return p.print(map[string]any{"deleted": true, "id": id, "type": kind}, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "deleted %s %d\n", kind, id)
		return err
	})
}

func tableWriter(w io.Writer) *tabwriter.Writer { return tabwriter.NewWriter(w, 0, 4, 2, ' ', 0) }
func tableLimit(total int) (int, int) {
	if total <= maxTableRows {
		return total, 0
	}
	return maxTableRows, total - maxTableRows
}
func tableMore(t *tabwriter.Writer, more int) {
	if more > 0 {
		fmt.Fprintf(t, "... %d more rows; use JSON output for the complete result\n", more)
	}
}
func ellipsis(value string, width int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= width {
		return value
	}
	if width < 4 {
		return value[:width]
	}
	return value[:width-3] + "..."
}

func projectTable(w io.Writer, projects []project.Project) error {
	t := tableWriter(w)
	fmt.Fprintln(t, "ID\tNAME\tPO\tTOTAL\tPLANNED\tSPENT\tPROGRESS")
	limit, more := tableLimit(len(projects))
	for _, p := range projects[:limit] {
		fmt.Fprintf(t, "%d\t%s\t%s\t%.2f\t%.2f\t%.2f\t%.1f%%\n", p.ID, ellipsis(p.Name, 28), ellipsis(p.PurchaseOrderName, 18), p.TotalHours, p.PlannedHours, p.SpentHours, p.ProgressPct)
	}
	tableMore(t, more)
	return t.Flush()
}
func subprojectTable(w io.Writer, values []subproject.Subproject) error {
	t := tableWriter(w)
	fmt.Fprintln(t, "ID\tPROJECT\tNAME\tTOTAL\tPLANNED\tSPENT")
	limit, more := tableLimit(len(values))
	for _, s := range values[:limit] {
		fmt.Fprintf(t, "%d\t%d\t%s\t%.2f\t%.2f\t%.2f\n", s.ID, s.ProjectID, ellipsis(s.Name, 28), s.TotalHours, s.PlannedHours, s.SpentHours)
	}
	tableMore(t, more)
	return t.Flush()
}
func taskTable(w io.Writer, values []task.Task) error {
	t := tableWriter(w)
	fmt.Fprintln(t, "ID\tNAME\tPROJECT\tTOTAL\tSPENT\tPROGRESS\tSTATUS")
	limit, more := tableLimit(len(values))
	for _, v := range values[:limit] {
		project := "-"
		if v.ProjectID != nil {
			project = fmt.Sprint(*v.ProjectID)
		}
		fmt.Fprintf(t, "%d\t%s\t%s\t%.2f\t%.2f\t%.1f%%\t%s\n", v.ID, ellipsis(v.Name, 30), project, v.TotalHours, v.SpentHours, v.Progress, v.Status)
	}
	tableMore(t, more)
	return t.Flush()
}
func weekTable(w io.Writer, v weekly.Cell) error {
	t := tableWriter(w)
	fmt.Fprintln(t, "TASK\tWEEK\tPLANNED\tSPENT\tPROGRESS")
	progress := ""
	if v.Progress != nil {
		progress = fmt.Sprintf("%.1f%%", *v.Progress)
	}
	fmt.Fprintf(t, "%d\t%s\t%.2f\t%.2f\t%s\n", v.TaskID, v.WeekStart, v.PlannedHours, v.SpentHours, progress)
	return t.Flush()
}
func curveTable(w io.Writer, v project.SCurve) error {
	t := tableWriter(w)
	fmt.Fprintf(t, "PROJECT\t%s\n", ellipsis(v.Project.Name, 32))
	fmt.Fprintln(t, "WEEK\tPLANNED\tSPENT\tEARNED")
	limit, more := tableLimit(len(v.Weeks))
	for _, week := range v.Weeks[:limit] {
		fmt.Fprintf(t, "%s\t%.2f\t%.2f\t%.2f\n", week.WeekStart, week.PlannedHours, week.SpentHours, week.EarnedHours)
	}
	tableMore(t, more)
	return t.Flush()
}

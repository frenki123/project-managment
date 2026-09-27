package views

import "cad-development/internal/task"

type TaskFormData struct {
	Action       string
	Title        string
	Task         TaskFormValues
	Summary      TaskPanelData
	DetailPath   string
	Projects     []task.Option
	Subprojects  []task.Option
	CanReassign  bool
	ReassignNote string
	Error        string
	DeleteAction string
}

type TaskFormValues struct {
	Name                string
	Description         string
	ImplementationNotes string
	Department          string
	Developers          string
	Priority            string
	ProjectID           string
	SubprojectID        string
}

type ProjectFormData struct {
	Action       string
	Title        string
	Context      string
	Project      ProjectFormValues
	Error        string
	DeleteAction string
}

type ProjectFormValues struct {
	Name              string
	PurchaseOrderName string
	TotalHours        string
	StartDate         string
	EndDate           string
}

type SubprojectFormData struct {
	Action       string
	Title        string
	Context      string
	Subproject   SubprojectFormValues
	Projects     []task.Option
	Error        string
	DeleteAction string
}

type SubprojectFormValues struct {
	Name       string
	ProjectID  string
	TotalHours string
}

type TaskPanelData struct {
	Task       task.GridRow
	Department string
	Developers string
	Priority   string
	Notes      string
	Desc       string
	EditPath   string
	DeletePath string
}

type ChartPageData struct {
	Series ChartSeries
}

type ChartSeries struct {
	Labels   []string       `json:"labels"`
	Datasets []ChartDataset `json:"datasets"`
}

type ChartDataset struct {
	Label string    `json:"label"`
	Data  []float64 `json:"data"`
}

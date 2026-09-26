package views

type Option struct {
	Value    string
	Label    string
	Selected bool
}

type GridData struct {
	Projects         []Option
	Subprojects      []Option
	FilterProject    string
	FilterSubproject string
	Ideas            bool
	Weeks            []string
	Rows             []TaskRow
	POName           string
	BudgetHours      float64
	PlannedHours     float64
	SpentHours       float64
	ProgressPct      *float64
	Overrun          bool
	LastMonth        string
	LastMonthUnlock  bool
	PastMonths       []Option
	Error            string
}

type TaskRow struct {
	ID          int64
	Name        string
	ProjectName string
	Subproject  string
	TotalHours  float64
	SpentHours  float64
	Progress    float64
	DetailPath  string
	Cells       []WeekCell
}

type WeekCell struct {
	WeekStart string
	Planned   float64
	Spent     float64
	Progress  float64
	SavePath  string
	Locked    bool
}

type TaskFormData struct {
	Action       string
	Title        string
	Task         TaskFormValues
	Projects     []Option
	Subprojects  []Option
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
	Subproject   SubprojectFormValues
	Projects     []Option
	Error        string
	DeleteAction string
}

type SubprojectFormValues struct {
	Name       string
	ProjectID  string
	TotalHours string
}

type TaskPanelData struct {
	Task       TaskRow
	Department string
	Developers string
	Priority   string
	Notes      string
	Desc       string
	EditPath   string
}

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
	Weeks            []WeekHeader
	WeekTotals       []WeekTotal
	Rows             []TaskRow
	LastMonth        string
	LastMonthUnlock  bool
	PastMonths       []Option
	Error            string
	Totals           GridTotalsData
}

type GridTotalsData struct {
	FilterProject    string
	FilterSubproject string
	POName           string
	BudgetHours      float64
	PlannedHours     float64
	SpentHours       float64
	ProgressPct      *float64
	Overrun          bool
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
	Stored    bool
	SavePath  string
	Locked    bool
	Error     string
}

type WeekRowResponseData struct {
	Row        TaskRow
	WeekTotals []WeekTotal
	Totals     GridTotalsData
}

type WeekHeader struct {
	Start      string
	Number     int
	Date       string
	Month      string
	MonthLabel string
}

type WeekTotal struct {
	Planned           float64
	Spent             float64
	CumulativePlanned float64
	CumulativeSpent   float64
	Earned            float64
}

type MonthGroup struct {
	Month string
	Label string
	Count int
}

type TaskFormData struct {
	Action       string
	Title        string
	Task         TaskFormValues
	Projects     []Option
	Subprojects  []Option
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
	DeletePath string
}

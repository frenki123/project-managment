# Project
Build a local-first web app replacing Excel CAD_Development.xlsx.

# Development rules
- One Go executable; embed templates, static assets, and Goose migrations.
- Single user on localhost; SQLite with modernc.org/sqlite; no auth.
- Use net/http, sqlc, templ, HTMX 4, hyperscript, and Pico CSS.
- No frontend build system; use plain CSS plus vendored assets.
- Design for desktop and laptop screens; mobile support and responsive mobile layouts are not required.
- Calculated values stay calculated; prefer SQL calculations over Go calculations.
- No Excel import; provide Excel/XLSX export.
- Do not edit applied migrations after deployment; add a migration when needed.
- Enter the dev environment with `devenv shell` before running anything.
- All commands go through `just` — run `just --list` to see available recipes.
- Keep tests simple and avoid brittle tests that require frequent updates during development. Use Go stdlib.
- Do not use curl or sqlite3 CLI checks as a substitute for Go tests.
- Prefer small, stable Go mock tests for important calculations and HTTP handlers.
- Organize by domain (task, project, subproject, etc.), not by technical layer.
- Do not commit. Run `just` checks and report results; I will confirm when a commit should be made.

# Original Excel
This Excel separates planning, execution, and progress into three linked sheets (`Plan`, `Actual`, `Progress`) so that estimated hours, real hours spent, and task completion can
be tracked independently instead of assuming "hours used = progress made." Each task's `Complexity` (h), set in `Plan`, is turned into a weight factor in `Progress` 
(task complexity / total complexity of all tasks) and combined with weekly progress % to produce a cumulative earned-value figure in hours — putting progress on the same scale as
planned and actual hours. These three hour-based curves (Planned, Actual, Earned) are plotted together on the `S curve` sheet, which is the key output of the whole file: 
a real picture of whether the project is on track, even when time spent doesn't match work actually done.

The key point of this Excel is that we can have estimated hours of 100, developer hours of 100, and progress can still be 50%. This just means project progress isn't going as 
planned, and that's exactly what the S-curve is meant to visualize — unlike other project management tools that would automatically mark a task as finished once all the hours are 
used. Planned hours, used hours, and progress must stay conceptually separate, and `Complexity` is what lets progress (%) be converted into the same hours metric as planned/actual,
so all three can be plotted together.

## Excel Data Model
This is a list of the data used in the Excel. It's not divided by sheet, as sheet usage was just an Excel limitation in the design, not something we want to copy.
In general we don't care which sheet something is on, only what the data represents. The basic data are a list of tasks with static data and weekly hours and progress tracking.

### User Inputs:
- task ID
- task name
- task description
- task implementation notes
- relevant department
- developer/developers
- priority
- weekly estimation of the planned hours for the task
- weekly actual hours worked per task
- weekly progress estimation per task. Progress can't go down, so if progress is 50% in W31 and we didn't make any progress in W32, progress in W32 is 50%. So progress is cumulative

### Calculated results
- Complexity (Total Hours per Task)
- Total planned hours per week
- Total planned hours cumulative - so we can track how much total hours is planned for the project
- Total hours spent/actual per Task
- Total hours spent/actual per week
- Total hours spent/actual per week cumulative
- Task progress per week - max value entered for progress
- Task weight factor - ratio between task and total complexity `TotalTaskHours / SumOfTotalTaskHours`. `TotalTaskHours` is a sum of weekly planned hours. We don't enter specificly total hours for task.
- Total weekly progress - sumproduct between task weight factor and task progress
- Total weekly progress (h) = Total weekly progress (%) × Complexity total
- S-curve graph

## Excel Missing data
- Project concept - in Excel we simplified the problem with project per year, but this is wrong. In the app we need to be able to define tasks and projects. Important is that we can have
tasks that don't have a project. These tasks represent ideas, but currently are not assigned to any project.
- Subproject concept - every project can also have a subproject. Subproject is basically just a simple extra label for part of the tasks in the project, so we can have one extra filter
on the tasks per project. 
Every project contains all the tasks in the subproject!

*In general, subproject and project are really similar*

### Project data model
#### Input data
- project name
- purchase order name
- project total hours
- tasks in the project
- subprojects in the project
- start date
- end date
#### Calculated data
- actual used hours
- progress
- planned used hours

### Subproject data model
- subproject name
- total hours
- tasks in the subproject
#### Calculated data
- actual used hours
- planned used hours

# Web App
In general the web app will have two entry points that should always be almost the same - user UI and machine JSON data. UI will be defined with `htmx` and `templ`, JSON data will be
used with the CLI and SDKs in the future so LLM harness can also use this application.

## UI/UX
We will try to make "Excel-like" inputs as much as possible. Fallback to form creation and edit only for data that we don't need often.
### Form data
#### New Task/Edit Task:
- task name
- task description
- task implementation notes
- relevant department
- developer/developers
- priority
- project
- optional subproject

#### New Project/Edit Project:
- name
- purchase order name
- total hours
- start date
- end date

#### New Subproject/Edit Subproject:
- name
- project
- total hours
- validate that sum of subproject hours per project is not more than total project hours

### Weight factor / Earned value calculation
- Task weight factor = task_hours / project_total_hours
- Idea tasks (no project assigned) have no weight factor and are excluded from any project-level S-curve/earned-value calculation
- Project progress (%) = sum(task weight factor × task progress) for all tasks in the project
- Project progress (h) = Project progress (%) × project_total_hours

### Table data:
Filtered by project and/or subproject. Rows are tasks, columns are weeks calculated from project start and end date.
#### Non-editable data in table (we need to use form)
- task name
- project name
- subproject name (if NULL then empty string)
#### Calculated data
- total task hours
- total task spent hours
- total task progress
#### Editable data
- weekly planned hours
- weekly spent hours
- current task progress
#### Data validation
- progress can't be less than the week before
- if progress is not entered for the week it stays like the last week
- if progress in week before is edited and progress in the week after are less update all progress for next week.
CHECK FOR LOCKED MONTHS! If months are locked don't allow the edit. Edit of locked months needs to be specificly enabled with the combobox!
- last month data is "locked" and needs to be unlocked with specific button. Locked data is for planned, spent and progress per tasks. Last month is not "before 30 days", it is acutall month before. So at 2nd of May we can't edit data in the April
- normal validation like no negative hours, no negative progress etc.
#### Row Click Action
- opens the task detail with all entered and calculated total data. Has `EDIT` button so we can edit tasks. Open it as a modal detail or side view (still open to decision).
#### Table header
- project filter or "ideas" which are not assigned. For ideas don't show weekly columns. Ideas can't be planned
- subproject filter - default OFF
- PO name of the project
- total project/subproject hours
- planned project/subproject hours
- spent project/subproject hours
- project progress in `%`. Subproject doesn't have progress!
#### Table look - row
----------------------------------------------------------------------
| TASKS DATA (fixed)                                             | W1             | W2 | W3 |... week are scrolable
----------------------------------------------------------------------
|      |         |            |          |          |            | weekly planned 
| Task | Project | Subproject | total hr | spent hr | progress % | weekly used
|      |         |            |          |          |            | progress
----------------------------------------------------------------------
                                               fixed till here ->|

### Graph
S curve designed per project

## Extra notes
- Weekly task hours are not used or validated in the total Project or Subproject values. We will just show that we planned more tasks than Project or Subproject allows.
- "ideas" task are moved to the project or subproject with edit button. You click on the task and then edit, then assign project and/or subproject

## CLI/REST JSON API
Built so LLM agent can modify/read all values in the application. It needs to be built in parallel with the main UI
Use routes like `/api/v1`, no need for specific HTML headers.

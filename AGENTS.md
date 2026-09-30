# Project
Build a local-first web app for project hour planning, time tracking, and progress tracking.

# Development rules
- When a minimal quick fix and a better long-term design conflict, always choose the better long-term design.
- One Go executable; embed templates, static assets, and Goose migrations.
- Single user on localhost; SQLite with modernc.org/sqlite; no auth.
- Use net/http, sqlc, templ, HTMX 4, hyperscript, Pico CSS, and Chart.js.
- No frontend build system; use plain CSS plus vendored assets.
- Vendored assets are pinned and committed: `static/js/htmx.min.js` (HTMX 4.0.0), `static/js/chart.umd.min.js` (Chart.js 4.5.1 UMD, exposes the global `Chart`), `static/css/pico.min.css` (Pico 2.1.1). Source maps are not vendored.
- Design for desktop and laptop screens; mobile support and responsive mobile layouts are not required.
- Calculated values stay calculated; prefer SQL calculations over Go calculations.
- No spreadsheet import; provide XLSX export.
- While in alpha, `sql/migrations/00001_init.sql` is edited in place; do not add migration files. Apply the change with `just db-reset`. Once a non-prerelease version ships, never edit an applied migration again — add a new one from then on.
- Use Go `1.27.1` through `devenv`; run project commands via `just` — run `just --list` to see available recipes.
- Keep tests simple and avoid brittle tests that require frequent updates during development. Use Go stdlib.
- Do not use curl or sqlite3 CLI checks as a substitute for Go tests.
- Prefer small, stable Go mock tests for important calculations and HTTP handlers.
- Organize by domain (task, project, subproject, etc.), not by technical layer. only exception is `internal\app` that will be used as small web&db framework.
- Never commit directly to `main`; always open a PR. Run `just check` before opening it.
- Keep the shared UI, REST, and CLI contract documented in the README API Contract section when behavior changes.

# Go 1.27
- Prefer `errors.AsType`, `new(expr)`, `t.Context()`, and `slices` helpers when they improve clarity.
- Use `encoding/json/v2` at HTTP boundaries; test its stricter behavior and preserve API contracts.
- Do not add generic abstractions unless they clearly simplify the code; generic methods cannot implement interface methods.

# Releases and compatibility
- The project is pre-release; no tagged release has shipped yet. Breaking changes are expected and allowed: schema, `/api/v1` contracts, and UI can change without deprecation or back-compat shims. `just db-reset` is the expected way to recover a local database.
- Tag a release with `v<semver>`, suffixed `alpha` while unstable (`v0.0.2-alpha`). Pushing the tag builds, tests, and publishes a GitHub Release. A tag with a `-` suffix is published as a pre-release.

# Release cycle
- Merging to `main` never publishes a release. Only pushing a `v*` tag does, and that path runs `just check` before it builds.
- Merging to `main` runs CI only: `just check` plus a cross-compile canary. No binary is published from that path.
- The rolling `dev` pre-release is a separate workflow, rebuilt nightly at 01:20 Europe/Zagreb for Linux and Windows. It skips tests, because `main` is already tested by CI. It is a convenience build, never a substitute for a tagged release.
- `dev` is a single fixed tag. When a new commit exists, it is force-moved to that commit so GitHub's generated source archives match the binaries; assets are replaced in place and old assets are pruned. A night with no new commit on `main` is skipped. Workflow dispatch publishes the dispatched commit intentionally.
- All release workflows build through `just dist <version>`; do not duplicate the cross-compile block in workflow files.
- `releases/latest` skips pre-releases, so every build is a pre-release and `releases/latest/download/<file>` returns 404. Always name the tag in download URLs.

# Domain model
Planning, execution, and progress are three separate dimensions. Estimated hours, real hours spent, and task completion are tracked independently instead of assuming
"hours used = progress made". A task can have 100 h estimated, 100 h spent, and still be 50% done, which simply means the project is not going as planned.

Each task's complexity (h) is turned into a weight factor and combined with weekly progress % to produce a cumulative earned-value figure in hours, putting progress on the same
scale as planned and spent hours. The three hour-based curves (Planned, Spent, Earned) are plotted together on the S-curve, which is the key output of the app: a real picture of
whether the project is on track, even when time spent doesn't match work actually done. Planned hours, spent hours, and progress must stay conceptually separate, and complexity is
what lets progress (%) be converted into the same hours metric as planned/spent, so all three can be plotted together.

## Task
The basic data is a list of tasks with static data plus weekly hours and progress tracking.
### Input data
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
### Calculated data
- Complexity (total planned hours per task) - a sum of weekly planned hours. We don't enter total hours for a task separately.
- Total planned hours per week and cumulative
- Total hours spent/actual per task, per week, and cumulative
- Task progress per week - max value entered for progress

## Project
Projects are first-class, not implied by a period. A task does not need a project; such tasks are ideas that are not assigned to any project yet.
### Input data
- project name
- purchase order name
- project total hours
- tasks in the project
- subprojects in the project
- start date
- end date
### Calculated data
- planned hours
- actual used hours
- progress

## Subproject
A subproject is a simple extra label for part of the tasks in a project, giving one extra filter on the tasks per project. Every project contains all the tasks in the
subproject. In general, subproject and project are really similar.
### Input data
- subproject name
- total hours
- tasks in the subproject
### Calculated data
- planned hours
- actual used hours
- subprojects do not have progress

# Web App
In general the web app will have two entry points that should always be almost the same - user UI and machine JSON data. UI will be defined with `htmx` and `templ`, JSON data will be
used with the CLI and SDKs in the future so LLM harness can also use this application.

## UI/UX
We will try to make "spreadsheet-like" inputs as much as possible. Fallback to form creation and edit only for data that we don't need often.
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
CHECK FOR LOCKED MONTHS! By default, all weeks whose Monday falls in a past calendar month are locked for planned hours, spent hours, and progress. On May 2, April and earlier weeks are locked (not merely weeks older than 30 days).
- Historical weeks stay locked unless the individual request explicitly unlocks them. The UI stores a short-lived browser cookie; API clients set optional `unlock: true` in the weekly update JSON. There are no persisted or per-month unlocks.
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

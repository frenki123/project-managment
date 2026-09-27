# CAD Development

Local-first web app for project hour planning, time tracking, and progress tracking. One Go
binary, one SQLite file, no auth, no frontend build system.

Planned hours, spent hours, and progress are tracked independently, so a task can have 100 h
estimated, 100 h spent, and still be only 50% done. Task complexity turns progress into the
same hours metric, which makes it possible to plot **Planned**, **Spent**, and **Earned** on
one S-curve per project.

## Features

- **Workboard** (`/`): the main grid. Rows are tasks, columns are weeks derived from the project
  start/end date. Editable cells for weekly planned hours, weekly spent hours, and current
  progress. Task, project, and subproject names come from forms, not the grid.
- **Projects**: name, purchase order name, total hours, start/end date. Header strip shows
  budget, planned, spent, and progress %.
- **Subprojects**: an extra label inside a project, with its own total hours. Sum of
  subproject hours cannot exceed project hours.
- **Ideas**: tasks without a project. They have no weekly columns and cannot be planned until
  a project is assigned.
- **Progress rules**: progress is cumulative, cannot decrease, an empty week keeps the last
  value, and lowering a week raises all later weeks to match.
- **Month locks**: by default every week whose Monday falls in a past calendar month is locked.
  A single global "Unlock history" toggle opens all history; "Lock history" restores the
  automatic rule.
- **S-curve** (`/chart`): per-project Chart.js chart of cumulative planned, spent, and
  earned hours. Earned value is computed in SQL from the recursive week series.
- **Task detail**: click a row to open a side panel with all entered and calculated totals,
  plus an edit button.
- **Filters**: project (or "Ideas"), optional subproject filter; last project remembered in a
  cookie.
- **JSON API** under `/api/v1` mirroring the UI, for scripts, CLIs, and LLM agents.

## Stack

Go 1.27.1, `net/http` (stdlib `ServeMux`, no router dependency), SQLite via
`modernc.org/sqlite`, `sqlc` for typed queries, Goose for embedded migrations, `templ` for
HTML. Frontend is vendored and pinned, no bundler: HTMX 4.0.0, Pico CSS 2.1.1, Chart.js 4.5.1
UMD. Desktop and laptop screens are the target; mobile layouts are out of scope.

## Getting started

Requires [devenv](https://devenv.sh/) and direnv; it provides Go 1.27, `just`, `templ`,
`sqlc`, `goose`, and `air`.

```sh
direnv allow          # or: devenv shell
just dev-reset        # reset DB, seed example data, start Air
```

Then open http://127.0.0.1:8080. The server also runs standalone with `just run`, and applies
migrations on boot. `PORT` overrides the default `8080`; the database is `data/app.db`.

Run `just --list` to see every recipe. `just check` runs tests, vet, and a build.

## Layout

Code is organized by domain, not by layer: `internal/task`, `internal/project`,
`internal/subproject`, `internal/weekly`, `internal/monthlock`. `internal/app` is the small web
and database framework; `internal/handlers` holds HTTP wiring; `internal/views` holds `templ`
components. SQL lives in `sql/queries` and `sql/migrations`, generated code in `internal/db`.

## License

MIT, see [LICENSE](LICENSE).
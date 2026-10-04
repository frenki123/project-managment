# Project Managment web app

Local-first web app for project hour planning, time tracking, and progress tracking. A Go server
and REST-backed CLI share one SQLite file, with no auth or frontend build system.

Planned hours, spent hours, and progress are tracked independently, so a task can have 100 h
estimated, 100 h spent, and still be only 50% done. Task complexity turns progress into the
same hours metric, which makes it possible to plot **Planned**, **Spent**, and **Earned** on
one S-curve per project.

> **Pre-release: expect breaking changes.** There is no tagged release yet; the nightly `dev` build
> tracks `main`. The data model, `/api/v1` responses, and UI can change or break at any time, with
> no migration path for existing data. Back up `data/app.db`; to start over after an upgrade, stop
> the server, delete it, and start again — the schema is rebuilt on boot.

## Features

- **Workboard** (`/`): the main grid. Rows are tasks, columns are weeks derived from the project
  start/end date. Editable cells for weekly planned hours, spent hours, and progress; task, project,
  and subproject names come from forms, not the grid.
- **All-task summary**: the default workboard view lists every task without weekly columns;
  selecting a project opens that project's weekly planning grid, while Ideas shows unassigned tasks.
- **Projects**: name, purchase order name, total hours, start/end date; header strip shows
  budget, planned, spent, and progress %.
- **Subprojects**: an extra label inside a project with its own total hours; their sum cannot exceed project hours.
- **Ideas**: tasks without a project; they have no weekly columns and cannot be planned until a project is assigned.
- **Progress rules**: effective progress is a running maximum and never decreases. An empty week
  carries the last value; storing below it stores nothing; raising a week clears later weeks below it.
- **Historical editing**: weeks whose Monday falls in a past calendar month are locked by default;
  the UI can allow editing in that browser for two hours, and API clients set `unlock: true` on that one update; no access state is stored in SQLite.
- **S-curve** (`/chart`): per-project Chart.js chart of cumulative planned, spent, and earned
  hours, computed in SQL from the recursive week series.
- **Task detail**: click a row to open a side panel with all totals plus an edit button.
- **Filters**: project (or "Ideas") and optional subproject filter; last project remembered in a cookie.
- **JSON API** under `/api/v1` mirroring the UI, for scripts, CLIs, and LLM agents.
- **`pmctl` CLI**: operate on the REST API without direct database access. JSON is the default;
  add `--table` for compact output. `pmctl tasks list` returns all tasks by default; project
  filters are name- or ID-based, names win if both given. POST creates use empty values for
  omitted fields; PUT is presence-driven — omitted fields preserved, `null` clears nullable
  fields, zero sets numeric fields. Weekly updates set only the flags passed (`--planned-hours`,
  `--spent-hours`, `--progress <hours>`, `--progress null` clears stored progress); historical
  edits send `--unlock` on that one request. Project/subproject filters beat `--ideas`; use
  `--field null` to clear text or nullable assignments, `--ideas` clears both task assignments;
  a reassignment with weekly data returns `409` only when that field was explicitly changed.

## API Contract

Task lists accept `project_id`, `subproject_id`, and `ideas=true`. Ideas and subproject filters are
mutually exclusive; requesting both returns `400`. Missing filters select the default view.
Unknown filters return `404`; a subproject from another selected project returns `400`.

All API errors use the same envelope and preserve the true HTTP status. Errors raised by SQL
conflict or scope checks also carry a stable machine `reason` code for automation:

```json
{"error":"ideas cannot be planned","reason":"idea-task-not-assignable"}
```

The reason codes are `idea-task-not-assignable`, `week-outside-project-bounds`, `task-has-weekly-data`
(week writes); `subproject-not-found`, `project-not-found`, `subproject-project-mismatch` (assignments);
and `project-name-taken`, `project-hours-below-subprojects`, `project-dates-exclude-weekly-data`,
`subproject-hours-exceed-project` (project/subproject). Validation errors without a SQL-backed reason omit the key.

API requests always receive JSON, even when HTMX headers are present; browser and HTMX requests
get HTML error views or fragments with the same status. Weekly updates are presence-driven: absent
fields unchanged, `"progress": null` clears the stored value; any value at or below the carried
one is not stored (the week keeps NULL and carries the value forward); only a value above it
is stored. Effective progress is a running maximum, so stored progress is strictly increasing.
Task detail responses return `weeks` from the canonical project-bounded series: every Monday from
the project's start-week through its end-week, including zero planned/spent weeks. `progress` is the
effective carried value; `stored_progress` is `null` when that week has no stored value. Idea tasks
have no weeks. The workboard, REST API, and `pmctl` CLI consume this same canonical series; future
monthly review and XLSX export must reuse it rather than calculate a consumer-specific series.

Project names are case-insensitively unique; duplicate creates or updates return `409` with
`project name already exists`. Deleting a referenced project, subproject, or task, or moving a
subproject that still has tasks, returns `409` with `record is still used by other data`;
reassigning a task with weekly data returns `409` with `cannot reassign task with weekly data`.
Assigning to a missing project or subproject returns `404`; a subproject from another project returns `400` with `subproject does not belong to project`.

## Stack

Go 1.27.1, `net/http` (stdlib `ServeMux`, no router dependency), SQLite via
`modernc.org/sqlite`, `sqlc` for typed queries, Goose for embedded migrations, `templ` for
HTML. Frontend is vendored and pinned, no bundler: HTMX 4.0.0, Pico CSS 2.1.1, Chart.js 4.5.1
UMD. Desktop and laptop screens are the target; mobile layouts are out of scope.

## Getting started

To run a downloaded binary, see [Releases](#releases) — it needs no toolchain. To build from
source, the rest of this section applies.

Requires [devenv](https://devenv.sh/) and direnv; it provides Go 1.27, `just`, `templ`,
`sqlc`, `goose`, and `air`.

```sh
direnv allow          # or: devenv shell
just dev-reset        # reset DB, seed example data, start Air
```

Then open http://127.0.0.1:8080. The server also runs standalone with `just run`, and applies
migrations on boot. `PORT` overrides the default `8080`; the database is `data/app.db`.

Run `just --list` to see every recipe. `just check` runs tests, a build, and lint (golangci-lint).

## Releases

There are two release channels. Server and `pmctl` binaries are published by both channels.

**Nightly `dev` build.** Every night at 01:20 Europe/Zagreb, the latest commit on `main` is built
and published as the rolling [`dev`](https://github.com/frenki123/project-managment/releases/tag/dev)
pre-release. The `dev` tag and its generated source archive track the commit used to build the
assets. One server binary and one `pmctl` binary per platform, plus checksums, are replaced in
place, so there is never a list of stale builds to choose from:

```sh
gh release download dev -R frenki123/project-managment -p server-linux-amd64
gh release download dev -R frenki123/project-managment -p server-windows-amd64.exe
gh release download dev -R frenki123/project-managment -p pmctl-linux-amd64
gh release download dev -R frenki123/project-managment -p pmctl-windows-amd64.exe
gh release download dev -R frenki123/project-managment -p SHA256SUMS
sha256sum -c SHA256SUMS
```

**Tagged releases.** Tag `v<semver>` and the Release workflow tests that exact commit before
building and publishing the server and `pmctl` binaries plus checksums. A tag with a `-` suffix,
such as `v0.0.2-alpha`, is published as a pre-release. None has shipped yet, so `dev` is the only
downloadable build today.

Download URLs must name the tag. GitHub's `releases/latest` skips pre-releases, and every build
here is a pre-release, so `releases/latest/download/<file>` returns 404.

```
https://github.com/frenki123/project-managment/releases/download/dev/server-linux-amd64
```

The binary is self-contained: no runtime, no toolchain, no dependencies. It creates `data/app.db`
in the current directory on first start and applies the schema itself. `DEVENV_ROOT` overrides
the directory holding `data/`, which keeps the database outside the folder you unpacked into.

```sh
./server-linux-amd64                # http://127.0.0.1:8080
PORT=9000 ./server-linux-amd64      # or pick another port
./server-linux-amd64 --version      # prints the version
```

The CLI targets the local server by default and can use `CAD_API_URL` or `--url` for another API:

```sh
pmctl projects list --table
pmctl tasks list --project "Project Alpha"
pmctl tasks create --name "Implement API" --project-id 5 --priority high
pmctl tasks update 12 --priority medium --project-id 5 --subproject-id 1
pmctl update-task-week 12 2026-09-21 --planned-hours 8 --unlock
```

## Layout

Code is organized by domain, not by layer: `internal/task`, `internal/project`, `internal/subproject`,
`internal/weekly`, `internal/historyaccess`. `internal/web` is the small web framework (HTTP helpers,
errors, CRUD, rendering, recover, static files, validation); `internal/db` holds database open,
migration wiring, and generated sqlc code, with `internal/db/testkit` as the test helper;
`internal/nullable` is the optional wire primitive for JSON fields. `internal/handlers` holds HTTP
wiring; `internal/views` holds `templ` components. SQL lives in `sql/queries` and `sql/migrations`.

## License

MIT, see [LICENSE](LICENSE).

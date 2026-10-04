-- +goose Up
CREATE TABLE projects (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE,
    purchase_order_name TEXT NOT NULL DEFAULT '',
    total_hours REAL NOT NULL CHECK (total_hours >= 0),
    start_date TEXT NOT NULL,
    end_date TEXT NOT NULL,
    CHECK (end_date >= start_date)
);

CREATE TABLE subprojects (
    id INTEGER PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects (id),
    name TEXT NOT NULL,
    total_hours REAL NOT NULL CHECK (total_hours >= 0),
    UNIQUE (project_id, id)
);

CREATE TABLE tasks (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    implementation_notes TEXT NOT NULL DEFAULT '',
    department TEXT NOT NULL DEFAULT '',
    developers TEXT NOT NULL DEFAULT '',
    priority TEXT NOT NULL DEFAULT '',
    project_id INTEGER REFERENCES projects (id),
    subproject_id INTEGER REFERENCES subprojects (id),
    CHECK (subproject_id IS NULL OR project_id IS NOT NULL),
    FOREIGN KEY (project_id, subproject_id)
        REFERENCES subprojects (project_id, id)
);

CREATE TABLE task_weeks (
    task_id INTEGER NOT NULL REFERENCES tasks (id),
    week_start TEXT NOT NULL,
    planned_hours REAL NOT NULL DEFAULT 0 CHECK (planned_hours >= 0),
    spent_hours REAL NOT NULL DEFAULT 0 CHECK (spent_hours >= 0),
    progress REAL CHECK (progress IS NULL OR (progress >= 0 AND progress <= 100)),
    PRIMARY KEY (task_id, week_start)
);

CREATE TABLE people (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE,
    weekly_capacity REAL NOT NULL DEFAULT 40 CHECK (weekly_capacity >= 0)
);

CREATE TABLE person_week_overrides (
    person_id INTEGER NOT NULL REFERENCES people (id),
    week_start TEXT NOT NULL,
    capacity REAL NOT NULL CHECK (capacity >= 0),
    PRIMARY KEY (person_id, week_start)
);

CREATE TABLE task_developers (
    task_id INTEGER NOT NULL REFERENCES tasks (id),
    person_id INTEGER NOT NULL REFERENCES people (id),
    PRIMARY KEY (task_id, person_id)
);
CREATE INDEX idx_task_developers_person ON task_developers (person_id);

ALTER TABLE tasks DROP COLUMN developers;

CREATE INDEX idx_subprojects_project_name
    ON subprojects (project_id, name COLLATE NOCASE, id);
CREATE INDEX idx_tasks_project_name
    ON tasks (project_id, name COLLATE NOCASE, id);
CREATE INDEX idx_tasks_subproject_name
    ON tasks (subproject_id, name COLLATE NOCASE, id);
CREATE INDEX idx_task_weeks_week_start ON task_weeks (week_start);

CREATE VIEW v_task_totals AS
SELECT
    t.id AS task_id,
    t.project_id,
    t.subproject_id,
    CAST(COALESCE(SUM(tw.planned_hours), 0) AS REAL) AS planned_hours,
    CAST(COALESCE(SUM(tw.spent_hours), 0) AS REAL) AS spent_hours,
    CAST(COALESCE(MAX(tw.progress), 0) AS REAL) AS progress
FROM tasks t
LEFT JOIN task_weeks tw ON tw.task_id = t.id
GROUP BY t.id;

CREATE VIEW v_project_totals AS
SELECT
    p.id,
    p.name,
    p.purchase_order_name,
    p.total_hours,
    p.start_date,
    p.end_date,
    CAST(COALESCE(SUM(tt.planned_hours), 0) AS REAL) AS planned_hours,
    CAST(COALESCE(SUM(tt.spent_hours), 0) AS REAL) AS spent_hours,
    CAST(CASE WHEN p.total_hours > 0
        THEN COALESCE(SUM(tt.planned_hours * tt.progress / p.total_hours), 0)
        ELSE 0 END AS REAL) AS progress,
    CAST(CAST(CASE WHEN p.total_hours > 0
        THEN COALESCE(SUM(tt.planned_hours * tt.progress / p.total_hours), 0)
        ELSE 0 END AS REAL) * p.total_hours / 100.0 AS REAL) AS earned_hours
FROM projects p
LEFT JOIN v_task_totals tt ON tt.project_id = p.id
GROUP BY p.id;

CREATE VIEW v_project_bounds AS
SELECT
    p.*,
    CAST(date(p.start_date, '-' || ((strftime('%w', p.start_date) + 6) % 7) || ' days') AS TEXT) AS first_week,
    CAST(date(p.end_date, '-' || ((strftime('%w', p.end_date) + 6) % 7) || ' days') AS TEXT) AS last_week
FROM projects p;

CREATE VIEW v_task_week_series AS
WITH RECURSIVE weeks AS (
    SELECT t.id AS task_id, b.first_week AS week_start, b.last_week
    FROM tasks t
    JOIN v_project_bounds b ON b.id = t.project_id
    UNION ALL
    SELECT task_id, date(week_start, '+7 days'), last_week
    FROM weeks
    WHERE week_start < last_week
)
SELECT
    w.task_id,
    w.week_start,
    CAST(COALESCE(tw.planned_hours, 0) AS REAL) AS planned_hours,
    CAST(COALESCE(tw.spent_hours, 0) AS REAL) AS spent_hours,
    tw.progress AS stored_progress,
    CAST(COALESCE(MAX(tw.progress) OVER (
        PARTITION BY w.task_id
        ORDER BY w.week_start
        ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
    ), 0) AS REAL) AS effective_progress
FROM weeks w
LEFT JOIN task_weeks tw
    ON tw.task_id = w.task_id
   AND tw.week_start = w.week_start;

-- +goose Down
DROP VIEW IF EXISTS v_task_week_series;
DROP VIEW IF EXISTS v_project_bounds;
DROP VIEW IF EXISTS v_project_totals;
DROP VIEW IF EXISTS v_task_totals;
DROP TABLE IF EXISTS task_developers;
DROP TABLE person_week_overrides;
DROP TABLE people;
DROP TABLE task_weeks;
DROP TABLE tasks;
DROP TABLE subprojects;
DROP TABLE projects;

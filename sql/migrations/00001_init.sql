-- +goose Up
CREATE TABLE projects (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    purchase_order_name TEXT NOT NULL DEFAULT '',
    total_hours REAL NOT NULL CHECK (total_hours >= 0),
    start_date TEXT NOT NULL,
    end_date TEXT NOT NULL,
    CHECK (end_date >= start_date)
);

CREATE TABLE subprojects (
    id INTEGER PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
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
    project_id INTEGER REFERENCES projects (id) ON DELETE SET NULL,
    subproject_id INTEGER REFERENCES subprojects (id) ON DELETE SET NULL,
    CHECK (subproject_id IS NULL OR project_id IS NOT NULL),
    FOREIGN KEY (project_id, subproject_id)
        REFERENCES subprojects (project_id, id)
);

CREATE TABLE task_weeks (
    task_id INTEGER NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    week_start TEXT NOT NULL,
    planned_hours REAL NOT NULL DEFAULT 0 CHECK (planned_hours >= 0),
    spent_hours REAL NOT NULL DEFAULT 0 CHECK (spent_hours >= 0),
    progress REAL CHECK (progress IS NULL OR (progress >= 0 AND progress <= 100)),
    PRIMARY KEY (task_id, week_start)
);

CREATE TABLE month_locks (
    year_month TEXT PRIMARY KEY NOT NULL,
    unlocked INTEGER NOT NULL DEFAULT 0 CHECK (unlocked IN (0, 1))
);

CREATE INDEX idx_tasks_project_id ON tasks (project_id);
CREATE INDEX idx_tasks_subproject_id ON tasks (subproject_id);

-- +goose Down
DROP TABLE task_weeks;
DROP TABLE month_locks;
DROP TABLE tasks;
DROP TABLE subprojects;
DROP TABLE projects;

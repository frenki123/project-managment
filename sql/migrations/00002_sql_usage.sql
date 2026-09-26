-- +goose Up
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
    COALESCE(SUM(tw.planned_hours), 0) AS planned_hours,
    COALESCE(SUM(tw.spent_hours), 0) AS spent_hours,
    COALESCE(MAX(tw.progress), 0) AS progress
FROM tasks t
LEFT JOIN task_weeks tw ON tw.task_id = t.id
GROUP BY t.id;

-- +goose Down
DROP VIEW v_task_totals;
DROP INDEX idx_task_weeks_week_start;
DROP INDEX idx_tasks_subproject_name;
DROP INDEX idx_tasks_project_name;
DROP INDEX idx_subprojects_project_name;

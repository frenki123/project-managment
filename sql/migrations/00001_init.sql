-- +goose Up
CREATE TABLE meta (
    key TEXT PRIMARY KEY NOT NULL,
    value TEXT NOT NULL
);

INSERT INTO meta (key, value) VALUES ('app', 'cad-development');

-- +goose Down
DROP TABLE meta;

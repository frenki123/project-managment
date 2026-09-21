{ pkgs, ... }:

{
  env.DATABASE_URL = "./data/app.db";
  env.GOOSE_DRIVER = "sqlite3";
  env.GOOSE_DBSTRING = "./data/app.db";
  env.GOOSE_MIGRATION_DIR = "./sql/migrations";
  env.PORT = "8080";

  packages = [
    pkgs.git
    pkgs.air
    pkgs.sqlc
    pkgs.goose
    pkgs.templ
    pkgs.sqlite
  ];

  languages.go.enable = true;

  processes.dev.exec = "air -c .air.toml";

  enterShell = ''
    mkdir -p data tmp
  '';

  enterTest = ''
    go version
    air -v
    sqlc version
    goose --version
    templ version
    sqlite3 --version
  '';
}

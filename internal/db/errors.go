package db

import (
	"errors"
	"strings"

	sqlite3 "modernc.org/sqlite/lib"
)

type sqliteError interface {
	error
	Code() int
}

func UniqueViolation(err error, column string) bool {
	sqliteErr, ok := errors.AsType[sqliteError](err)
	if !ok || sqliteErr.Code() != sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return false
	}
	const prefix = "UNIQUE constraint failed:"
	message := sqliteErr.Error()
	_, details, found := strings.Cut(message, prefix)
	if !found {
		return false
	}
	details = strings.TrimSpace(details)
	if codeIndex := strings.Index(details, " ("); codeIndex >= 0 {
		details = details[:codeIndex]
	}
	for failedColumn := range strings.SplitSeq(details, ",") {
		if strings.TrimSpace(failedColumn) == column {
			return true
		}
	}
	return false
}

func ForeignKeyViolation(err error) bool {
	sqliteErr, ok := errors.AsType[sqliteError](err)
	return ok && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY
}

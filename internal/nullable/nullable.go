package nullable

import "database/sql"

func Int64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

func Int64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	return new(value.Int64)
}

func Float64(value *float64) sql.NullFloat64 {
	if value == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *value, Valid: true}
}

func Float64Pointer(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return new(value.Float64)
}

func String(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func StringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return new(value.String)
}

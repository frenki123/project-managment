package weekly

import "cad-development/internal/db"

func StoredProgress(w db.TaskWeek) *float64 {
	if !w.Progress.Valid {
		return nil
	}
	v := w.Progress.Float64
	return &v
}

func EffectiveFromPrev(prev float64, stored *float64) float64 {
	if stored != nil {
		return *stored
	}
	return prev
}

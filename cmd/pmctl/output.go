package main

import (
	json "encoding/json/v2"
	"fmt"
	"io"
)

type result struct {
	Deleted bool   `json:"deleted,omitzero"`
	ID      int64  `json:"id,omitzero"`
	Type    string `json:"type,omitzero"`
	Error   string `json:"error,omitzero"`
	Status  int    `json:"status,omitzero"`
	Reason  string `json:"reason,omitzero"`
}

func writeJSON(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func writeDeleted(w io.Writer, kind string, id int64) error {
	return writeJSON(w, result{Deleted: true, ID: id, Type: kind})
}

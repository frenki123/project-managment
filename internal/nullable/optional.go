package nullable

import json "encoding/json/v2"

// Optional represents an incoming JSON field with three states: absent,
// present with null, or present with a value. Apply merges the field into the
// current value, using T's zero value for an explicit null.
type Optional[T any] struct {
	Value *T
	Set   bool
}

func (o Optional[T]) Apply(current T) T {
	if !o.Set {
		return current
	}
	if o.Value == nil {
		var zero T
		return zero
	}
	return *o.Value
}

func Set[T any](value T) *Optional[T] {
	return &Optional[T]{Value: &value, Set: true}
}

func Clear[T any]() *Optional[T] {
	return &Optional[T]{Set: true}
}

func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Value = nil
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}

func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if o.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*o.Value)
}

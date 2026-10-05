package nullable

import json "encoding/json/v2"

// Optional represents an incoming JSON field with three states: absent,
// present with null, or present with a value. Apply merges the field into the
// current value, using T's zero value for an explicit null.
type Optional[T any] struct {
	Value   *T
	Present bool
}

// IsZero reports whether the field is absent, letting omitzero omit it from
// marshaled patches while still emitting explicit null for a present-null.
func (o Optional[T]) IsZero() bool { return !o.Present }

// HasValue reports whether the field carries a concrete value (present and not
// an explicit null).
func (o Optional[T]) HasValue() bool { return o.Present && o.Value != nil }

// IsNull reports whether the field was explicitly present as null.
func (o Optional[T]) IsNull() bool { return o.Present && !o.HasValue() }

// Apply merges the field into current: absent → current, null → T's zero value
// ("no value": nil/""/0), present value → the value. A nil receiver or an
// absent field is a no-op.
func (o *Optional[T]) Apply(current T) T {
	if o == nil || !o.Present {
		return current
	}
	if o.Value == nil {
		var zero T
		return zero
	}
	return *o.Value
}

func Present[T any](value T) Optional[T] {
	return Optional[T]{Value: &value, Present: true}
}

func Clear[T any]() Optional[T] {
	return Optional[T]{Present: true}
}

func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Present = true
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

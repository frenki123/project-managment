package nullable

import (
	json "encoding/json/v2"
	"testing"
)

func TestOptionalRoundTripStates(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		present bool
		want    *string
	}{
		{name: "absent", in: `{}`, want: nil},
		{name: "null", in: `null`, present: true, want: nil},
		{name: "value", in: `"value"`, present: true, want: new("value")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got Optional[string]
			if tc.in == `{}` {
				var wrapper struct {
					Field Optional[string] `json:"field,omitzero"`
				}
				if err := json.Unmarshal([]byte(tc.in), &wrapper); err != nil {
					t.Fatal(err)
				}
				got = wrapper.Field
			} else if err := json.Unmarshal([]byte(tc.in), &got); err != nil {
				t.Fatal(err)
			}
			if got.Present != tc.present {
				t.Fatalf("present = %v, want %v", got.Present, tc.present)
			}
			if got.Value != tc.want && (got.Value == nil || tc.want == nil || *got.Value != *tc.want) {
				t.Fatalf("value = %v, want %v", got.Value, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name  string
		value Optional[string]
		want  string
	}{
		{name: "null", value: Clear[string](), want: "null"},
		{name: "value", value: Present("value"), want: `"value"`},
	} {
		t.Run("marshal_"+tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.value)
			if err != nil || string(data) != tc.want {
				t.Fatalf("got %s, %v", data, err)
			}
		})
	}
	for _, tc := range []struct {
		name  string
		value Optional[string]
		want  string
	}{
		{name: "absent", value: Optional[string]{}, want: "{}"},
		{name: "null", value: Clear[string](), want: `{"field":null}`},
		{name: "value", value: Present("value"), want: `{"field":"value"}`},
	} {
		t.Run("marshal_patch_"+tc.name, func(t *testing.T) {
			var patch struct {
				Field Optional[string] `json:"field,omitzero"`
			}
			patch.Field = tc.value
			data, err := json.Marshal(patch)
			if err != nil || string(data) != tc.want {
				t.Fatalf("got %s, %v", data, err)
			}
		})
	}
}

func TestOptionalApply(t *testing.T) {
	var absent Optional[string]
	if got := absent.Apply("current"); got != "current" {
		t.Fatal(got)
	}
	var cleared = Clear[string]()
	if got := cleared.Apply("current"); got != "" {
		t.Fatal(got)
	}
	var present = Present("next")
	if got := present.Apply("current"); got != "next" {
		t.Fatal(got)
	}
	var nilOptional *Optional[string]
	if got := nilOptional.Apply("current"); got != "current" {
		t.Fatal(got)
	}
}

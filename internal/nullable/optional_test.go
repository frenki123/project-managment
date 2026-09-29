package nullable

import (
	json "encoding/json/v2"
	"testing"
)

func TestOptionalRoundTripStates(t *testing.T) {
	tests := []struct {
		name string
		in   string
		set  bool
		want *string
	}{
		{name: "absent", in: `{}`, want: nil},
		{name: "null", in: `null`, set: true, want: nil},
		{name: "value", in: `"value"`, set: true, want: new("value")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got Optional[string]
			if tc.in == `{}` {
				var wrapper struct {
					Field Optional[string] `json:"field"`
				}
				if err := json.Unmarshal([]byte(tc.in), &wrapper); err != nil {
					t.Fatal(err)
				}
				got = wrapper.Field
			} else if err := json.Unmarshal([]byte(tc.in), &got); err != nil {
				t.Fatal(err)
			}
			if got.Set != tc.set {
				t.Fatalf("set = %v, want %v", got.Set, tc.set)
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
		{name: "null", value: *Clear[string](), want: "null"},
		{name: "value", value: *Set("value"), want: `"value"`},
	} {
		t.Run("marshal_"+tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.value)
			if err != nil || string(data) != tc.want {
				t.Fatalf("got %s, %v", data, err)
			}
		})
	}
}

func TestOptionalApply(t *testing.T) {
	if got := (Optional[string]{}).Apply("current"); got != "current" {
		t.Fatal(got)
	}
	if got := Clear[string]().Apply("current"); got != "" {
		t.Fatal(got)
	}
	if got := Set("next").Apply("current"); got != "next" {
		t.Fatal(got)
	}
}

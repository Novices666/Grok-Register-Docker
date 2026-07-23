package main

import "testing"

func TestEnvIntRange(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  int
	}{
		{name: "valid", value: "37", want: 37},
		{name: "empty", value: "", want: 10},
		{name: "invalid", value: "many", want: 10},
		{name: "below minimum", value: "0", want: 10},
		{name: "above maximum", value: "10001", want: 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TEST_TARGET", tc.value)
			if got := envIntRange("TEST_TARGET", 10, 1, 10000); got != tc.want {
				t.Fatalf("envIntRange() = %d, want %d", got, tc.want)
			}
		})
	}
}

package stalk

import "testing"

func TestNormalizeTarget(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"Stan", "stan", true},
		{"@Stan", "stan", true},
		{"  @Dave  ", "dave", true},
		{"stan_the_man", "stan_the_man", true},
		{"", "", false},
		{"@@stan", "", false},
		{"has space", "", false},
		{"bad!name", "", false},
		{"thisloginistoolongtobevalidok", "", false},
	}
	for _, tc := range cases {
		got, ok := NormalizeTarget(tc.in)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("NormalizeTarget(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

package verbs

import (
	"errors"
	"strings"
	"testing"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overseer/wire"
)

func TestParseAppoint(t *testing.T) {
	tests := []struct {
		name        string
		input       []string
		wantOffice  string
		wantID      int
		wantErr     bool
		wantReject  bool // wantErr == true AND the error is *wire.RejectReason
		errMatch    string
		rejectMatch string
	}{
		{
			name:       "position then id",
			input:      []string{"manager", "8423"},
			wantOffice: "manager", wantID: 8423,
		},
		{
			name:       "id then position (reversed)",
			input:      []string{"8423", "broker"},
			wantOffice: "broker", wantID: 8423,
		},
		{
			name:       "leading # on id is stripped",
			input:      []string{"manager", "#8423"},
			wantOffice: "manager", wantID: 8423,
		},
		{
			name:       "doctor synonym for chief medical",
			input:      []string{"doctor", "100"},
			wantOffice: "doctor", wantID: 100,
		},
		{name: "missing args", input: []string{"manager"}, wantErr: true},
		{name: "too many args", input: []string{"manager", "1", "extra"}, wantErr: true},
		{name: "negative id rejected", input: []string{"manager", "-5"}, wantErr: true, errMatch: "invalid unit id"},
		{name: "two numbers", input: []string{"42", "8423"}, wantErr: true},
		{name: "two keywords", input: []string{"manager", "broker"}, wantErr: true},
		{name: "unknown position", input: []string{"jester", "1"}, wantErr: true, errMatch: "unknown position"},
		{
			name:        "captain deferred",
			input:       []string{"captain", "1"},
			wantErr:     true,
			wantReject:  true,
			rejectMatch: "captain needs a squad",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAppoint(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.wantReject {
					var rr *wire.RejectReason
					if !errors.As(err, &rr) {
						t.Errorf("expected *wire.RejectReason, got %T", err)
					} else if tt.rejectMatch != "" && !strings.Contains(rr.Msg, tt.rejectMatch) {
						t.Errorf("RejectReason %q does not contain %q", rr.Msg, tt.rejectMatch)
					}
				}
				if tt.errMatch != "" && !strings.Contains(err.Error(), tt.errMatch) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMatch)
				}
				return
			}
			if got.Office != tt.wantOffice || got.UnitID != tt.wantID {
				t.Errorf("got office=%q id=%d, want office=%q id=%d",
					got.Office, got.UnitID, tt.wantOffice, tt.wantID)
			}
		})
	}
}

func TestSubmitAppoint_LuaSubstitutes(t *testing.T) {
	ex := &fakeExecutor{}
	err := SubmitAppoint(ex, wire.Action{
		Kind: wire.ActionKindAppoint, Office: "manager", UnitID: 17566,
	})
	if err != nil {
		t.Fatalf("SubmitAppoint: %v", err)
	}
	lua := ex.lastLua(t)
	assertContainsAll(t, lua,
		"local UNIT_ID = 17566",
		`local CODE = "MANAGER"`,
		"df.global.plotinfo.main.fortress_entity",
		"newfig.entity_links:insert",
	)
}

func TestSubmitAppoint_TranslatesOfficeToCode(t *testing.T) {
	cases := []struct {
		office, code string
	}{
		{"manager", "MANAGER"},
		{"bookkeeper", "BOOKKEEPER"},
		{"broker", "BROKER"},
		{"doctor", "CHIEF_MEDICAL_DWARF"},
		{"commander", "MILITIA_COMMANDER"},
	}
	for _, c := range cases {
		t.Run(c.office, func(t *testing.T) {
			ex := &fakeExecutor{}
			if err := SubmitAppoint(ex, wire.Action{Office: c.office, UnitID: 1}); err != nil {
				t.Fatalf("SubmitAppoint: %v", err)
			}
			assertContains(t, ex.lastLua(t), `"`+c.code+`"`)
		})
	}
}

func TestSubmitAppoint_UnknownOfficeErrors(t *testing.T) {
	ex := &fakeExecutor{}
	if err := SubmitAppoint(ex, wire.Action{Office: "jester", UnitID: 1}); err == nil {
		t.Error("expected error for unknown office")
	}
}

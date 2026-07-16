package palworld

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestPlayersJSONRequiresPresentArray(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		valid bool
	}{
		{name: "non-empty", body: `{"players":[{"name":"One","userId":"steam_1"}]}`, valid: true},
		{name: "empty", body: `{"players":[]}`, valid: true},
		{name: "missing", body: `{}`},
		{name: "null", body: `{"players":null}`},
		{name: "not-array", body: `{"players":{}}`},
		{name: "invalid-json", body: `{"players":[`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var snapshot Players
			err := json.Unmarshal([]byte(test.body), &snapshot)
			if test.valid && err != nil {
				t.Fatalf("valid snapshot rejected: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("invalid snapshot accepted")
			}
			if !test.valid && test.name != "invalid-json" && !errors.Is(err, ErrInvalidPlayersSnapshot) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestValidatePlayersStrictIdentityAndUniqueness(t *testing.T) {
	valid := []Players{
		{Players: []Player{{Name: "One", UserID: "steam_1", PlayerID: "p1"}}},
		{Players: []Player{}},
		{Players: []Player{{Name: "One", UserID: "steam_1"}, {Name: "Two", UserID: "steam_2"}}},
	}
	for index, snapshot := range valid {
		if err := ValidatePlayers(snapshot); err != nil {
			t.Fatalf("valid[%d]=%v", index, err)
		}
	}

	invalid := []Players{
		{Players: []Player{{Name: "One"}}},
		{Players: []Player{{Name: "One", UserID: "xbox_1"}}},
		{Players: []Player{{Name: "One", UserID: "steam_abc"}}},
		{Players: []Player{{Name: "One", UserID: "steam_0"}}},
		{Players: []Player{{Name: "One", UserID: "steam_18446744073709551616"}}},
		{Players: []Player{{Name: " ", UserID: "steam_1"}}},
		{Players: []Player{{Name: "One", UserID: "steam_1"}, {Name: "Again", UserID: "steam_1"}}},
		{Players: []Player{{Name: "One", UserID: "steam_1", PlayerID: "same"}, {Name: "Two", UserID: "steam_2", PlayerID: "same"}}},
	}
	for index, snapshot := range invalid {
		if err := ValidatePlayers(snapshot); !errors.Is(err, ErrInvalidPlayersSnapshot) {
			t.Fatalf("invalid[%d]=%v", index, err)
		}
	}
}

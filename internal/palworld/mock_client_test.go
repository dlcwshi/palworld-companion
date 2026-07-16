package palworld

import (
	"context"
	"testing"
)

func TestMockClientExercisesRenameAndOfflineTransitions(t *testing.T) {
	client := &MockClient{}
	first, err := client.GetPlayers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.GetPlayers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	third, err := client.GetPlayers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Players) != 4 || len(second.Players) != 3 || len(third.Players) != 2 {
		t.Fatalf("snapshot sizes=%d,%d,%d", len(first.Players), len(second.Players), len(third.Players))
	}
	if first.Players[0].Name != "Moss" || second.Players[0].Name != "Moss Prime" {
		t.Fatalf("rename=%q -> %q", first.Players[0].Name, second.Players[0].Name)
	}
	for _, snapshot := range []Players{first, second, third} {
		if err := ValidatePlayers(snapshot); err != nil {
			t.Fatalf("invalid mock snapshot: %v", err)
		}
	}
}

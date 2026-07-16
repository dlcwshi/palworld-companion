package palworld

import (
	"context"
	"sync"
)

type MockClient struct {
	mu          sync.Mutex
	playerCalls int
}

func (*MockClient) GetInfo(context.Context) (Info, error) {
	version, description := "mock-0.1", "Windows local development server"
	return Info{Version: &version, ServerName: "Palworld Community Server", Description: &description}, nil
}
func (*MockClient) GetMetrics(context.Context) (Metrics, error) {
	return Metrics{ServerFPS: 57, CurrentPlayers: 4, MaxPlayers: 50, Uptime: 18 * 60 * 60, Days: 326, BaseCampCount: 12}, nil
}
func (m *MockClient) GetPlayers(context.Context) (Players, error) {
	m.mu.Lock()
	phase := m.playerCalls % 3
	m.playerCalls++
	m.mu.Unlock()

	level1, level2, level3 := 55, 42, 31
	ping1, ping2, ping3 := 35.0, 48.0, 67.0
	x1, y1, x2, y2 := 100.5, -200.5, 342.0, 88.25
	players := []Player{
		{Name: "Moss", UserID: "steam_76561198000000101", PlayerID: "mock-player-1", AccountName: "mock-account-1", Level: &level1, Ping: &ping1, LocationX: &x1, LocationY: &y1},
		{Name: "Luna", UserID: "steam_76561198000000102", PlayerID: "mock-player-2", AccountName: "mock-account-2", Level: &level2, Ping: &ping2, LocationX: &x2, LocationY: &y2},
		{Name: "Builder", UserID: "steam_76561198000000103", PlayerID: "mock-player-3", AccountName: "mock-account-3", Level: &level3, Ping: &ping3},
		{Name: "Wanderer", UserID: "steam_76561198000000104", PlayerID: "mock-player-4", AccountName: "mock-account-4"},
	}
	if phase > 0 {
		players[0].Name = "Moss Prime"
		players = players[:3]
	}
	if phase == 2 {
		players = players[:2]
	}
	return Players{Players: players}, nil
}

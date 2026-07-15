package palworld

import "context"

type MockClient struct{}

func (MockClient) GetInfo(context.Context) (Info, error) {
	version, description := "mock-0.1", "Windows local development server"
	return Info{Version: &version, ServerName: "Palworld Community Server", Description: &description}, nil
}
func (MockClient) GetMetrics(context.Context) (Metrics, error) {
	return Metrics{ServerFPS: 57, CurrentPlayers: 4, MaxPlayers: 50, Uptime: 18 * 60 * 60, Days: 326, BaseCampCount: 12}, nil
}
func (MockClient) GetPlayers(context.Context) (Players, error) {
	level1, level2, level3 := 55, 42, 31
	ping1, ping2, ping3 := 35.0, 48.0, 67.0
	x1, y1, x2, y2 := 100.5, -200.5, 342.0, 88.25
	return Players{Players: []Player{
		{Name: "Moss", Level: &level1, Ping: &ping1, LocationX: &x1, LocationY: &y1},
		{Name: "Luna", Level: &level2, Ping: &ping2, LocationX: &x2, LocationY: &y2},
		{Name: "Builder", Level: &level3, Ping: &ping3},
		{Name: "Wanderer"},
	}}, nil
}

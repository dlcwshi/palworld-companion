package palworld

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPClientParsesAndAuthenticates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "admin" || p != "secret" {
			t.Error("missing basic auth")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/api/info":
			_, _ = w.Write([]byte(`{"version":"v1","servername":"Test","description":"Desc","worldguid":"private"}`))
		case "/v1/api/metrics":
			_, _ = w.Write([]byte(`{"serverfps":58.4,"currentplayernum":4,"maxplayernum":50,"uptime":90,"basecampnum":12,"days":326}`))
		case "/v1/api/players":
			_, _ = w.Write([]byte(`{"players":[{"name":"P","playerId":"private","userId":"steam_1","ip":"10.0.0.1","ping":35,"location_x":1.5,"location_y":2.5,"level":55}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c, err := NewHTTPClient(server.URL, "admin", "secret", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	info, err := c.GetInfo(context.Background())
	if err != nil || info.ServerName != "Test" {
		t.Fatalf("info=%+v err=%v", info, err)
	}
	metrics, err := c.GetMetrics(context.Background())
	if err != nil || metrics.ServerFPS != 58.4 {
		t.Fatalf("metrics=%+v err=%v", metrics, err)
	}
	players, err := c.GetPlayers(context.Background())
	if err != nil || *players.Players[0].Level != 55 {
		t.Fatalf("players=%+v err=%v", players, err)
	}
}

func TestHTTPClientRejectsNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "secret upstream details", http.StatusUnauthorized)
	}))
	defer server.Close()
	c, _ := NewHTTPClient(server.URL, "", "", time.Second)
	if _, err := c.GetInfo(context.Background()); err == nil {
		t.Fatal("expected status error")
	}
}
func TestHTTPClientRejectsInvalidPlayerSnapshots(t *testing.T) {
	bodies := []string{
		`{}`,
		`{"players":null}`,
		`{"players":[`,
		`{"players":[{"name":"Missing identity"}]}`,
		`{"players":[{"name":"","userId":"steam_1"}]}`,
		`{"players":[{"name":"One","userId":"steam_1"},{"name":"Again","userId":"steam_1"}]}`,
		`{"players":[{"name":"One","userId":"steam_1","playerId":"same"},{"name":"Two","userId":"steam_2","playerId":"same"}]}`,
		`{"players":[]} {"players":[]}`,
	}
	for _, body := range bodies {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
		client, err := NewHTTPClient(server.URL, "", "", time.Second)
		if err != nil {
			server.Close()
			t.Fatal(err)
		}
		_, err = client.GetPlayers(context.Background())
		server.Close()
		if err == nil {
			t.Fatalf("invalid snapshot accepted: %s", body)
		}
	}
}

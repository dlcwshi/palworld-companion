package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const SteamOpenIDEndpoint = "https://steamcommunity.com/openid/login"

type Verifier interface {
	Verify(context.Context, url.Values) (string, error)
}
type SteamVerifier struct {
	Client   *http.Client
	Endpoint string
}

func (v SteamVerifier) Verify(ctx context.Context, values url.Values) (string, error) {
	claimed := values.Get("openid.claimed_id")
	identity := values.Get("openid.identity")
	steamID, err := parseClaimedID(claimed)
	if err != nil || identity != claimed {
		return "", fmt.Errorf("invalid Steam claimed identity")
	}
	check := url.Values{}
	for key, items := range values {
		if strings.HasPrefix(key, "openid.") {
			for _, item := range items {
				check.Add(key, item)
			}
		}
	}
	check.Set("openid.mode", "check_authentication")
	endpoint := v.Endpoint
	if endpoint == "" {
		endpoint = SteamOpenIDEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(check.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := v.Client
	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Steam verification failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "is_valid:true") {
		return "", fmt.Errorf("Steam rejected OpenID assertion")
	}
	return steamID, nil
}

func parseClaimedID(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || strings.ToLower(u.Hostname()) != "steamcommunity.com" || u.Port() != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("invalid claimed_id")
	}
	const prefix = "/openid/id/"
	if !strings.HasPrefix(u.EscapedPath(), prefix) {
		return "", fmt.Errorf("invalid claimed_id path")
	}
	id := strings.TrimPrefix(u.EscapedPath(), prefix)
	if id == "" || strings.Contains(id, "/") {
		return "", fmt.Errorf("invalid SteamID")
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("invalid SteamID")
		}
	}
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil || n == 0 {
		return "", fmt.Errorf("invalid SteamID")
	}
	return id, nil
}

func SafeReturnPath(raw string) (string, error) {
	if raw == "" {
		return "/tasks", nil
	}
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.ContainsAny(raw, "\r\n") {
		return "", fmt.Errorf("unsafe returnTo")
	}
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" {
		return "", fmt.Errorf("unsafe returnTo")
	}
	return raw, nil
}

package palworld

import "context"

// FreshClient adapts a raw Client for tests and command paths that do not use
// the persistent roster service.
type FreshClient struct{ Client Client }

func (f FreshClient) FreshPlayers(ctx context.Context) (Players, error) {
	return GetPlayersFreshForIdentityBinding(ctx, f.Client)
}

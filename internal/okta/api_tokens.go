package okta

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tedilabs/ota/internal/domain"
)

// APITokensAdapter implements domain.APITokensPort.
type APITokensAdapter struct{ client *Client }

func (a *APITokensAdapter) List(ctx context.Context) ([]domain.APIToken, error) {
	u := a.client.buildURL("/api/v1/api-tokens")
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp)
	var raw []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("okta: api-tokens decode: %w", err)
	}
	out := make([]domain.APIToken, 0, len(raw))
	for _, msg := range raw {
		var w wireAPIToken
		if err := json.Unmarshal(msg, &w); err != nil {
			return nil, fmt.Errorf("okta: api-token decode: %w", err)
		}
		out = append(out, mapAPIToken(&w))
	}
	return out, nil
}

type wireAPIToken struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	UserID      string    `json:"userId"`
	ClientName  string    `json:"clientName"`
	Created     time.Time `json:"created"`
	LastUpdated time.Time `json:"lastUpdated"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

func mapAPIToken(w *wireAPIToken) domain.APIToken {
	return domain.APIToken{
		ID:          w.ID,
		Name:        w.Name,
		UserID:      w.UserID,
		ClientName:  w.ClientName,
		Created:     w.Created,
		LastUpdated: w.LastUpdated,
		ExpiresAt:   w.ExpiresAt,
	}
}

package okta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/tedilabs/ota/internal/domain"
)

// AuthenticatorsAdapter implements domain.AuthenticatorsPort against
// /api/v1/authenticators (#F1 v0.2.5). The endpoint returns a flat
// JSON array (no pagination, typical org has < 30 entries) so we use
// a single GET rather than the paged-iterator helper.
type AuthenticatorsAdapter struct{ client *Client }

func (a *AuthenticatorsAdapter) List(ctx context.Context) ([]domain.Authenticator, error) {
	u := a.client.buildURL("/api/v1/authenticators")
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp)
	var raw []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("okta: authenticators decode: %w", err)
	}
	out := make([]domain.Authenticator, 0, len(raw))
	for _, msg := range raw {
		var w wireAuthenticator
		if err := json.Unmarshal(msg, &w); err != nil {
			return nil, fmt.Errorf("okta: authenticator decode: %w", err)
		}
		out = append(out, mapAuthenticator(&w, msg))
	}
	return out, nil
}

func (a *AuthenticatorsAdapter) Get(ctx context.Context, id string) (domain.Authenticator, error) {
	u := a.client.buildURL("/api/v1/authenticators/" + url.PathEscape(id))
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return domain.Authenticator{}, err
	}
	defer drainAndClose(resp)
	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return domain.Authenticator{}, fmt.Errorf("okta: authenticator decode: %w", err)
	}
	var w wireAuthenticator
	if err := json.Unmarshal(raw, &w); err != nil {
		return domain.Authenticator{}, fmt.Errorf("okta: authenticator decode: %w", err)
	}
	return mapAuthenticator(&w, raw), nil
}

// wireAuthenticator mirrors the fields ota actually consumes off the
// /api/v1/authenticators payload. Unknown fields are tolerated; the
// raw bytes get round-tripped via domain.Authenticator-equivalent
// extension if needed (the screen's Raw tab reads `_raw`).
type wireAuthenticator struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Provider    *struct{} `json:"provider"` // unused for now; presence-only
	ProviderRaw struct {
		Type string `json:"type"`
	} `json:"-"`
	Created     time.Time `json:"created"`
	LastUpdated time.Time `json:"lastUpdated"`
}

func mapAuthenticator(w *wireAuthenticator, _ json.RawMessage) domain.Authenticator {
	return domain.Authenticator{
		ID:          w.ID,
		Type:        domain.AuthenticatorType(w.Type),
		Key:         w.Key,
		Name:        w.Name,
		Status:      domain.AuthenticatorStatus(w.Status),
		Provider:    domain.AuthenticatorProvider("OKTA"),
		Created:     w.Created,
		LastUpdated: w.LastUpdated,
	}
}

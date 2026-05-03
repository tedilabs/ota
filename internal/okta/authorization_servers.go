package okta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/tedilabs/ota/internal/domain"
)

// AuthorizationServersAdapter implements domain.AuthorizationServersPort.
type AuthorizationServersAdapter struct{ client *Client }

func (a *AuthorizationServersAdapter) List(ctx context.Context) ([]domain.AuthorizationServer, error) {
	u := a.client.buildURL("/api/v1/authorizationServers")
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp)
	var raw []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("okta: authorization servers decode: %w", err)
	}
	out := make([]domain.AuthorizationServer, 0, len(raw))
	for _, msg := range raw {
		var w wireAuthorizationServer
		if err := json.Unmarshal(msg, &w); err != nil {
			return nil, fmt.Errorf("okta: authorization server decode: %w", err)
		}
		out = append(out, mapAuthorizationServer(&w))
	}
	return out, nil
}

func (a *AuthorizationServersAdapter) Get(ctx context.Context, id string) (domain.AuthorizationServer, error) {
	u := a.client.buildURL("/api/v1/authorizationServers/" + url.PathEscape(id))
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return domain.AuthorizationServer{}, err
	}
	defer drainAndClose(resp)
	var w wireAuthorizationServer
	if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
		return domain.AuthorizationServer{}, fmt.Errorf("okta: authorization server decode: %w", err)
	}
	return mapAuthorizationServer(&w), nil
}

type wireAuthorizationServer struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Audiences   []string  `json:"audiences"`
	Issuer      string    `json:"issuer"`
	Status      string    `json:"status"`
	Created     time.Time `json:"created"`
	LastUpdated time.Time `json:"lastUpdated"`
}

func mapAuthorizationServer(w *wireAuthorizationServer) domain.AuthorizationServer {
	return domain.AuthorizationServer{
		ID:          w.ID,
		Name:        w.Name,
		Description: w.Description,
		Audiences:   append([]string(nil), w.Audiences...),
		Issuer:      w.Issuer,
		Status:      domain.AuthorizationServerStatus(w.Status),
		Created:     w.Created,
		LastUpdated: w.LastUpdated,
	}
}

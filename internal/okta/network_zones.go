package okta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/tedilabs/ota/internal/domain"
)

// NetworkZonesAdapter implements domain.NetworkZonesPort.
type NetworkZonesAdapter struct{ client *Client }

func (a *NetworkZonesAdapter) List(ctx context.Context) ([]domain.NetworkZone, error) {
	u := a.client.buildURL("/api/v1/zones")
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp)
	var raw []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("okta: zones decode: %w", err)
	}
	out := make([]domain.NetworkZone, 0, len(raw))
	for _, msg := range raw {
		var w wireNetworkZone
		if err := json.Unmarshal(msg, &w); err != nil {
			return nil, fmt.Errorf("okta: zone decode: %w", err)
		}
		out = append(out, mapNetworkZone(&w))
	}
	return out, nil
}

func (a *NetworkZonesAdapter) Get(ctx context.Context, id string) (domain.NetworkZone, error) {
	u := a.client.buildURL("/api/v1/zones/" + url.PathEscape(id))
	resp, err := a.client.doGet(ctx, u)
	if err != nil {
		return domain.NetworkZone{}, err
	}
	defer drainAndClose(resp)
	var w wireNetworkZone
	if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
		return domain.NetworkZone{}, fmt.Errorf("okta: zone decode: %w", err)
	}
	return mapNetworkZone(&w), nil
}

type wireNetworkZone struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	Usage       string    `json:"usage"`
	System      bool      `json:"system"`
	Created     time.Time `json:"created"`
	LastUpdated time.Time `json:"lastUpdated"`
}

func mapNetworkZone(w *wireNetworkZone) domain.NetworkZone {
	return domain.NetworkZone{
		ID:          w.ID,
		Name:        w.Name,
		Type:        domain.NetworkZoneType(w.Type),
		Status:      domain.NetworkZoneStatus(w.Status),
		Usage:       domain.NetworkZoneUsage(w.Usage),
		System:      w.System,
		Created:     w.Created,
		LastUpdated: w.LastUpdated,
	}
}

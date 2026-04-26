package okta

import (
	"encoding/json"
	"time"

	"github.com/tedilabs/ota/internal/domain"
)

// Wire structs mirror Okta's JSON response shapes. They never escape this
// package — map*() constructors translate them into domain.* types.

// --- Users ---------------------------------------------------------------

type wireUser struct {
	ID              string          `json:"id"`
	Status          string          `json:"status"`
	Created         string          `json:"created"`
	Activated       string          `json:"activated"`
	LastLogin       string          `json:"lastLogin"`
	LastUpdated     string          `json:"lastUpdated"`
	StatusChanged   string          `json:"statusChanged"`
	PasswordChanged string          `json:"passwordChanged"`
	Profile         json.RawMessage `json:"profile"`
	Credentials     struct {
		Provider struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"provider"`
	} `json:"credentials"`
}

type wireUserProfile struct {
	Login       string `json:"login"`
	Email       string `json:"email"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	DisplayName string `json:"displayName"`
	MobilePhone string `json:"mobilePhone"`
	SecondEmail string `json:"secondEmail"`
	Department  string `json:"department"`
}

func mapUser(wu *wireUser) domain.User {
	var prof wireUserProfile
	extras := map[string]any{}
	if len(wu.Profile) > 0 {
		_ = json.Unmarshal(wu.Profile, &prof)
		var generic map[string]any
		if err := json.Unmarshal(wu.Profile, &generic); err == nil {
			for k, v := range generic {
				switch k {
				case "login", "email", "firstName", "lastName", "displayName",
					"mobilePhone", "secondEmail", "department":
				default:
					extras[k] = v
				}
			}
		}
	}
	return domain.User{
		ID:              wu.ID,
		Status:          domain.UserStatus(wu.Status),
		Created:         parseOktaTime(wu.Created),
		Activated:       parseOktaTimePtr(wu.Activated),
		LastLogin:       parseOktaTimePtr(wu.LastLogin),
		LastUpdated:     parseOktaTime(wu.LastUpdated),
		StatusChanged:   parseOktaTimePtr(wu.StatusChanged),
		PasswordChanged: parseOktaTimePtr(wu.PasswordChanged),
		Profile: domain.UserProfile{
			Login:       prof.Login,
			Email:       prof.Email,
			FirstName:   prof.FirstName,
			LastName:    prof.LastName,
			DisplayName: prof.DisplayName,
			MobilePhone: prof.MobilePhone,
			SecondEmail: prof.SecondEmail,
			Department:  prof.Department,
			Extras:      extras,
		},
		Credentials: domain.UserCredentials{
			Provider:     wu.Credentials.Provider.Name,
			ProviderType: wu.Credentials.Provider.Type,
		},
	}
}

// --- Groups --------------------------------------------------------------

type wireGroup struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Profile struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"profile"`
	Created               string `json:"created"`
	LastUpdated           string `json:"lastUpdated"`
	LastMembershipUpdated string `json:"lastMembershipUpdated"`
}

func mapGroup(wg *wireGroup) domain.Group {
	return domain.Group{
		ID:                    wg.ID,
		Type:                  domain.GroupType(wg.Type),
		Profile:               domain.GroupProfile{Name: wg.Profile.Name, Description: wg.Profile.Description},
		Created:               parseOktaTime(wg.Created),
		LastUpdated:           parseOktaTime(wg.LastUpdated),
		LastMembershipUpdated: parseOktaTimePtr(wg.LastMembershipUpdated),
	}
}

// --- Group Rules ---------------------------------------------------------

type wireGroupRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Created     string `json:"created"`
	LastUpdated string `json:"lastUpdated"`
	Conditions  struct {
		Expression struct {
			Value string `json:"value"`
			Type  string `json:"type"`
		} `json:"expression"`
	} `json:"conditions"`
	Actions struct {
		AssignUserToGroups struct {
			GroupIDs []string `json:"groupIds"`
		} `json:"assignUserToGroups"`
	} `json:"actions"`
}

func mapGroupRule(wr *wireGroupRule) domain.GroupRule {
	return domain.GroupRule{
		ID:             wr.ID,
		Name:           wr.Name,
		Status:         domain.GroupRuleStatus(wr.Status),
		Expression:     wr.Conditions.Expression.Value,
		TargetGroupIDs: wr.Actions.AssignUserToGroups.GroupIDs,
		Created:        parseOktaTime(wr.Created),
		LastUpdated:    parseOktaTime(wr.LastUpdated),
	}
}

// --- Policies ------------------------------------------------------------

type wirePolicy struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Priority    int    `json:"priority"`
	Status      string `json:"status"`
	System      bool   `json:"system"`
	Created     string `json:"created"`
	LastUpdated string `json:"lastUpdated"`
}

func mapPolicy(wp *wirePolicy, raw json.RawMessage) domain.Policy {
	return domain.Policy{
		ID:          wp.ID,
		Name:        wp.Name,
		Description: wp.Description,
		Type:        domain.PolicyType(wp.Type),
		Priority:    wp.Priority,
		Status:      domain.PolicyStatus(wp.Status),
		System:      wp.System,
		Created:     parseOktaTime(wp.Created),
		LastUpdated: parseOktaTime(wp.LastUpdated),
		Raw:         raw,
	}
}

type wirePolicyRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Priority    int    `json:"priority"`
	Status      string `json:"status"`
	System      bool   `json:"system"`
	Created     string `json:"created"`
	LastUpdated string `json:"lastUpdated"`
}

func mapPolicyRule(wr *wirePolicyRule, raw json.RawMessage) domain.PolicyRule {
	return domain.PolicyRule{
		ID:          wr.ID,
		Name:        wr.Name,
		Priority:    wr.Priority,
		Status:      domain.PolicyStatus(wr.Status),
		System:      wr.System,
		Created:     parseOktaTime(wr.Created),
		LastUpdated: parseOktaTime(wr.LastUpdated),
		Raw:         raw,
	}
}

// --- Logs ----------------------------------------------------------------

type wireLogEvent struct {
	UUID       string `json:"uuid"`
	Published  string `json:"published"`
	Severity   string `json:"severity"`
	EventType  string `json:"eventType"`
	DisplayMsg string `json:"displayMessage"`
	Actor      struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		DisplayName string `json:"displayName"`
		AlternateID string `json:"alternateId"`
	} `json:"actor"`
	Target []struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		DisplayName string `json:"displayName"`
		AlternateID string `json:"alternateId"`
	} `json:"target"`
	Client struct {
		IPAddress string `json:"ipAddress"`
		UserAgent struct {
			RawUserAgent string `json:"rawUserAgent"`
		} `json:"userAgent"`
		GeographicalContext struct {
			Country string `json:"country"`
			State   string `json:"state"`
			City    string `json:"city"`
		} `json:"geographicalContext"`
	} `json:"client"`
	Outcome struct {
		Result string `json:"result"`
		Reason string `json:"reason"`
	} `json:"outcome"`
	Request     json.RawMessage `json:"request"`
	DebugCtx    json.RawMessage `json:"debugContext"`
	Transaction json.RawMessage `json:"transaction"`
}

func mapLogEvent(we *wireLogEvent, raw json.RawMessage) domain.LogEvent {
	targets := make([]domain.Target, 0, len(we.Target))
	for _, t := range we.Target {
		targets = append(targets, domain.Target{
			ID:          t.ID,
			Type:        t.Type,
			DisplayName: t.DisplayName,
			AlternateID: t.AlternateID,
		})
	}
	return domain.LogEvent{
		UUID:       we.UUID,
		Published:  parseOktaTime(we.Published),
		Severity:   domain.Severity(we.Severity),
		EventType:  we.EventType,
		DisplayMsg: we.DisplayMsg,
		Actor: domain.Actor{
			ID:          we.Actor.ID,
			Type:        domain.ActorType(we.Actor.Type),
			DisplayName: we.Actor.DisplayName,
			AlternateID: we.Actor.AlternateID,
		},
		Targets: targets,
		Client: domain.Client{
			IPAddress: we.Client.IPAddress,
			UserAgent: we.Client.UserAgent.RawUserAgent,
			Geo: domain.Geo{
				Country: we.Client.GeographicalContext.Country,
				State:   we.Client.GeographicalContext.State,
				City:    we.Client.GeographicalContext.City,
			},
		},
		Outcome: domain.Outcome{
			Result: domain.OutcomeResult(we.Outcome.Result),
			Reason: we.Outcome.Reason,
		},
		Request:     we.Request,
		Debug:       we.DebugCtx,
		Transaction: we.Transaction,
		Raw:         raw,
	}
}

// --- Factors -------------------------------------------------------------

type wireFactor struct {
	ID          string `json:"id"`
	FactorType  string `json:"factorType"`
	Provider    string `json:"provider"`
	VendorName  string `json:"vendorName"`
	Status      string `json:"status"`
	Created     string `json:"created"`
	LastUpdated string `json:"lastUpdated"`
	Profile     struct {
		PhoneNumber  string `json:"phoneNumber"`
		Email        string `json:"email"`
		CredentialID string `json:"credentialId"`
		DeviceType   string `json:"deviceType"`
		Name         string `json:"name"`
	} `json:"profile"`
}

func mapFactor(wf *wireFactor) domain.Factor {
	return domain.Factor{
		ID:          wf.ID,
		Type:        domain.FactorType(wf.FactorType),
		Provider:    wf.Provider,
		VendorName:  wf.VendorName,
		Status:      domain.FactorStatus(wf.Status),
		Created:     parseOktaTime(wf.Created),
		LastUpdated: parseOktaTime(wf.LastUpdated),
		Profile: domain.FactorProfile{
			PhoneNumber:  wf.Profile.PhoneNumber,
			Email:        wf.Profile.Email,
			CredentialID: wf.Profile.CredentialID,
			DeviceType:   wf.Profile.DeviceType,
			Name:         wf.Profile.Name,
		},
	}
}

// --- helpers -------------------------------------------------------------

// parseOktaTime parses Okta's ISO-8601 UTC timestamps. Returns zero time on
// empty or unparseable input.
func parseOktaTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		if t2, err2 := time.Parse(time.RFC3339, s); err2 == nil {
			return t2.UTC()
		}
		return time.Time{}
	}
	return t.UTC()
}

func parseOktaTimePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	t := parseOktaTime(s)
	if t.IsZero() {
		return nil
	}
	return &t
}

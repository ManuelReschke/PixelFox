package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
)

const (
	defaultPatreonAuthorizeURL = "https://www.patreon.com/oauth2/authorize"
	defaultPatreonTokenURL     = "https://www.patreon.com/api/oauth2/token"
	defaultPatreonAPIBaseURL   = "https://www.patreon.com/api/oauth2/v2"
)

type PatreonClient struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string

	AuthorizeURL string
	TokenURL     string
	APIBaseURL   string

	HTTPClient *http.Client
}

type PatreonTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

type PatreonIdentity struct {
	PatreonUserID string
	Email         string
	MembershipID  string
	PatronStatus  string
	IsFollower    bool
	TierIDs       []string
}

type PatreonWebhookMemberEvent struct {
	MemberID      string
	PatreonUserID string
	PatronStatus  string
	IsFollower    bool
	TierIDs       []string
}

func NewPatreonClientFromEnv() *PatreonClient {
	base := strings.TrimRight(env.GetEnv("PUBLIC_DOMAIN", ""), "/")
	redirectURI := strings.TrimSpace(env.GetEnv("PATREON_REDIRECT_URI", ""))
	if redirectURI == "" && base != "" {
		redirectURI = base + "/user/settings/billing/patreon/callback"
	}

	return &PatreonClient{
		ClientID:     strings.TrimSpace(env.GetEnv("PATREON_CLIENT_ID", "")),
		ClientSecret: strings.TrimSpace(env.GetEnv("PATREON_CLIENT_SECRET", "")),
		RedirectURI:  redirectURI,
		AuthorizeURL: strings.TrimSpace(env.GetEnv("PATREON_AUTHORIZE_URL", defaultPatreonAuthorizeURL)),
		TokenURL:     strings.TrimSpace(env.GetEnv("PATREON_TOKEN_URL", defaultPatreonTokenURL)),
		APIBaseURL:   strings.TrimSpace(env.GetEnv("PATREON_API_BASE_URL", defaultPatreonAPIBaseURL)),
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *PatreonClient) AuthorizeURLWithState(state string) (string, error) {
	if strings.TrimSpace(c.ClientID) == "" {
		return "", errors.New("PATREON_CLIENT_ID is not configured")
	}
	if strings.TrimSpace(c.RedirectURI) == "" {
		return "", errors.New("PATREON_REDIRECT_URI is not configured")
	}
	u, err := url.Parse(c.AuthorizeURL)
	if err != nil {
		return "", fmt.Errorf("invalid PATREON_AUTHORIZE_URL: %w", err)
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", c.ClientID)
	q.Set("redirect_uri", c.RedirectURI)
	q.Set("scope", "identity identity.memberships")
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *PatreonClient) ExchangeCode(ctx context.Context, code string) (*PatreonTokenResponse, error) {
	if strings.TrimSpace(c.ClientID) == "" || strings.TrimSpace(c.ClientSecret) == "" {
		return nil, errors.New("PATREON_CLIENT_ID/PATREON_CLIENT_SECRET are not configured")
	}
	if strings.TrimSpace(c.RedirectURI) == "" {
		return nil, errors.New("PATREON_REDIRECT_URI is not configured")
	}
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("oauth code is required")
	}

	form := url.Values{}
	form.Set("code", strings.TrimSpace(code))
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	form.Set("redirect_uri", c.RedirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("patreon token exchange failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var out PatreonTokenResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		return nil, errors.New("patreon token exchange returned empty access_token")
	}
	return &out, nil
}

func (c *PatreonClient) GetIdentity(ctx context.Context, accessToken string) (*PatreonIdentity, error) {
	token := strings.TrimSpace(accessToken)
	if token == "" {
		return nil, errors.New("access token is required")
	}

	baseURL := strings.TrimRight(c.APIBaseURL, "/")
	u, err := url.Parse(baseURL + "/identity")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("include", "memberships.currently_entitled_tiers")
	q.Set("fields[user]", "email")
	q.Set("fields[member]", "patron_status,is_follower,last_charge_status,last_charge_date")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("patreon identity request failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	type relData struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	type rawResponse struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				Email string `json:"email"`
			} `json:"attributes"`
			Relationships struct {
				Memberships struct {
					Data []relData `json:"data"`
				} `json:"memberships"`
			} `json:"relationships"`
		} `json:"data"`
		Included []struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Attributes struct {
				PatronStatus string `json:"patron_status"`
				IsFollower   bool   `json:"is_follower"`
			} `json:"attributes"`
			Relationships struct {
				CurrentlyEntitledTiers struct {
					Data []relData `json:"data"`
				} `json:"currently_entitled_tiers"`
			} `json:"relationships"`
		} `json:"included"`
	}

	var raw rawResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if strings.TrimSpace(raw.Data.ID) == "" {
		return nil, errors.New("patreon identity response missing user id")
	}

	membershipID := ""
	if len(raw.Data.Relationships.Memberships.Data) > 0 {
		membershipID = strings.TrimSpace(raw.Data.Relationships.Memberships.Data[0].ID)
	}

	var status string
	isFollower := false
	var tierIDs []string
	for _, inc := range raw.Included {
		if inc.Type != "member" {
			continue
		}
		if membershipID != "" && inc.ID != membershipID {
			continue
		}
		if membershipID == "" {
			membershipID = strings.TrimSpace(inc.ID)
		}
		status = strings.TrimSpace(inc.Attributes.PatronStatus)
		isFollower = inc.Attributes.IsFollower
		for _, td := range inc.Relationships.CurrentlyEntitledTiers.Data {
			if tid := strings.TrimSpace(td.ID); tid != "" {
				tierIDs = append(tierIDs, tid)
			}
		}
		break
	}

	return &PatreonIdentity{
		PatreonUserID: strings.TrimSpace(raw.Data.ID),
		Email:         strings.TrimSpace(raw.Data.Attributes.Email),
		MembershipID:  membershipID,
		PatronStatus:  status,
		IsFollower:    isFollower,
		TierIDs:       tierIDs,
	}, nil
}

func ParsePatreonWebhookMemberEvent(payload []byte) (*PatreonWebhookMemberEvent, error) {
	type relData struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	type rawPayload struct {
		Data struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Attributes struct {
				PatronStatus string `json:"patron_status"`
				IsFollower   bool   `json:"is_follower"`
			} `json:"attributes"`
			Relationships struct {
				User struct {
					Data relData `json:"data"`
				} `json:"user"`
				CurrentlyEntitledTiers struct {
					Data []relData `json:"data"`
				} `json:"currently_entitled_tiers"`
			} `json:"relationships"`
		} `json:"data"`
		Included []struct {
			ID            string `json:"id"`
			Type          string `json:"type"`
			Relationships struct {
				CurrentlyEntitledTiers struct {
					Data []relData `json:"data"`
				} `json:"currently_entitled_tiers"`
			} `json:"relationships"`
		} `json:"included"`
	}

	var raw rawPayload
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}

	if raw.Data.Type != "" && raw.Data.Type != "member" {
		return nil, fmt.Errorf("unsupported patreon webhook data type: %s", raw.Data.Type)
	}

	out := &PatreonWebhookMemberEvent{
		MemberID:      strings.TrimSpace(raw.Data.ID),
		PatreonUserID: strings.TrimSpace(raw.Data.Relationships.User.Data.ID),
		PatronStatus:  strings.TrimSpace(raw.Data.Attributes.PatronStatus),
		IsFollower:    raw.Data.Attributes.IsFollower,
	}
	for _, td := range raw.Data.Relationships.CurrentlyEntitledTiers.Data {
		if tid := strings.TrimSpace(td.ID); tid != "" {
			out.TierIDs = append(out.TierIDs, tid)
		}
	}

	// Fallback: some payload variants expose tiers only via included.member.
	if len(out.TierIDs) == 0 && out.MemberID != "" {
		for _, inc := range raw.Included {
			if inc.Type != "member" || strings.TrimSpace(inc.ID) != out.MemberID {
				continue
			}
			for _, td := range inc.Relationships.CurrentlyEntitledTiers.Data {
				if tid := strings.TrimSpace(td.ID); tid != "" {
					out.TierIDs = append(out.TierIDs, tid)
				}
			}
			break
		}
	}

	if out.MemberID == "" {
		return nil, errors.New("patreon webhook payload missing member id")
	}
	if out.PatreonUserID == "" {
		return nil, errors.New("patreon webhook payload missing user id")
	}
	return out, nil
}

func PatreonStatusToBillingStatus(patronStatus string) string {
	return PatreonMembershipToBillingStatus(patronStatus, false)
}

func PatreonMembershipToBillingStatus(patronStatus string, isFollower bool) string {
	switch strings.ToLower(strings.TrimSpace(patronStatus)) {
	case "active_patron":
		return models.BillingStatusActive
	case "active_member", "free_member":
		return models.BillingStatusActive
	case "declined_patron":
		return models.BillingStatusPastDue
	case "former_patron":
		return models.BillingStatusCanceled
	case "":
		if !isFollower {
			// Free memberships can have empty patron_status but are still active memberships.
			return models.BillingStatusActive
		}
		return models.BillingStatusIncomplete
	default:
		return models.BillingStatusIncomplete
	}
}

package okta

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/okta/errormap"
	"github.com/tedilabs/ota/internal/okta/ratelimit"
)

// Config configures a Client.
type Config struct {
	OrgURL     string
	APIToken   string
	UserAgent  string
	HTTPClient *http.Client // injectable (tests supply httptest.Server-bound client)
}

// Client is the outbound Okta adapter. It issues authenticated HTTP calls,
// observes rate-limit headers, retries 429s honoring Retry-After, and maps
// Okta error responses to domain errors.
//
// MVP uses a direct net/http implementation. The thin-wrapper contract in
// docs/TECH_STACK.md §4.1 reserves the option to swap okta-sdk-golang/v5
// into this layer without exposing SDK types beyond the package.
type Client struct {
	baseURL string
	token   secretToken
	ua      string
	http    *http.Client
	monitor *ratelimit.Monitor
	log     *slog.Logger
	clock   clock.Clock
	// maxRetries bounds automatic 429 retries per request (REQ-E01 AC-2).
	maxRetries int
}

// secretToken is a string newtype whose String/Format methods always render
// "***", so panics, %v / %+v / fmt.Errorf wrappers cannot leak the API token
// (REQ-C05 AC-3). The raw value is read only via secretToken.reveal() for
// the Authorization header.
type secretToken string

func (secretToken) String() string                          { return "***" }
func (secretToken) GoString() string                        { return "***" }
func (s secretToken) Format(f fmt.State, _ rune)            { _, _ = f.Write([]byte("***")) }
func (s secretToken) reveal() string                        { return string(s) }

// Option configures a Client.
type Option func(*clientOptions)

type clientOptions struct {
	Logger     *slog.Logger
	Clock      clock.Clock
	Monitor    *ratelimit.Monitor
	MaxRetries int
}

// WithLogger injects a structured logger.
func WithLogger(l *slog.Logger) Option { return func(o *clientOptions) { o.Logger = l } }

// WithClock injects a Clock.
func WithClock(c clock.Clock) Option { return func(o *clientOptions) { o.Clock = c } }

// WithMonitor lets tests inject a shared Monitor.
func WithMonitor(m *ratelimit.Monitor) Option { return func(o *clientOptions) { o.Monitor = m } }

// WithMaxRetries overrides the automatic 429 retry budget (default 3 per REQ-E01 AC-2).
func WithMaxRetries(n int) Option { return func(o *clientOptions) { o.MaxRetries = n } }

// NewClient constructs a Client.
func NewClient(ctx context.Context, cfg Config, opts ...Option) (*Client, error) {
	if cfg.OrgURL == "" {
		return nil, errors.New("okta: OrgURL is required")
	}
	if cfg.APIToken == "" {
		return nil, errors.New("okta: APIToken is required")
	}

	o := clientOptions{
		Logger:     slog.Default(),
		Clock:      clock.Real(),
		MaxRetries: 3,
	}
	for _, fn := range opts {
		fn(&o)
	}
	if o.Monitor == nil {
		o.Monitor = ratelimit.NewMonitor(o.Clock)
	}

	httpCli := cfg.HTTPClient
	if httpCli == nil {
		httpCli = http.DefaultClient
	}
	ua := cfg.UserAgent
	if ua == "" {
		ua = "ota"
	}

	return &Client{
		baseURL:    strings.TrimRight(cfg.OrgURL, "/"),
		token:      secretToken(cfg.APIToken),
		ua:         ua,
		http:       httpCli,
		monitor:    o.Monitor,
		log:        o.Logger,
		clock:      o.Clock,
		maxRetries: o.MaxRetries,
	}, nil
}

// RateLimitMonitor returns the adapter's Monitor (implements domain.RateLimitPort).
func (c *Client) RateLimitMonitor() *ratelimit.Monitor { return c.monitor }

// Users returns a UsersAdapter bound to this client.
func (c *Client) Users() *UsersAdapter { return &UsersAdapter{client: c} }

// Groups returns a GroupsAdapter bound to this client.
func (c *Client) Groups() *GroupsAdapter { return &GroupsAdapter{client: c} }

// GroupRules returns a GroupRulesAdapter bound to this client.
func (c *Client) GroupRules() *GroupRulesAdapter { return &GroupRulesAdapter{client: c} }

// Policies returns a PoliciesAdapter bound to this client.
func (c *Client) Policies() *PoliciesAdapter { return &PoliciesAdapter{client: c} }

// Logs returns a LogsAdapter bound to this client.
func (c *Client) Logs() *LogsAdapter { return &LogsAdapter{client: c} }

// doGet performs a GET, applying auth, rate-limit observation, and 429 retry.
// The caller owns the returned response body and MUST close it.
func (c *Client) doGet(ctx context.Context, urlStr string) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
		if err != nil {
			return nil, fmt.Errorf("okta: build request: %w", err)
		}
		req.Header.Set("Authorization", "SSWS "+c.token.reveal())
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.ua)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("okta: %w", errors.Join(domain.ErrNetwork, err))
		}

		c.monitor.Observe(resp)

		if resp.StatusCode == http.StatusTooManyRequests {
			rle := errormap.FromResponse(resp) // consumes body
			if attempt < c.maxRetries {
				var detail *domain.RateLimitedError
				if errors.As(rle, &detail) && detail.RetryAfter > 0 {
					if err := c.sleepRespectingCtx(ctx, detail.RetryAfter); err != nil {
						return nil, err
					}
				}
				lastErr = rle
				continue
			}
			return nil, rle
		}

		if resp.StatusCode >= 400 {
			return nil, errormap.FromResponse(resp)
		}
		return resp, nil
	}
	return nil, lastErr
}

// doPost performs a POST with a JSON body (body == nil → empty body).
// Same auth, rate-limit observation, and 429 retry semantics as doGet.
// Caller owns the returned response body and MUST close it.
func (c *Client) doPost(ctx context.Context, urlStr string, body []byte) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		var rdr io.Reader
		if len(body) > 0 {
			rdr = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, rdr)
		if err != nil {
			return nil, fmt.Errorf("okta: build request: %w", err)
		}
		req.Header.Set("Authorization", "SSWS "+c.token.reveal())
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.ua)
		if len(body) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("okta: %w", errors.Join(domain.ErrNetwork, err))
		}

		c.monitor.Observe(resp)

		if resp.StatusCode == http.StatusTooManyRequests {
			rle := errormap.FromResponse(resp)
			if attempt < c.maxRetries {
				var detail *domain.RateLimitedError
				if errors.As(rle, &detail) && detail.RetryAfter > 0 {
					if err := c.sleepRespectingCtx(ctx, detail.RetryAfter); err != nil {
						return nil, err
					}
				}
				lastErr = rle
				continue
			}
			return nil, rle
		}

		if resp.StatusCode >= 400 {
			return nil, errormap.FromResponse(resp)
		}
		return resp, nil
	}
	return nil, lastErr
}

// buildURL joins the configured base URL with a path + raw query.
func (c *Client) buildURL(pathAndQuery string) string {
	if strings.HasPrefix(pathAndQuery, "http://") || strings.HasPrefix(pathAndQuery, "https://") {
		return pathAndQuery
	}
	if !strings.HasPrefix(pathAndQuery, "/") {
		pathAndQuery = "/" + pathAndQuery
	}
	return c.baseURL + pathAndQuery
}

// rewriteAbsoluteURL adjusts a Link-header "next" URL to point at this
// client's baseURL (so fixtures stamped with dev-example.okta.com still route
// to the httptest.Server). Preserves path + query verbatim.
func (c *Client) rewriteAbsoluteURL(absURL string) string {
	u, err := url.Parse(absURL)
	if err != nil || (u.Scheme == "" && u.Host == "") {
		return absURL
	}
	q := ""
	if u.RawQuery != "" {
		q = "?" + u.RawQuery
	}
	return c.baseURL + u.Path + q
}

// drainAndClose reads and discards the rest of a response body.
func drainAndClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// sleepRespectingCtx waits d, honoring ctx cancellation. Uses the injected
// clock so tests can supply FakeClock + Advance; falls back to a stdlib
// time.After for clocks (like FakeClock) whose timers require explicit
// Advance — this keeps production deterministic (Clock.NewTimer) while
// keeping basic 429-retry tests runnable without plumbing Advance into every
// scenario driver.
func (c *Client) sleepRespectingCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := c.clock.NewTimer(d)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C():
		return nil
	case <-time.After(d):
		// Safety fallback: if the injected clock never fires (e.g., FakeClock
		// without Advance), do not hang the process.
		timer.Stop()
		return nil
	}
}

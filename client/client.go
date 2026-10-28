package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	// DefaultBaseURL is Torn API v2.
	DefaultBaseURL = "https://api.torn.com/v2"

	// DefaultUserAgent identifies this client. Torn asks callers to set a
	// custom User-Agent because CloudFlare may block generic ones.
	DefaultUserAgent = "gitlab.com/torn-city/go-torn-client"

	// DefaultRequestsPerMinute matches common Torn key budgets.
	DefaultRequestsPerMinute = 100

	defaultHTTPTimeout = 15 * time.Second
)

// API is the typed Torn API v2 client. Endpoint methods are promoted from the
// embedded generated client (for example GetMyBarsWithResponse).
type API struct {
	*ClientWithResponses
}

// ConfigOption configures New. Named distinctly from the generated ClientOption helpers.
type ConfigOption func(*config) error

type config struct {
	baseURL           string
	httpClient        *http.Client
	userAgent         string
	comment           string
	requestsPerMinute float64
	disableLimiter    bool
}

// WithServerURL overrides the API base URL (default DefaultBaseURL).
func WithServerURL(baseURL string) ConfigOption {
	return func(o *config) error {
		o.baseURL = strings.TrimRight(baseURL, "/")
		return nil
	}
}

// WithHTTP supplies a custom *http.Client. Timeout defaults to 15s when the
// provided client has no timeout and is otherwise left unchanged.
func WithHTTP(httpClient *http.Client) ConfigOption {
	return func(o *config) error {
		o.httpClient = httpClient
		return nil
	}
}

// WithUserAgent sets the User-Agent request header.
func WithUserAgent(userAgent string) ConfigOption {
	return func(o *config) error {
		o.userAgent = userAgent
		return nil
	}
}

// WithComment sets the Torn comment query parameter used to identify your
// application in Torn's API logs. Empty disables the automatic comment.
func WithComment(comment string) ConfigOption {
	return func(o *config) error {
		o.comment = comment
		return nil
	}
}

// WithRequestsPerMinute configures client-side throttling. Pass 0 or a
// negative value to disable throttling.
func WithRequestsPerMinute(n float64) ConfigOption {
	return func(o *config) error {
		if n <= 0 {
			o.disableLimiter = true
			return nil
		}
		o.requestsPerMinute = n
		o.disableLimiter = false
		return nil
	}
}

// New builds an API client that authenticates with apiKey using the
// Authorization ApiKey header required by Torn API v2.
func New(apiKey string, opts ...ConfigOption) (*API, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("torn: api key is required")
	}

	cfg := config{
		baseURL:           DefaultBaseURL,
		userAgent:         DefaultUserAgent,
		requestsPerMinute: DefaultRequestsPerMinute,
	}
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	httpClient := cfg.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	} else if httpClient.Timeout == 0 {
		cloned := *httpClient
		cloned.Timeout = defaultHTTPTimeout
		httpClient = &cloned
	}

	doer := &apiDoer{
		client:    httpClient,
		apiKey:    apiKey,
		userAgent: cfg.userAgent,
		comment:   cfg.comment,
	}
	if !cfg.disableLimiter {
		// Burst of 1 keeps spacing close to the sustained rate.
		doer.limiter = rate.NewLimiter(rate.Limit(cfg.requestsPerMinute/60.0), 1)
	}

	generated, err := NewClientWithResponses(
		cfg.baseURL,
		WithHTTPClient(doer),
	)
	if err != nil {
		return nil, err
	}
	return &API{ClientWithResponses: generated}, nil
}

type apiDoer struct {
	client    *http.Client
	apiKey    string
	userAgent string
	comment   string
	limiter   *rate.Limiter
}

func (d *apiDoer) Do(req *http.Request) (*http.Response, error) {
	if d.limiter != nil {
		if err := d.limiter.Wait(req.Context()); err != nil {
			return nil, err
		}
	}

	cloned := req.Clone(req.Context())
	if d.userAgent != "" {
		cloned.Header.Set("User-Agent", d.userAgent)
	}
	cloned.Header.Set("Accept", "application/json")
	// Torn OpenAPI security scheme: Authorization: ApiKey <key>
	cloned.Header.Set("Authorization", "ApiKey "+d.apiKey)

	if d.comment != "" {
		query := cloned.URL.Query()
		if query.Get("comment") == "" {
			query.Set("comment", d.comment)
			cloned.URL.RawQuery = query.Encode()
		}
	}

	resp, err := d.client.Do(cloned)
	if err != nil {
		return nil, err
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	closeErr := resp.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	if apiErr := checkBody(body, resp.StatusCode); apiErr != nil {
		return nil, apiErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Body:       append([]byte(nil), body...),
		}
	}
	return resp, nil
}

// HTTPError is a non-2xx HTTP response that did not carry a Torn error object.
type HTTPError struct {
	StatusCode int
	Body       []byte
}

func (e *HTTPError) Error() string {
	if len(e.Body) == 0 {
		return fmt.Sprintf("torn http %d", e.StatusCode)
	}
	msg := string(e.Body)
	if len(msg) > 200 {
		msg = msg[:200] + "..."
	}
	return fmt.Sprintf("torn http %d: %s", e.StatusCode, msg)
}

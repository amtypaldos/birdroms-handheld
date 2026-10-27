// Package pair implements the device side of QR / phone-approve pairing:
// start a pairing session, then poll until the server hands back a device
// token, which is persisted with config.Save. It is pure Go (no SDL, no cgo).
//
// The pairing endpoints are unauthenticated, so none of these requests carry
// an Authorization header, and they are deliberately not part of api.Client.
package pair

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

	"github.com/amtypaldos/birdroms-handheld/internal/config"
)

const (
	pollInterval       = time.Second     // server allows 120/min per client
	rateLimitBackoff   = 2 * time.Second // after a 429, same code
	transportBackoff   = 2 * time.Second // after a transient failure
	maxTransportFails  = 3               // consecutive, then give up
	maxBodyBytes       = 64 << 10        // pairing replies are tiny
	startPath          = "/api/devices/pair/start"
	pollPath           = "/api/devices/pair/poll"
	maxRestartsOnError = 1 // one fresh code after expiry, not a loop
)

// ErrExpired is returned by Loop when a pairing code expired (or was already
// consumed) and the single fresh code also expired.
var ErrExpired = errors.New("pair: pairing code expired")

// StatusError is a non-success HTTP status from a pairing endpoint that Loop
// does not treat as a normal poll outcome. It never carries a response body.
type StatusError struct {
	Path string
	Code int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("pair: %s: unexpected status %d %s", e.Path, e.Code, http.StatusText(e.Code))
}

// transient reports whether retrying might help (server-side trouble).
func (e *StatusError) transient() bool { return e.Code >= 500 }

// Session is one pairing attempt as issued by the server.
type Session struct {
	Code      string    // short code shown to the user
	QRURL     string    // absolute URL the phone opens; this is what the QR encodes
	ExpiresAt time.Time // server expiry (the server TTL is 5 minutes)
}

// Status is the outcome of a single poll.
type Status int

const (
	StatusPending     Status = iota // 202: keep polling the same code
	StatusApproved                  // 200: Token is set
	StatusExpired                   // 410: expired or already consumed
	StatusRateLimited               // 429: same code, back off longer
)

// PollResult is the outcome of PollOnce. Token is non-empty only when Status
// is StatusApproved.
type PollResult struct {
	Status Status
	Token  string
}

func trimBase(base string) string { return strings.TrimRight(base, "/") }

// Start begins a pairing session: POST {base}/api/devices/pair/start.
func Start(ctx context.Context, hc *http.Client, baseURL string) (Session, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, trimBase(baseURL)+startPath, nil)
	if err != nil {
		return Session{}, fmt.Errorf("pair: build start request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return Session{}, fmt.Errorf("pair: start: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return Session{}, &StatusError{Path: startPath, Code: resp.StatusCode}
	}
	var raw struct {
		Code      string `json:"code"`
		QRURL     string `json:"qr_url"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&raw); err != nil {
		return Session{}, errors.New("pair: start: malformed response")
	}
	if raw.Code == "" || raw.QRURL == "" {
		return Session{}, errors.New("pair: start: response missing code or qr_url")
	}
	exp, err := time.Parse(time.RFC3339Nano, raw.ExpiresAt) // also accepts plain RFC3339
	if err != nil {
		return Session{}, fmt.Errorf("pair: start: bad expires_at: %w", err)
	}
	return Session{Code: raw.Code, QRURL: raw.QRURL, ExpiresAt: exp}, nil
}

// PollOnce checks the approval state of code once. 202/410/429 are normal
// outcomes reported through PollResult.Status; their bodies are never read, so
// no token can come from anything but a 200. 404/400 and other statuses are
// returned as *StatusError.
func PollOnce(ctx context.Context, hc *http.Client, baseURL, code string) (PollResult, error) {
	u := trimBase(baseURL) + pollPath + "?" + url.Values{"code": {code}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return PollResult{}, fmt.Errorf("pair: build poll request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return PollResult{}, fmt.Errorf("pair: poll: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusAccepted:
		return PollResult{Status: StatusPending}, nil
	case http.StatusGone:
		return PollResult{Status: StatusExpired}, nil
	case http.StatusTooManyRequests:
		return PollResult{Status: StatusRateLimited}, nil
	case http.StatusOK:
		var raw struct {
			DeviceToken string `json:"device_token"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&raw); err != nil {
			// Do not include the decode error or body: it is the token response.
			return PollResult{}, errors.New("pair: poll: malformed approval response")
		}
		if raw.DeviceToken == "" {
			return PollResult{}, errors.New("pair: poll: approval response missing device_token")
		}
		return PollResult{Status: StatusApproved, Token: raw.DeviceToken}, nil
	default:
		return PollResult{}, &StatusError{Path: pollPath, Code: resp.StatusCode}
	}
}

// Options configures Loop.
type Options struct {
	BaseURL    string              // server URL; a trailing slash is trimmed
	ConfigPath string              // where the paired config is saved
	Client     *http.Client        // required
	Sleep      func(time.Duration) // defaults to a context-aware sleep
	Now        func() time.Time    // defaults to time.Now
	OnSession  func(Session)       // called for each new session (initial and the one retry)
}

// Loop runs the pairing state machine: Start, show (via OnSession), poll
// until approved, then persist {ServerURL, DeviceToken} with config.Save and
// return it. A 410 or local time past expires_at restarts once; a second
// expiry returns ErrExpired. 429 backs off on the same code. 404/400 and other
// client errors stop. Three consecutive transient failures return the last
// error without restarting. Cancelling ctx stops with ctx.Err().
func Loop(ctx context.Context, o Options) (config.Config, error) {
	base := trimBase(o.BaseURL)
	now := o.Now
	if now == nil {
		now = time.Now
	}
	sleep := o.Sleep
	if sleep == nil {
		sleep = func(d time.Duration) {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
			case <-t.C:
			}
		}
	}

	var (
		sess     Session
		have     bool
		restarts int
		fails    int
	)
	// fail records a transient failure; it returns a non-nil error when the
	// consecutive budget is spent.
	fail := func(err error) error {
		fails++
		if fails >= maxTransportFails {
			return err
		}
		sleep(transportBackoff)
		return nil
	}

	for {
		if err := ctx.Err(); err != nil {
			return config.Config{}, err
		}

		if !have {
			s, err := Start(ctx, o.Client, base)
			if err != nil {
				if !retryable(err) {
					return config.Config{}, err
				}
				if ferr := fail(err); ferr != nil {
					return config.Config{}, ferr
				}
				continue
			}
			fails = 0
			sess, have = s, true
			if o.OnSession != nil {
				o.OnSession(sess)
			}
		}

		if !now().Before(sess.ExpiresAt) {
			if restarts >= maxRestartsOnError {
				return config.Config{}, ErrExpired
			}
			restarts++
			have = false
			continue
		}

		res, err := PollOnce(ctx, o.Client, base, sess.Code)
		if err != nil {
			if !retryable(err) {
				return config.Config{}, err
			}
			if ferr := fail(err); ferr != nil {
				return config.Config{}, ferr
			}
			continue
		}
		fails = 0

		switch res.Status {
		case StatusApproved:
			cfg := config.Config{ServerURL: base, DeviceToken: res.Token}
			if err := config.Save(o.ConfigPath, cfg); err != nil {
				return config.Config{}, fmt.Errorf("pair: save config: %w", err)
			}
			return cfg, nil
		case StatusExpired:
			if restarts >= maxRestartsOnError {
				return config.Config{}, ErrExpired
			}
			restarts++
			have = false
		case StatusRateLimited:
			sleep(rateLimitBackoff)
		default: // StatusPending
			sleep(pollInterval)
		}
	}
}

// retryable reports whether err is a transport error or a 5xx, i.e. worth
// another attempt. Context cancellation and 4xx statuses are not.
func retryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var se *StatusError
	if errors.As(err, &se) {
		return se.transient()
	}
	// Malformed-response errors are plain errors.New values; treat only
	// wrapped transport errors (from hc.Do) as retryable.
	var ue *url.Error
	return errors.As(err, &ue)
}

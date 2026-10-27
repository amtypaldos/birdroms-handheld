package pair

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"

	"github.com/amtypaldos/birdroms-handheld/internal/config"
)

const fakeToken = "fake-device-token-abc123"

// fakeServer scripts the pairing endpoints. Each Start returns the next code
// in codes; each poll for a code pops the next scripted response for it.
type fakeServer struct {
	mu        sync.Mutex
	t         *testing.T
	codes     []string
	starts    int
	polls     map[string][]pollReply // code -> remaining scripted replies
	pollCount map[string]int
	expiresIn time.Duration
	authSeen  []string
	srv       *httptest.Server
}

type pollReply struct {
	status int
	body   string
}

func newFakeServer(t *testing.T, codes []string, polls map[string][]pollReply) *fakeServer {
	t.Helper()
	f := &fakeServer{t: t, codes: codes, polls: polls, pollCount: map[string]int{}, expiresIn: 5 * time.Minute}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeServer) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.authSeen = append(f.authSeen, r.Header.Get("Authorization"))
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/devices/pair/start":
		code := f.codes[f.starts]
		f.starts++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"code":       code,
			"qr_url":     f.srv.URL + "/pair/approve/" + code,
			"expires_at": time.Now().Add(f.expiresIn).UTC().Format(time.RFC3339Nano),
		})
	case r.Method == http.MethodGet && r.URL.Path == "/api/devices/pair/poll":
		code := r.URL.Query().Get("code")
		f.pollCount[code]++
		q := f.polls[code]
		if len(q) == 0 {
			f.t.Errorf("unexpected extra poll for code %q", code)
			http.Error(w, "unexpected", http.StatusInternalServerError)
			return
		}
		reply := q[0]
		f.polls[code] = q[1:]
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(reply.status)
		_, _ = w.Write([]byte(reply.body))
	default:
		http.NotFound(w, r)
	}
}

type recorder struct {
	delays []time.Duration
}

func (r *recorder) sleep(d time.Duration) { r.delays = append(r.delays, d) }

func (f *fakeServer) opts(t *testing.T, rec *recorder) (Options, string) {
	path := filepath.Join(t.TempDir(), "config.json")
	return Options{
		BaseURL:    f.srv.URL + "/", // trailing slash must be trimmed
		ConfigPath: path,
		Client:     f.srv.Client(),
		Sleep:      rec.sleep,
	}, path
}

func pending() pollReply { return pollReply{http.StatusAccepted, `{"status":"pending"}`} }
func approved() pollReply {
	return pollReply{http.StatusOK, `{"device_token":"` + fakeToken + `","user":{"username":"friend"}}`}
}
func gone() pollReply { return pollReply{http.StatusGone, `{"detail":"expired"}`} }

func assertNoConfig(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("config file must not exist, stat err = %v", err)
	}
}

func TestLoopPendingThenToken(t *testing.T) {
	f := newFakeServer(t, []string{"ABCD-1234"}, map[string][]pollReply{
		"ABCD-1234": {pending(), pending(), approved()},
	})
	rec := &recorder{}
	opts, path := f.opts(t, rec)
	var seen []Session
	opts.OnSession = func(s Session) { seen = append(seen, s) }

	cfg, err := Loop(context.Background(), opts)
	if err != nil {
		t.Fatalf("Loop: %v", err)
	}
	if !cfg.Paired() || cfg.DeviceToken != fakeToken || cfg.ServerURL != f.srv.URL {
		t.Fatalf("cfg = %+v", cfg)
	}
	saved, err := config.Load(path)
	if err != nil || saved != cfg {
		t.Fatalf("saved = %+v, err = %v, want %+v", saved, err, cfg)
	}
	if len(seen) != 1 || seen[0].Code != "ABCD-1234" {
		t.Fatalf("OnSession calls = %+v", seen)
	}
	want := []time.Duration{time.Second, time.Second}
	if len(rec.delays) != 2 || rec.delays[0] != want[0] || rec.delays[1] != want[1] {
		t.Fatalf("sleeps = %v, want %v", rec.delays, want)
	}
	if f.starts != 1 {
		t.Fatalf("starts = %d, want 1", f.starts)
	}
	for _, a := range f.authSeen {
		if a != "" {
			t.Fatalf("pairing request carried Authorization %q", a)
		}
	}
}

func TestLoopExpiryRestartsOnceThenSucceeds(t *testing.T) {
	f := newFakeServer(t, []string{"OLD-CODE", "NEW-CODE"}, map[string][]pollReply{
		"OLD-CODE": {pending(), gone()},
		"NEW-CODE": {approved()},
	})
	rec := &recorder{}
	opts, path := f.opts(t, rec)
	var codes []string
	opts.OnSession = func(s Session) { codes = append(codes, s.Code) }

	cfg, err := Loop(context.Background(), opts)
	if err != nil {
		t.Fatalf("Loop: %v", err)
	}
	if cfg.DeviceToken != fakeToken {
		t.Fatalf("cfg = %+v", cfg)
	}
	if f.starts != 2 || len(codes) != 2 || codes[1] != "NEW-CODE" {
		t.Fatalf("starts = %d codes = %v", f.starts, codes)
	}
	if got, err := config.Load(path); err != nil || got.DeviceToken != fakeToken {
		t.Fatalf("saved = %+v err = %v", got, err)
	}
}

func TestLoopSecondExpiryReturnsErrExpiredNoToken(t *testing.T) {
	f := newFakeServer(t, []string{"C1", "C2"}, map[string][]pollReply{
		"C1": {gone()},
		"C2": {gone()},
	})
	opts, path := f.opts(t, &recorder{})
	_, err := Loop(context.Background(), opts)
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("err = %v, want ErrExpired", err)
	}
	if f.starts != 2 {
		t.Fatalf("starts = %d, want exactly 2 (one retry)", f.starts)
	}
	assertNoConfig(t, path)
}

func TestLoopLocalExpiryRestartsWithoutPolling(t *testing.T) {
	f := newFakeServer(t, []string{"C1", "C2"}, map[string][]pollReply{
		"C2": {approved()},
	})
	opts, _ := f.opts(t, &recorder{})
	// Clock far in the future: C1's expires_at is already past locally.
	// The same clock applies to C2, so give C2 a fresh window by moving the
	// clock back after the first Start.
	base := time.Now()
	calls := 0
	opts.Now = func() time.Time {
		calls++
		if calls <= 1 { // first expiry check, on C1
			return base.Add(time.Hour)
		}
		return base
	}
	cfg, err := Loop(context.Background(), opts)
	if err != nil {
		t.Fatalf("Loop: %v", err)
	}
	if cfg.DeviceToken != fakeToken {
		t.Fatalf("cfg = %+v", cfg)
	}
	if f.pollCount["C1"] != 0 {
		t.Fatalf("polled locally-expired code %d times", f.pollCount["C1"])
	}
	if f.starts != 2 {
		t.Fatalf("starts = %d, want 2", f.starts)
	}
}

func TestLoopRateLimitedBacksOffSameCode(t *testing.T) {
	f := newFakeServer(t, []string{"ONLY"}, map[string][]pollReply{
		"ONLY": {{http.StatusTooManyRequests, `{"detail":"throttled"}`}, approved()},
	})
	rec := &recorder{}
	opts, _ := f.opts(t, rec)
	if _, err := Loop(context.Background(), opts); err != nil {
		t.Fatalf("Loop: %v", err)
	}
	if f.starts != 1 {
		t.Fatalf("429 must not start a new code; starts = %d", f.starts)
	}
	if len(rec.delays) != 1 || rec.delays[0] != 2*time.Second {
		t.Fatalf("sleeps = %v, want [2s]", rec.delays)
	}
}

func TestLoopNotFoundAndBadRequestStopWithError(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusBadRequest} {
		f := newFakeServer(t, []string{"X"}, map[string][]pollReply{
			"X": {{status, `{"detail":"nope"}`}},
		})
		opts, path := f.opts(t, &recorder{})
		_, err := Loop(context.Background(), opts)
		if err == nil || errors.Is(err, ErrExpired) {
			t.Fatalf("status %d: err = %v, want non-expiry error", status, err)
		}
		if f.pollCount["X"] != 1 || f.starts != 1 {
			t.Fatalf("status %d: polls=%d starts=%d, want no looping", status, f.pollCount["X"], f.starts)
		}
		assertNoConfig(t, path)
	}
}

func TestLoopThreeTransportFailuresReturnError(t *testing.T) {
	f := newFakeServer(t, []string{"X"}, map[string][]pollReply{"X": {}})
	opts, path := f.opts(t, &recorder{})
	// Start succeeds, then the server goes away.
	opts.OnSession = func(Session) { f.srv.CloseClientConnections(); f.srv.Close() }
	_, err := Loop(context.Background(), opts)
	if err == nil {
		t.Fatal("Loop returned nil error with dead server")
	}
	if errors.Is(err, ErrExpired) {
		t.Fatalf("transport error misreported as expiry: %v", err)
	}
	if f.starts != 1 {
		t.Fatalf("transport failure must not call Start again; starts = %d", f.starts)
	}
	assertNoConfig(t, path)
}

func TestLoopTransportFailureCounterResetsOnSuccess(t *testing.T) {
	// fail, fail, pending (resets), fail, fail, approved => success.
	var mu sync.Mutex
	n := 0
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/api/devices/pair/start", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"code": "K", "qr_url": srv.URL + "/q",
			"expires_at": time.Now().Add(time.Minute).UTC().Format(time.RFC3339),
		})
	})
	mux.HandleFunc("/api/devices/pair/poll", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n++
		step := n
		mu.Unlock()
		switch step {
		case 1, 2, 4, 5:
			w.WriteHeader(http.StatusBadGateway)
		case 3:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"pending"}`))
		default:
			_, _ = w.Write([]byte(`{"device_token":"` + fakeToken + `"}`))
		}
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	path := filepath.Join(t.TempDir(), "c.json")
	cfg, err := Loop(context.Background(), Options{BaseURL: srv.URL, ConfigPath: path, Client: srv.Client(), Sleep: func(time.Duration) {}})
	if err != nil {
		t.Fatalf("Loop: %v", err)
	}
	if cfg.DeviceToken != fakeToken {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestLoopContextCancelStopsWithoutToken(t *testing.T) {
	f := newFakeServer(t, []string{"X"}, map[string][]pollReply{"X": {pending(), pending(), pending()}})
	ctx, cancel := context.WithCancel(context.Background())
	opts, path := f.opts(t, &recorder{})
	opts.Sleep = func(time.Duration) { cancel() }
	_, err := Loop(ctx, opts)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	assertNoConfig(t, path)
}

func TestNonApprovedResponsesNeverYieldToken(t *testing.T) {
	// Even if a buggy/hostile server put a token field in a pending or gone
	// body, only a 200 may produce a token.
	for _, status := range []int{http.StatusAccepted, http.StatusGone, http.StatusTooManyRequests} {
		f := newFakeServer(t, []string{"X"}, map[string][]pollReply{
			"X": {{status, `{"device_token":"` + fakeToken + `"}`}},
		})
		res, err := PollOnce(context.Background(), f.srv.Client(), f.srv.URL, "X")
		if err != nil {
			t.Fatalf("status %d: %v", status, err)
		}
		if res.Token != "" || res.Status == StatusApproved {
			t.Fatalf("status %d leaked token: %+v", status, res)
		}
	}
}

func TestApprovedWithoutTokenIsError(t *testing.T) {
	f := newFakeServer(t, []string{"X"}, map[string][]pollReply{"X": {{http.StatusOK, `{"user":{}}`}}})
	res, err := PollOnce(context.Background(), f.srv.Client(), f.srv.URL, "X")
	if err == nil || res.Token != "" {
		t.Fatalf("res = %+v err = %v, want error", res, err)
	}
}

func TestPollOnceEscapesCode(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query().Get("code")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"pending"}`))
	}))
	t.Cleanup(srv.Close)
	if _, err := PollOnce(context.Background(), srv.Client(), srv.URL, "a&b=c d"); err != nil {
		t.Fatal(err)
	}
	if got != "a&b=c d" {
		t.Fatalf("server saw code %q", got)
	}
}

func TestStartParsesSession(t *testing.T) {
	f := newFakeServer(t, []string{"S1"}, nil)
	s, err := Start(context.Background(), f.srv.Client(), f.srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	if s.Code != "S1" || s.QRURL != f.srv.URL+"/pair/approve/S1" {
		t.Fatalf("session = %+v", s)
	}
	if time.Until(s.ExpiresAt) < 4*time.Minute {
		t.Fatalf("ExpiresAt = %v", s.ExpiresAt)
	}
}

func TestStartRejectsIncompleteResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"code":"X"}`))
	}))
	t.Cleanup(srv.Close)
	if _, err := Start(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Fatal("Start accepted response without qr_url")
	}
}

func TestQRImageEncodesPayloadExactly(t *testing.T) {
	const payload = "https://roms.example.com/pair/approve/ABCD-1234?x=1&y=2"
	img, err := QRImage(payload)
	if err != nil {
		t.Fatal(err)
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		t.Fatal(err)
	}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, nil)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.GetText() != payload {
		t.Fatalf("decoded %q, want %q", res.GetText(), payload)
	}
}

func TestQRImageFromServerURLDecodesToQRURL(t *testing.T) {
	f := newFakeServer(t, []string{"ZZ-9"}, nil)
	s, err := Start(context.Background(), f.srv.Client(), f.srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	img, err := QRImage(s.QRURL)
	if err != nil {
		t.Fatal(err)
	}
	bmp, _ := gozxing.NewBinaryBitmapFromImage(img)
	res, err := qrcode.NewQRCodeReader().Decode(bmp, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.GetText() != s.QRURL {
		t.Fatalf("decoded %q, want server qr_url %q", res.GetText(), s.QRURL)
	}
}

func TestQRImageEmptyPayloadErrors(t *testing.T) {
	if _, err := QRImage(""); err == nil {
		t.Fatal("QRImage(\"\") = nil error")
	}
}

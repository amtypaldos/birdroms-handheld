// Package api is the read-only birdROMs device API client. Every request is a
// GET authenticated with "Authorization: Device <token>".
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ErrUnauthorized is returned (wrapped) when the server answers HTTP 401,
// i.e. the device token is revoked or unknown. Check with errors.Is.
var ErrUnauthorized = errors.New("api: unauthorized (device token revoked or unknown)")

// ErrRangeNotSatisfiable is returned (wrapped) when the server answers HTTP 416
// to a ranged download, e.g. the partial file is already as long as the ROM.
var ErrRangeNotSatisfiable = errors.New("api: range not satisfiable")

// ROM is a ROM in the device shape returned by the server. SizeBytes and
// CoverURL are nil when the server reports null.
type ROM struct {
	ID        int     `json:"id"`
	Title     string  `json:"title"`
	Platform  string  `json:"platform"`
	Extension string  `json:"extension"`
	SizeBytes *int64  `json:"size_bytes"`
	CoverURL  *string `json:"cover_url"`
}

// ROMPage is one page of the ROM listing. Next and Previous are the server's
// pagination links (nil when absent); the client never follows them itself.
type ROMPage struct {
	Count    int     `json:"count"`
	Next     *string `json:"next"`
	Previous *string `json:"previous"`
	Results  []ROM   `json:"results"`
}

// Playlist is a playlist summary.
type Playlist struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	CreatedBy int    `json:"created_by"` // user id, not a username
	CreatedAt string `json:"created_at"`
	ROMCount  int    `json:"rom_count"`
}

// PlaylistDetail is a playlist plus its ROMs in server order.
type PlaylistDetail struct {
	Playlist
	ROMs []ROM `json:"roms"`
}

// Client talks to one birdROMs server with one device token.
type Client struct {
	base  string
	token string
	hc    *http.Client
}

// New returns a Client for baseURL (a trailing slash is trimmed) using the
// given device token. hc may be nil, in which case a default http.Client with
// no overall timeout is used (ROM downloads can legitimately take minutes).
func New(baseURL, token string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{}
	}
	return &Client{base: strings.TrimRight(baseURL, "/"), token: token, hc: hc}
}

// ListPlatforms returns the platform names that have ROMs.
func (c *Client) ListPlatforms() ([]string, error) {
	var out []string
	if err := c.getJSON("/api/roms/platforms/", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListROMs returns one page (1-based) of ROMs in the device shape. An empty
// platform lists all platforms.
func (c *Client) ListROMs(platform string, page int) (ROMPage, error) {
	q := url.Values{}
	q.Set("fields", "device")
	q.Set("page", strconv.Itoa(page))
	if platform != "" {
		q.Set("platform", platform)
	}
	var out ROMPage
	if err := c.getJSON("/api/roms/", q, &out); err != nil {
		return ROMPage{}, err
	}
	return out, nil
}

// ListPlaylists returns all playlists (the endpoint is unpaginated).
func (c *Client) ListPlaylists() ([]Playlist, error) {
	var out []Playlist
	if err := c.getJSON("/api/playlists/", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetPlaylist returns playlist id with its ROMs in server order.
func (c *Client) GetPlaylist(id int) (PlaylistDetail, error) {
	var out PlaylistDetail
	if err := c.getJSON("/api/playlists/"+strconv.Itoa(id)+"/", nil, &out); err != nil {
		return PlaylistDetail{}, err
	}
	return out, nil
}

// MaxCoverBytes caps a cover download; a larger body is an error rather than
// being buffered whole on a memory-poor handheld.
const MaxCoverBytes = 4 << 20

// GetCover fetches the image at coverURL, which is absolute (as returned in
// ROM.CoverURL) and is requested as-is with the device token; the client base
// URL is never prefixed. Callers must not call it for a nil CoverURL.
func (c *Client) GetCover(coverURL string) ([]byte, error) {
	resp, err := c.getURL(coverURL, "cover", "image/*")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxCoverBytes+1))
	if err != nil {
		return nil, fmt.Errorf("api: read cover: %w", err)
	}
	if len(data) > MaxCoverBytes {
		return nil, fmt.Errorf("api: cover larger than %d bytes", MaxCoverBytes)
	}
	return data, nil
}

// DownloadROM streams ROM id into w. If progressCb is non-nil it is called
// after each chunk written with the cumulative number of bytes so far. On any
// non-2xx response nothing is written to w.
func (c *Client) DownloadROM(id int, w io.Writer, progressCb func(written int64)) error {
	return c.DownloadROMFrom(id, 0, w, progressCb)
}

// restartable is what a writer must offer so a 200 answer to a ranged request
// (the server ignored Range) can be written from the start. *os.File has it.
type restartable interface {
	io.Seeker
	Truncate(size int64) error
}

// DownloadROMFrom streams ROM id into w starting at byte offset, which is the
// number of bytes w already holds. offset 0 sends no Range header; offset > 0
// sends "Range: bytes=<offset>-". A 206 appends to w. A 200 means the server
// ignored the range: w must be restartable (an *os.File); it is truncated and
// the whole body is written from the start. progress, if non-nil, receives the
// cumulative size including the offset. On a non-2xx response nothing is
// written to w.
func (c *Client) DownloadROMFrom(id int, offset int64, w io.Writer, progress func(written int64)) error {
	path := "/api/roms/" + strconv.Itoa(id) + "/download/"
	resp, err := c.doGet(c.base+path, path, "application/json", offset) // same Accept as before B6
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	start := offset
	if offset > 0 && resp.StatusCode == http.StatusOK {
		rw, ok := w.(restartable)
		if !ok {
			return fmt.Errorf("api: download rom %d: server ignored Range and the writer cannot restart", id)
		}
		if err := rw.Truncate(0); err != nil {
			return fmt.Errorf("api: download rom %d: truncate: %w", id, err)
		}
		if _, err := rw.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("api: download rom %d: seek: %w", id, err)
		}
		start = 0
	}

	cw := &countingWriter{w: w, cb: progress, total: start}
	if _, err := io.Copy(cw, resp.Body); err != nil {
		return fmt.Errorf("api: download rom %d: %w", id, err)
	}
	return nil
}

// countingWriter forwards writes to w and reports the running total to cb.
type countingWriter struct {
	w     io.Writer
	cb    func(int64)
	total int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	if n > 0 {
		cw.total += int64(n)
		if cw.cb != nil {
			cw.cb(cw.total)
		}
	}
	return n, err
}

// getJSON GETs path and decodes the JSON body into out.
func (c *Client) getJSON(path string, q url.Values, out any) error {
	resp, err := c.get(path, q)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("api: decode %s: %w", path, err)
	}
	return nil
}

// get issues an authenticated GET and returns the response only for 2xx
// statuses. 401 maps to ErrUnauthorized; other statuses become an error that
// includes the code. The caller must close the body of a returned response.
func (c *Client) get(path string, q url.Values) (*http.Response, error) {
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return c.getURL(u, path, "application/json")
}

// getURL is get for a fully-formed URL. path names the request in errors
// (the path for API calls; never the URL, which may carry a signed query).
func (c *Client) getURL(u, path, accept string) (*http.Response, error) {
	return c.doGet(u, path, accept, 0)
}

// doGet is getURL with an optional start offset: rangeFrom > 0 adds
// "Range: bytes=<rangeFrom>-".
func (c *Client) doGet(u, path, accept string, rangeFrom int64) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("api: build request %s: %w", path, err)
	}
	req.Header.Set("Authorization", "Device "+c.token)
	req.Header.Set("Accept", accept)
	if rangeFrom > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(rangeFrom, 10)+"-")
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api: GET %s: %w", path, err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("GET %s: %w", path, ErrUnauthorized)
	}
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		return nil, fmt.Errorf("GET %s: status 416: %w", path, ErrRangeNotSatisfiable)
	}
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
	return nil, fmt.Errorf("api: GET %s: unexpected status %d %s: %s",
		path, resp.StatusCode, http.StatusText(resp.StatusCode), strings.TrimSpace(string(snippet)))
}

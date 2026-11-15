// Package download fetches a match replay from Valve's replay servers, using
// OpenDota to look up the replay cluster and salt.
package download

import (
	"bufio"
	"bytes"
	"compress/bzip2"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

// OpenDotaURL is the base of the OpenDota match endpoint (replaceable in tests).
var OpenDotaURL = "https://api.opendota.com/api/matches/"

// ReplayURL builds the Valve replay URL (replaceable in tests).
var ReplayURL = func(cluster int, matchID uint64, salt int64) string {
	return fmt.Sprintf("http://replay%d.valve.net/570/%d_%d.dem.bz2", cluster, matchID, salt)
}

// Errors with user-facing meaning.
var (
	ErrNotFound = errors.New("OpenDota does not know this match")
	ErrNoSalt   = errors.New("OpenDota has no replay info for this match yet: open it on opendota.com and request a parse, then retry")
	ErrExpired  = errors.New("Valve already deleted this replay (replays are kept for about two weeks)")
)

// Progress is reported while downloading.
type Progress struct {
	MatchID uint64 `json:"match_id"`
	Stage   string `json:"stage"` // lookup, download, done, failed
	Bytes   int64  `json:"bytes"`
	Total   int64  `json:"total"` // compressed size, 0 if unknown
	Path    string `json:"path"`
	Error   string `json:"error"`
}

// Client downloads replays.
type Client struct {
	HTTP     *http.Client
	Progress func(Progress) // optional
}

// New returns a client with sane timeouts (the download itself can take minutes).
func New(progress func(Progress)) *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Minute}, Progress: progress}
}

func (c *Client) report(p Progress) {
	if c.Progress != nil {
		c.Progress(p)
	}
}

// ParseMatchID accepts a match id or an OpenDota/Dotabuff/Stratz URL ending in one.
func ParseMatchID(s string) (uint64, error) {
	s = strings.TrimRight(strings.TrimSpace(s), "/")
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		s = s[i+1:]
	}
	id, err := strconv.ParseUint(s, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("not a match id: %q", s)
	}
	return id, nil
}

type matchInfo struct {
	MatchID    uint64 `json:"match_id"`
	Cluster    int    `json:"cluster"`
	ReplaySalt int64  `json:"replay_salt"`
}

// Lookup asks OpenDota for the replay cluster and salt.
func (c *Client) Lookup(ctx context.Context, matchID uint64) (cluster int, salt int64, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, OpenDotaURL+strconv.FormatUint(matchID, 10), nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("User-Agent", "drstats")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return 0, 0, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("OpenDota: HTTP %s", resp.Status)
	}
	var mi matchInfo
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&mi); err != nil {
		return 0, 0, fmt.Errorf("OpenDota: %w", err)
	}
	if mi.Cluster == 0 || mi.ReplaySalt == 0 {
		return 0, 0, ErrNoSalt
	}
	return mi.Cluster, mi.ReplaySalt, nil
}

// RequestURL is the OpenDota parse request endpoint (replaceable in tests).
var RequestURL = "https://api.opendota.com/api/request/"

// RequestParse asks OpenDota to parse a match, which also makes it fetch the
// replay salt; Lookup succeeds a few minutes later.
func (c *Client) RequestParse(ctx context.Context, matchID uint64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, RequestURL+strconv.FormatUint(matchID, 10), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "drstats")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OpenDota parse request: HTTP %s", resp.Status)
	}
	return nil
}

// Fetch downloads and decompresses the replay of matchID into dir/<id>.dem
// and returns the file path. An existing file is returned as is.
func (c *Client) Fetch(ctx context.Context, matchID uint64, dir string) (path string, err error) {
	p := Progress{MatchID: matchID, Stage: "lookup"}
	defer func() {
		if err != nil {
			p.Stage, p.Error = "failed", err.Error()
		} else {
			p.Stage, p.Path = "done", path
		}
		c.report(p)
	}()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path = filepath.Join(dir, strconv.FormatUint(matchID, 10)+".dem")
	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		return path, nil
	}
	c.report(p)
	cluster, salt, err := c.Lookup(ctx, matchID)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ReplayURL(cluster, matchID, salt), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "drstats")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden:
		return "", ErrExpired
	case resp.StatusCode != http.StatusOK:
		return "", fmt.Errorf("replay server: HTTP %s", resp.Status)
	}
	p.Stage, p.Total = "download", resp.ContentLength
	c.report(p)

	counted := &countingReader{r: resp.Body, every: 1 << 20, tick: func(n int64) {
		p.Bytes = n
		c.report(p)
	}}
	body, err := Decompress(counted)
	if err != nil {
		return "", err
	}
	defer body.Close()

	tmp, err := os.CreateTemp(dir, ".download-*.dem")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := io.Copy(tmp, body); err != nil {
		tmp.Close()
		return "", fmt.Errorf("download: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return path, os.Rename(tmp.Name(), path)
}

// Decompress detects the compression by magic bytes: Valve serves ".dem.bz2"
// files that are currently zstd; bzip2 and uncompressed replays also work.
func Decompress(r io.Reader) (io.ReadCloser, error) {
	br := bufio.NewReaderSize(r, 64<<10)
	magic, err := br.Peek(4)
	if err != nil {
		return nil, fmt.Errorf("read replay: %w", err)
	}
	switch {
	case bytes.Equal(magic, []byte{0x28, 0xb5, 0x2f, 0xfd}):
		zr, err := zstd.NewReader(br, zstd.WithDecoderMaxWindow(1<<31))
		if err != nil {
			return nil, err
		}
		return zr.IOReadCloser(), nil
	case bytes.HasPrefix(magic, []byte("BZh")):
		return io.NopCloser(bzip2.NewReader(br)), nil
	case bytes.HasPrefix(magic, []byte("PBDE")):
		return io.NopCloser(br), nil
	}
	return nil, fmt.Errorf("unknown replay format (magic % x)", magic)
}

type countingReader struct {
	r     io.Reader
	n     int64
	next  int64
	every int64
	tick  func(int64)
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.n >= c.next {
		c.next = c.n + c.every
		c.tick(c.n)
	}
	return n, err
}

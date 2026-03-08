package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is an HTTP client for the remote AMAROS node registry (R2 bucket).
//
// Bucket layout:
//
//	nodes/{name}/metadata.json        — NodeManifest (JSON)
//	nodes/{name}/readme.md            — Markdown readme
//	nodes/{name}/versions/{v}.tar.gz  — Version tarball
type Client struct {
	BaseURL string
	http    *http.Client
}

// NewClient creates a Client pointing at the given R2 public bucket URL.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// get performs a GET and returns the response body bytes.
func (c *Client) get(path string) ([]byte, error) {
	url := c.BaseURL + "/" + path
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("registry client: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("registry client: not found: %s", path)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry client: unexpected status %d for %s", resp.StatusCode, path)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("registry client: read body: %w", err)
	}
	return body, nil
}

// GetManifest fetches the manifest for a node by name.
func (c *Client) GetManifest(name string) (*NodeManifest, error) {
	data, err := c.get("nodes/" + name + "/metadata.json")
	if err != nil {
		return nil, err
	}
	var m NodeManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("registry client: invalid manifest for %q: %w", name, err)
	}
	return &m, nil
}

// GetReadme fetches the readme markdown for a node.
func (c *Client) GetReadme(name string) (string, error) {
	data, err := c.get("nodes/" + name + "/readme.md")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DownloadVersion downloads the tarball for a specific node version.
func (c *Client) DownloadVersion(name, version string) ([]byte, error) {
	return c.get("nodes/" + name + "/versions/" + version + ".tar.gz")
}

// FetchIndex fetches manifests for all known seed nodes and returns
// a searchable Index. In the future this will query an index endpoint;
// for now it fetches each known node individually.
func (c *Client) FetchIndex() (*Index, error) {
	idx := NewIndex()
	for _, name := range KnownNodes() {
		m, err := c.GetManifest(name)
		if err != nil {
			continue // skip unavailable nodes
		}
		idx.Add(m)
	}
	return idx, nil
}

// KnownNodes returns the names of all nodes seeded in the R2 bucket.
func KnownNodes() []string {
	return []string{
		"llm-inference",
		"http-request",
		"sqlite-store",
		"msg-relay",
		"web-scraper",
		"cron-scheduler",
		"vector-search",
		"file-watcher",
		"email-sender",
		"json-transform",
	}
}

// checksum returns the hex-encoded SHA-256 of data.
func checksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

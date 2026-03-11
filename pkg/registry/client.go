package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultBaseURL = "https://amaros.vercel.app"

// Client is an HTTP client for the AMAROS registry API.
//
// API:
//
//	GET  /api/nodes              — list/search nodes
//	GET  /api/nodes/:name        — node manifest + readme
//	GET  /api/nodes/:name/:ver   — version metadata
//	GET  /api/nodes/:name/:ver?download — download tarball
type Client struct {
	BaseURL string
	http    *http.Client
}

// NewClient creates a Client pointing at the given registry API base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// apiError is the JSON error body returned by the registry API.
type apiError struct {
	Error string `json:"error"`
}

// get performs a GET and returns the raw response body.
func (c *Client) get(path string) ([]byte, error) {
	u := c.BaseURL + path
	resp, err := c.http.Get(u)
	if err != nil {
		return nil, fmt.Errorf("registry: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("registry: read body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		var ae apiError
		if json.Unmarshal(body, &ae) == nil && ae.Error != "" {
			return nil, fmt.Errorf("registry: %s", ae.Error)
		}
		return nil, fmt.Errorf("registry: not found: %s", path)
	}
	if resp.StatusCode != http.StatusOK {
		var ae apiError
		if json.Unmarshal(body, &ae) == nil && ae.Error != "" {
			return nil, fmt.Errorf("registry: %s", ae.Error)
		}
		return nil, fmt.Errorf("registry: unexpected status %d for %s", resp.StatusCode, path)
	}

	return body, nil
}

// nodesListResponse is the JSON shape returned by GET /api/nodes.
type nodesListResponse struct {
	Nodes []NodeManifest `json:"nodes"`
	Count int            `json:"count"`
}

// nodeDetailResponse is the JSON shape returned by GET /api/nodes/:name.
// It extends NodeManifest with an inline readme field.
type nodeDetailResponse struct {
	NodeManifest
	Readme string `json:"readme"`
}

// versionMetaResponse is the JSON shape returned by GET /api/nodes/:name/:version.
type versionMetaResponse struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
	Size     int    `json:"size"`
}

// ListNodes fetches all nodes from the remote registry.
// Optional query filters by name/description/tags; optional tag filters by exact tag.
func (c *Client) ListNodes(query, tag string) ([]NodeManifest, error) {
	params := url.Values{}
	if query != "" {
		params.Set("q", query)
	}
	if tag != "" {
		params.Set("tag", tag)
	}

	path := "/api/nodes"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	data, err := c.get(path)
	if err != nil {
		return nil, err
	}

	var resp nodesListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("registry: invalid nodes response: %w", err)
	}
	return resp.Nodes, nil
}

// GetManifest fetches the full manifest and readme for a node by name.
func (c *Client) GetManifest(name string) (*NodeManifest, string, error) {
	data, err := c.get("/api/nodes/" + name)
	if err != nil {
		return nil, "", err
	}

	var resp nodeDetailResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, "", fmt.Errorf("registry: invalid manifest for %q: %w", name, err)
	}
	return &resp.NodeManifest, resp.Readme, nil
}

// GetVersionMeta fetches metadata for a specific version of a node.
// Use "latest" as the version to resolve to the newest release.
func (c *Client) GetVersionMeta(name, version string) (*versionMetaResponse, error) {
	data, err := c.get("/api/nodes/" + name + "/" + version)
	if err != nil {
		return nil, err
	}

	var resp versionMetaResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("registry: invalid version response: %w", err)
	}
	return &resp, nil
}

// DownloadVersion downloads the tarball for a specific node version.
func (c *Client) DownloadVersion(name, version string) ([]byte, error) {
	return c.get("/api/nodes/" + name + "/" + version + "?download")
}

// FetchIndex fetches all remote nodes and returns a searchable Index.
func (c *Client) FetchIndex() (*Index, error) {
	nodes, err := c.ListNodes("", "")
	if err != nil {
		return nil, err
	}
	idx := NewIndex()
	for i := range nodes {
		n := nodes[i]
		idx.Add(&n)
	}
	return idx, nil
}

// checksum returns the hex-encoded SHA-256 of data.
func checksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

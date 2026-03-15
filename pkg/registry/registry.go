package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	ilog "github.com/amar-jay/amaros/internal/logger"
	"github.com/amar-jay/amaros/pkg/config"
)

// Registry is the top-level API for managing AMAROS nodes.
// It combines the remote R2 client with a local on-disk store.
type Registry struct {
	Client *Client
	Store  *Store
	logger *ilog.Logger
}

// New creates a Registry backed by the given remote base URL and
// local root directory (typically ~/.config/amaros).
func New() (*Registry, error) {
	cfg := config.Get()
	if cfg == nil || cfg.Registry.Path == "" {
		return nil, fmt.Errorf("registry: root directory config error")
	}
	store, err := NewStore(cfg.Registry.Path)
	if err != nil {
		return nil, fmt.Errorf("registry: init store: %w", err)
	}
	return &Registry{
		Client: NewClient(cfg.Registry.APIURL),
		Store:  store,
		logger: ilog.New(),
	}, nil
}

// Search queries the remote registry for nodes matching the given term.
func (r *Registry) Search(query string) ([]SearchResult, error) {
	nodes, err := r.Client.ListNodes(query, "")
	if err != nil {
		return nil, err
	}
	var results []SearchResult
	for _, m := range nodes {
		results = append(results, SearchResult{
			Name:        m.Name,
			Description: m.Description,
			Latest:      m.Latest,
			Downloads:   m.TotalDownloads(),
		})
	}
	return results, nil
}

// SearchByTag queries the remote registry for nodes with an exact tag match.
func (r *Registry) SearchByTag(tag string) ([]SearchResult, error) {
	nodes, err := r.Client.ListNodes("", tag)
	if err != nil {
		return nil, err
	}
	var results []SearchResult
	for _, m := range nodes {
		results = append(results, SearchResult{
			Name:        m.Name,
			Description: m.Description,
			Latest:      m.Latest,
			Downloads:   m.TotalDownloads(),
		})
	}
	return results, nil
}

// Info fetches detailed information about a node from the remote registry.
// Returns the manifest and its readme.
func (r *Registry) Info(name string) (*NodeManifest, string, error) {
	return r.Client.GetManifest(name)
}

// Install downloads a node version from the remote registry and installs
// it locally. If version is empty, the latest version is used.
func (r *Registry) Install(name, version string) error {
	manifest, readme, err := r.Client.GetManifest(name)
	if err != nil {
		return fmt.Errorf("registry: fetch manifest: %w", err)
	}

	if version == "" {
		version = manifest.Latest
	}

	vi := manifest.GetVersion(version)
	if vi == nil {
		return fmt.Errorf("registry: version %q not found for node %q", version, name)
	}

	// check if already installed at this version
	if installed, _ := r.Store.GetInstalled(name); installed != nil {
		if installed.Version == version {
			r.logger.WithFields(map[string]interface{}{
				"node":    name,
				"version": version,
			}).Info("already installed, skipping")
			return nil
		}
	}

	r.logger.WithFields(map[string]interface{}{
		"node":    name,
		"version": version,
	}).Info("downloading")

	tarball, err := r.Client.DownloadVersion(name, version)
	if err != nil {
		return fmt.Errorf("registry: download: %w", err)
	}

	// verify checksum
	actual := checksum(tarball)
	if actual != vi.Checksum {
		return fmt.Errorf("registry: checksum mismatch for %s@%s (expected %s, got %s)",
			name, version, vi.Checksum, actual)
	}

	if err := r.Store.Install(manifest, vi, tarball, readme); err != nil {
		return fmt.Errorf("registry: install: %w", err)
	}

	r.logger.WithFields(map[string]interface{}{
		"node":    name,
		"version": version,
	}).Info("installed")
	return nil
}

// Uninstall removes a locally installed node.
func (r *Registry) Uninstall(name string) error {
	if err := r.Store.Uninstall(name); err != nil {
		return fmt.Errorf("registry: uninstall: %w", err)
	}
	r.logger.WithFields(map[string]interface{}{
		"node": name,
	}).Info("uninstalled")
	return nil
}

// Upgrade re-installs a node to its latest remote version.
func (r *Registry) Upgrade(name string) error {
	return r.Install(name, "")
}

// List returns all locally installed nodes.
func (r *Registry) List() ([]InstalledNode, error) {
	return r.Store.ListInstalled()
}

// ListRemote returns all nodes available in the remote registry.
func (r *Registry) ListRemote() ([]SearchResult, error) {
	nodes, err := r.Client.ListNodes("", "")
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, m := range nodes {
		results = append(results, SearchResult{
			Name:        m.Name,
			Description: m.Description,
			Latest:      m.Latest,
			Downloads:   m.TotalDownloads(),
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Downloads > results[j].Downloads
	})
	return results, nil
}

// Readme returns the readme for a node. It checks local first, then remote.
func (r *Registry) Readme(name string) (string, error) {
	// try local first
	if content, err := r.Store.GetReadme(name); err == nil {
		return content, nil
	}
	// fall back to remote
	_, readme, err := r.Client.GetManifest(name)
	if err != nil {
		return "", err
	}
	return readme, nil
}

// ── InstalledNode ───────────────────────────────────────────

// InstalledNode represents a node that has been downloaded to the local store.
type InstalledNode struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

// ── Index ───────────────────────────────────────────────────

// Index is an in-memory catalog of node manifests for search and listing.
type Index struct {
	Nodes map[string]*NodeManifest
}

// NewIndex creates an empty index.
func NewIndex() *Index {
	return &Index{Nodes: make(map[string]*NodeManifest)}
}

// Add inserts or updates a node manifest in the index.
func (idx *Index) Add(m *NodeManifest) {
	idx.Nodes[m.Name] = m
}

// Get returns a manifest by name, or nil if not found.
func (idx *Index) Get(name string) *NodeManifest {
	return idx.Nodes[name]
}

// List returns all node names sorted alphabetically.
func (idx *Index) List() []string {
	names := make([]string, 0, len(idx.Nodes))
	for name := range idx.Nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Search returns manifests matching by name, description, tags, or capabilities.
func (idx *Index) Search(query string) []SearchResult {
	q := strings.ToLower(query)
	var results []SearchResult

	for _, m := range idx.Nodes {
		if matchesQuery(m, q) {
			results = append(results, SearchResult{
				Name:        m.Name,
				Description: m.Description,
				Latest:      m.Latest,
				Downloads:   m.TotalDownloads(),
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Downloads > results[j].Downloads
	})
	return results
}

// matchesQuery checks name, description, tags, and capabilities.
func matchesQuery(m *NodeManifest, q string) bool {
	if strings.Contains(strings.ToLower(m.Name), q) {
		return true
	}
	if strings.Contains(strings.ToLower(m.Description), q) {
		return true
	}
	for _, tag := range m.Tags {
		if strings.Contains(strings.ToLower(tag), q) {
			return true
		}
	}
	for _, cap := range m.Capabilities {
		if strings.Contains(strings.ToLower(cap), q) {
			return true
		}
	}
	return false
}

// ── SearchResult ────────────────────────────────────────────

// SearchResult is a summary entry returned by search operations.
type SearchResult struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Latest      string `json:"latest"`
	Downloads   int    `json:"downloads"`
}

// ── helpers (used by Store for lock files) ───────────────────

func writeJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func readJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

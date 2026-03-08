package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Store manages the local on-disk layout of installed AMAROS nodes.
//
// Directory structure under rootDir:
//
//	~/.config/amaros/
//	├── config.yaml                  (managed by internal/config)
//	├── nodes/
//	│   ├── llm-inference/
//	│   │   ├── manifest.json        — NodeManifest snapshot at install time
//	│   │   ├── readme.md            — cached readme
//	│   │   ├── node.tar.gz          — version tarball
//	│   │   └── installed.json       — InstalledNode metadata
//	│   ├── http-request/
//	│   │   └── ...
//	│   └── ...
//	└── registry/                    (reserved for future index cache)
type Store struct {
	RootDir  string // ~/.config/amaros
	NodesDir string // ~/.config/amaros/nodes
}

// NewStore creates a Store rooted at the given directory, creating the
// nodes/ subdirectory if it doesn't exist.
func NewStore(rootDir string) (*Store, error) {
	nodesDir := filepath.Join(rootDir, "nodes")
	if err := os.MkdirAll(nodesDir, 0755); err != nil {
		return nil, fmt.Errorf("store: create nodes dir: %w", err)
	}
	return &Store{
		RootDir:  rootDir,
		NodesDir: nodesDir,
	}, nil
}

// nodeDir returns the path for a specific node's local directory.
func (s *Store) nodeDir(name string) string {
	return filepath.Join(s.NodesDir, name)
}

// Install writes a node's tarball, manifest, and readme into the local store.
func (s *Store) Install(manifest *NodeManifest, version *VersionInfo, tarball []byte, readme string) error {
	dir := s.nodeDir(manifest.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("store: create node dir: %w", err)
	}

	// write manifest
	if err := writeJSON(filepath.Join(dir, "manifest.json"), manifest); err != nil {
		return fmt.Errorf("store: write manifest: %w", err)
	}

	// write installed metadata
	installed := InstalledNode{
		Name:    manifest.Name,
		Version: version.Version,
		Path:    dir,
	}
	if err := writeJSON(filepath.Join(dir, "installed.json"), installed); err != nil {
		return fmt.Errorf("store: write installed.json: %w", err)
	}

	// write tarball
	if err := os.WriteFile(filepath.Join(dir, "node.tar.gz"), tarball, 0644); err != nil {
		return fmt.Errorf("store: write tarball: %w", err)
	}

	// write readme (best effort)
	if readme != "" {
		_ = os.WriteFile(filepath.Join(dir, "readme.md"), []byte(readme), 0644)
	}

	return nil
}

// Uninstall removes a node's directory from the local store.
func (s *Store) Uninstall(name string) error {
	dir := s.nodeDir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("store: node %q is not installed", name)
	}
	return os.RemoveAll(dir)
}

// GetInstalled reads the installed.json for a node, returning nil if not installed.
func (s *Store) GetInstalled(name string) (*InstalledNode, error) {
	p := filepath.Join(s.nodeDir(name), "installed.json")
	var n InstalledNode
	if err := readJSON(p, &n); err != nil {
		return nil, err
	}
	return &n, nil
}

// GetManifest reads the cached manifest for an installed node.
func (s *Store) GetManifest(name string) (*NodeManifest, error) {
	p := filepath.Join(s.nodeDir(name), "manifest.json")
	var m NodeManifest
	if err := readJSON(p, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// GetReadme reads the cached readme for an installed node.
func (s *Store) GetReadme(name string) (string, error) {
	p := filepath.Join(s.nodeDir(name), "readme.md")
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// IsInstalled checks whether a node exists in the local store.
func (s *Store) IsInstalled(name string) bool {
	p := filepath.Join(s.nodeDir(name), "installed.json")
	_, err := os.Stat(p)
	return err == nil
}

// ListInstalled returns all locally installed nodes, sorted by name.
func (s *Store) ListInstalled() ([]InstalledNode, error) {
	entries, err := os.ReadDir(s.NodesDir)
	if err != nil {
		return nil, fmt.Errorf("store: read nodes dir: %w", err)
	}

	var nodes []InstalledNode
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n, err := s.GetInstalled(e.Name())
		if err != nil {
			continue // skip corrupt entries
		}
		nodes = append(nodes, *n)
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// NodeTarballPath returns the path to the installed tarball for a node,
// or an error if not installed.
func (s *Store) NodeTarballPath(name string) (string, error) {
	p := filepath.Join(s.nodeDir(name), "node.tar.gz")
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("store: tarball not found for %q", name)
	}
	return p, nil
}

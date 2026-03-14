package registry

// VersionInfo describes a single published version of a node.
type VersionInfo struct {
	Version     string `json:"version"`
	PublishedAt string `json:"publishedAt"`
	Checksum    string `json:"checksum"`
	Size        int    `json:"size"`
	Downloads   int    `json:"downloads"`
}

// NodeManifest is the metadata for a registered AMAROS node.
// This mirrors the R2 bucket layout: nodes/{name}/metadata.json
type NodeManifest struct {
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Author       string        `json:"author"`
	Organization string        `json:"organization,omitempty"`
	License      string        `json:"license"`
	Repository   string        `json:"repository,omitempty"`
	Latest       string        `json:"latest"`
	Versions     []VersionInfo `json:"versions"`
	Tags         []string      `json:"tags"`
	Capabilities []string      `json:"capabilities"`
	SubscribesTo []string      `json:"subscribesTo"`
	PublishesTo  []string      `json:"publishesTo"`
	CreatedAt    string        `json:"createdAt"`
	UpdatedAt    string        `json:"updatedAt"`
}

// TotalDownloads returns the sum of downloads across all versions.
func (m *NodeManifest) TotalDownloads() int {
	total := 0
	for _, v := range m.Versions {
		total += v.Downloads
	}
	return total
}

// GetVersion returns the VersionInfo for a specific version string,
// or nil if not found.
func (m *NodeManifest) GetVersion(version string) *VersionInfo {
	for i := range m.Versions {
		if m.Versions[i].Version == version {
			return &m.Versions[i]
		}
	}
	return nil
}

// LatestVersion returns the VersionInfo for the latest release.
func (m *NodeManifest) LatestVersion() *VersionInfo {
	return m.GetVersion(m.Latest)
}


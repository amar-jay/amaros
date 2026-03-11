# pkg/registry

The registry package manages AMAROS node discovery, installation, and local storage. It talks to the AMAROS registry API at `https://amaros.vercel.app/` and maintains a local cache under `~/.config/amaros/nodes/`.

## Package structure

| File | Purpose |
|------|---------|
| `types.go` | `NodeManifest`, `VersionInfo` — data types matching the registry API |
| `client.go` | HTTP client wrapping the registry REST API |
| `store.go` | Local filesystem store — install/uninstall/query nodes on disk |
| `registry.go` | High-level API combining client + store |

## API endpoints used

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/nodes?q=&tag=` | List/search nodes |
| GET | `/api/nodes/:name` | Node manifest + readme |
| GET | `/api/nodes/:name/:version` | Version metadata |
| GET | `/api/nodes/:name/:version?download` | Download tarball |

## Local store layout

```
~/.config/amaros/
├── config.yaml
└── nodes/
    ├── llm-inference/
    │   ├── manifest.json      # cached NodeManifest
    │   ├── installed.json     # { name, version, path }
    │   ├── node.tar.gz        # version tarball
    │   └── readme.md          # cached readme
    └── ...
```

## Usage

```go
import "github.com/amar-jay/amaros/pkg/registry"

// create a registry (uses the real API)
reg, err := registry.New(
    registry.DefaultBaseURL,         // https://amaros.vercel.app
)

// search remote nodes by text
results, _ := reg.Search("llm")

// search by tag
results, _ = reg.SearchByTag("ai")

// install a node (latest version)
reg.Install("llm-inference", "")

// install a specific version
reg.Install("http-request", "1.5.0")

// list locally installed nodes
installed, _ := reg.List()

// get info + readme about a remote node
manifest, readme, _ := reg.Info("sqlite-store")

// read just the readme (checks local first, then remote)
readme, _ = reg.Readme("llm-inference")

// list everything available remotely
all, _ := reg.ListRemote()

// upgrade to latest
reg.Upgrade("llm-inference")

// uninstall
reg.Uninstall("llm-inference")
```

## Lower-level access

```go
// client-only (no local store)
client := registry.NewClient(registry.DefaultBaseURL)
manifest, readme, _ := client.GetManifest("web-scraper")
tarball, _ := client.DownloadVersion("web-scraper", "2.0.4")
nodes, _ := client.ListNodes("llm", "")         // search by query
nodes, _ = client.ListNodes("", "ai")            // filter by tag
meta, _ := client.GetVersionMeta("web-scraper", "latest")

// store-only (no network)
store, _ := registry.NewStore(registry.DefaultRootDir())
ok := store.IsInstalled("llm-inference")
installed, _ := store.ListInstalled()
```

## Install flow

1. Fetch manifest + readme from `GET /api/nodes/{name}`
2. Resolve version (default: `manifest.Latest`)
3. Skip if already installed at that version
4. Download tarball via `GET /api/nodes/{name}/{version}?download`
5. Verify SHA-256 checksum against `VersionInfo.Checksum`
6. Write manifest, tarball, readme, and `installed.json` to `~/.config/amaros/nodes/{name}/`

# pkg/registry

The registry package manages AMAROS node discovery, installation, and local storage. It talks to a remote Cloudflare R2 bucket for the node catalog and maintains a local cache under `~/.config/amaros/nodes/`.

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

// create a registry pointing at the R2 bucket
reg, err := registry.New(
    "https://your-bucket.r2.dev",
    registry.DefaultRootDir(),       // ~/.config/amaros
)

// search remote nodes
results, _ := reg.Search("llm")

// install a node (latest version)
reg.Install("llm-inference", "")

// install a specific version
reg.Install("http-request", "1.5.0")

// list locally installed nodes
installed, _ := reg.List()

// get info about a remote node
manifest, _ := reg.Info("sqlite-store")

// read a node's readme (checks local first, then remote)
readme, _ := reg.Readme("llm-inference")

// upgrade to latest
reg.Upgrade("llm-inference")

// uninstall
reg.Uninstall("llm-inference")

// list everything available remotely
all, _ := reg.ListRemote()
```

## Lower-level access

```go
// client-only (no local store)
client := registry.NewClient("https://your-bucket.r2.dev")
manifest, _ := client.GetManifest("web-scraper")
tarball, _ := client.DownloadVersion("web-scraper", "2.0.4")

// store-only (no network)
store, _ := registry.NewStore(registry.DefaultRootDir())
ok := store.IsInstalled("llm-inference")
nodes, _ := store.ListInstalled()

// in-memory search index
idx, _ := client.FetchIndex()
results := idx.Search("ai")
```

## Install flow

1. Fetch `NodeManifest` from R2 (`nodes/{name}/metadata.json`)
2. Resolve version (default: `manifest.Latest`)
3. Skip if already installed at that version
4. Download tarball (`nodes/{name}/versions/{v}.tar.gz`)
5. Verify SHA-256 checksum against `VersionInfo.Checksum`
6. Write manifest, tarball, readme, and `installed.json` to `~/.config/amaros/nodes/{name}/`

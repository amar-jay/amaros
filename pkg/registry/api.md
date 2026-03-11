# AMAROS Registry API

Base URL: `https://amaros.vercel.app/`

---

## Nodes

### `GET /api/nodes`

List all nodes in the registry, with optional filtering.

**Query Parameters**

| Parameter | Type   | Description                                                        |
| --------- | ------ | ------------------------------------------------------------------ |
| `q`       | string | Case-insensitive text search across node name, description, & tags |
| `tag`     | string | Exact match on a tag (case-insensitive)                            |

**Response** `200`

```json
{
  "nodes": [NodeManifest],
  "count": 12
}
```

---

### `GET /api/nodes/:name`

Get a single node's full manifest and readme.

**Path Parameters**

| Parameter | Type   | Description          |
| --------- | ------ | -------------------- |
| `name`    | string | The node's unique name |

**Response** `200`

```json
{
  "name": "llm-inference",
  "description": "...",
  "author": "...",
  "organization": "...",
  "license": "MIT",
  "repository": "https://github.com/...",
  "latest": "1.2.0",
  "versions": [VersionEntry],
  "tags": ["ai", "llm"],
  "capabilities": ["inference"],
  "subscribesTo": ["prompt.request"],
  "publishesTo": ["prompt.response"],
  "createdAt": "2025-01-01T00:00:00Z",
  "updatedAt": "2025-06-01T00:00:00Z",
  "readme": "# LLM Inference\n..."
}
```

**Response** `404`

```json
{ "error": "Node \"foo\" not found" }
```

---

### `GET /api/nodes/:name/:version`

Get version metadata, or download the tarball.

**Path Parameters**

| Parameter | Type   | Description                                  |
| --------- | ------ | -------------------------------------------- |
| `name`    | string | The node's unique name                       |
| `version` | string | Semver version string, or `"latest"`         |

**Query Parameters**

| Parameter  | Type | Description                                  |
| ---------- | ---- | -------------------------------------------- |
| `download` | flag | If present, streams the `.tar.gz` tarball    |

**Response** `200` (metadata, without `?download`)

```json
{
  "name": "llm-inference",
  "version": "1.2.0",
  "checksum": "sha256:abc123...",
  "size": 102400
}
```

**Response** `200` (tarball, with `?download`)

Binary stream with headers:
- `Content-Type: application/gzip`
- `Content-Disposition: attachment; filename="llm-inference-1.2.0.tar.gz"`

**Response** `404`

```json
{ "error": "Version \"9.9.9\" not found for \"llm-inference\"" }
```

---

## Publish

### `POST /api/publish`

Publish a new node version to the registry. NB:Auth requred

**Content-Type:** `multipart/form-data`

**Form Fields**

| Field      | Type   | Required | Description                          |
| ---------- | ------ | -------- | ------------------------------------ |
| `manifest` | string | Yes      | JSON-encoded `NodeManifest` object   |
| `tarball`  | file   | Yes      | `.tar.gz` archive of the node        |
| `readme`   | string | No       | Markdown readme content              |

**Manifest Validation**
- `name`, `latest`, and `versions` (non-empty) are required.
- `name` must match `/^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/` (lowercase alphanumeric with hyphens).

**Response** `201`

```json
{
  "ok": true,
  "name": "my-node",
  "version": "1.0.0"
}
```

**Response** `400`

```json
{ "error": "Missing 'manifest' field (JSON string)" }
```

**Response** `409`

```json
{ "error": "Version 1.0.0 already exists" }
```

---

## Types

### `NodeManifest`

```ts
interface NodeManifest {
  name: string;
  description: string;
  author: string;
  organization?: string;
  license: string;
  repository?: string;
  latest: string;
  versions: VersionEntry[];
  tags: string[];
  capabilities: string[];
  subscribesTo: string[];
  publishesTo: string[];
  createdAt: string;
  updatedAt: string;
}
```

### `VersionEntry`

```ts
interface VersionEntry {
  version: string;
  publishedAt: string;
  checksum: string;   // sha256
  size: number;       // bytes
  downloads: number;
}
```

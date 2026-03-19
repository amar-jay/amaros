package memory

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	chroma "github.com/amikos-tech/chroma-go/pkg/api/v2"
	ort "github.com/amikos-tech/chroma-go/pkg/embeddings/ort"
)

// TODO: add limit to hot store and auto-evict to warm store when exceeded

// HotStore is an in-memory key-value store using a concurrent map.
type HotStore struct {
	mu         sync.RWMutex
	client     chroma.Client
	col        chroma.Collection
	closeEmbed func() error
}

const (
	hotCollectionName  = "amaros_hot_store"
	hotMetaCreatedAtMS = "created_at_ms"
	hotMetaUpdatedAtMS = "updated_at_ms"
	hotMetaPayloadB64  = "payload_b64"
	hotMetaOriginalKey = "original_key"
)

// opts is either a connection string for remote chroma or a directory path for local chroma.
func NewHotStore(opts string) (*HotStore, error) {
	ctx := context.Background()
	if opts == "" {
		return nil, fmt.Errorf("parameter required for hot store")
	}

	var (
		client chroma.Client
		err    error
	)

	if _, err := os.Stat(opts); err == nil {
		hotPath := filepath.Join(opts, "hot")
		if err := os.MkdirAll(hotPath, 0o750); err != nil {
			return nil, fmt.Errorf("create hot chroma dir: %w", err)
		}
		client, err = chroma.NewPersistentClient(chroma.WithPersistentPath(hotPath))
	} else if url, err := url.Parse(opts); err == nil && url.Scheme != "" && url.Host != "" {
		client, err = chroma.NewHTTPClient(chroma.WithBaseURL(opts))
	} else {
		return nil, fmt.Errorf("invalid parameter for hot store: must be a directory path or URL")
	}

	if err != nil {
		return nil, fmt.Errorf("init hot chroma client: %w", err)
	}

	embedFn, closeEmbedFn, err := ort.NewDefaultEmbeddingFunction()
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("init hot ort embedding function: %w", err)
	}

	col, err := client.GetOrCreateCollection(
		ctx,
		hotCollectionName,
		chroma.WithEmbeddingFunctionCreate(embedFn),
	)
	if err != nil {
		_ = closeEmbedFn()
		_ = client.Close()
		return nil, fmt.Errorf("init hot chroma collection: %w", err)
	}

	return &HotStore{client: client, col: col, closeEmbed: closeEmbedFn}, nil
}

// Get should retrieve the one with the highest relevance score > threshold (default 0.8) and return nil if no match.
func (h *HotStore) Get(key string) (*Entry, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	query := strings.TrimSpace(key)
	if query == "" {
		return nil, nil
	}
	queryRes, err := h.col.Query(
		context.Background(),
		chroma.WithQueryTexts(query),
		chroma.WithNResults(1),
		chroma.WithInclude(chroma.IncludeDocuments, chroma.IncludeMetadatas),
	)
	if err != nil {
		return nil, fmt.Errorf("query hot entry for %q: %w", key, err)
	}

	idGroups := queryRes.GetIDGroups()
	if len(idGroups) == 0 || len(idGroups[0]) == 0 {
		return nil, nil
	}
	ids := idGroups[0]

	var docs chroma.Documents
	if docGroups := queryRes.GetDocumentsGroups(); len(docGroups) > 0 {
		docs = docGroups[0]
	}
	var metas chroma.DocumentMetadatas
	if metadataGroups := queryRes.GetMetadatasGroups(); len(metadataGroups) > 0 {
		metas = metadataGroups[0]
	}

	return buildHotEntryFromResult(string(ids[0]), docs, metas, 0), nil
}

func (h *HotStore) GetWithThreshold(key string, threshold float64) (*Entry, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if threshold < 0 || threshold > 1 {
		return nil, fmt.Errorf("threshold must be between 0 and 1")
	}

	query := strings.TrimSpace(key)
	if query == "" {
		return nil, nil
	}

	queryRes, err := h.col.Query(
		context.Background(),
		chroma.WithQueryTexts(query),
		chroma.WithNResults(1),
		// CRITICAL: We must include Distances to calculate relevance
		chroma.WithInclude(chroma.IncludeDocuments, chroma.IncludeMetadatas, chroma.IncludeDistances),
	)
	if err != nil {
		return nil, fmt.Errorf("query hot entry for %q: %w", key, err)
	}

	idGroups := queryRes.GetIDGroups()
	if len(idGroups) == 0 || len(idGroups[0]) == 0 {
		return nil, nil
	}

	// 1. Extract Distance
	distGroups := queryRes.GetDistancesGroups()
	if len(distGroups) == 0 || len(distGroups[0]) == 0 {
		return nil, nil // No distance returned, cannot verify threshold
	}

	distance := distGroups[0][0]
	// Convert L2 Distance to a Similarity Score (0.0 to 1.0 range)
	// Note: This assumes your embedding space is Cosine or Normalized L2
	similarity := 1.0 - float64(distance)

	// 2. Threshold Gate
	if similarity < threshold {
		// Log this for debugging if needed
		// fmt.Printf("[Low Confidence] Query: %s (Score: %.2f)\n", query, similarity)
		return nil, nil
	}

	// 3. Build Result
	ids := idGroups[0]
	var docs chroma.Documents
	if docGroups := queryRes.GetDocumentsGroups(); len(docGroups) > 0 {
		docs = docGroups[0]
	}
	var metas chroma.DocumentMetadatas
	if metadataGroups := queryRes.GetMetadatasGroups(); len(metadataGroups) > 0 {
		metas = metadataGroups[0]
	}

	return buildHotEntryFromResult(string(ids[0]), docs, metas, 0), nil
}

func (h *HotStore) Set(key string, value []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	ctx := context.Background()
	now := time.Now()
	createdAt := now
	if existing, err := h.getByID(ctx, key); err != nil {
		return err
	} else if existing != nil {
		createdAt = existing.CreatedAt
	}

	md := chroma.NewDocumentMetadata(
		chroma.NewIntAttribute(hotMetaCreatedAtMS, createdAt.UnixMilli()),
		chroma.NewIntAttribute(hotMetaUpdatedAtMS, now.UnixMilli()),
		chroma.NewStringAttribute(hotMetaPayloadB64, base64.StdEncoding.EncodeToString(value)),
		chroma.NewStringAttribute(hotMetaOriginalKey, key),
	)

	err := h.col.Upsert(
		ctx,
		chroma.WithIDs(chroma.DocumentID(key)),
		chroma.WithTexts(key),
		chroma.WithMetadatas(md),
	)
	if err != nil {
		return fmt.Errorf("upsert hot entry %q: %w", key, err)
	}
	return nil
}

func (h *HotStore) Delete(key string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	err := h.col.Delete(context.Background(), chroma.WithIDs(chroma.DocumentID(key)))
	if err != nil {
		return fmt.Errorf("delete hot entry %q: %w", key, err)
	}
	return nil
}

func (h *HotStore) List(prefix string) ([]*Entry, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	res, err := h.col.Get(
		context.Background(),
		chroma.WithInclude(chroma.IncludeDocuments, chroma.IncludeMetadatas),
	)
	if err != nil {
		return nil, fmt.Errorf("list hot entries: %w", err)
	}

	ids := res.GetIDs()
	docs := res.GetDocuments()
	metas := res.GetMetadatas()
	entries := make([]*Entry, 0, len(ids))
	for i, id := range ids {
		entry := buildHotEntryFromResult(string(id), docs, metas, i)
		if prefix != "" && !strings.HasPrefix(entry.Key, prefix) {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (h *HotStore) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var firstErr error
	if h.col != nil {
		if err := h.col.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close hot collection: %w", err)
		}
		h.col = nil
	}
	if h.client != nil {
		if err := h.client.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close hot client: %w", err)
		}
		h.client = nil
	}
	if h.closeEmbed != nil {
		if err := h.closeEmbed(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close hot embedding function: %w", err)
		}
		h.closeEmbed = nil
	}

	return firstErr
}

func (h *HotStore) getByID(ctx context.Context, key string) (*Entry, error) {
	res, err := h.col.Get(
		ctx,
		chroma.WithIDs(chroma.DocumentID(key)),
		chroma.WithInclude(chroma.IncludeDocuments, chroma.IncludeMetadatas),
	)
	if err != nil {
		return nil, fmt.Errorf("get hot entry %q: %w", key, err)
	}

	ids := res.GetIDs()
	if len(ids) == 0 {
		return nil, nil
	}

	docs := res.GetDocuments()
	metas := res.GetMetadatas()
	return buildHotEntryFromResult(string(ids[0]), docs, metas, 0), nil
}

func buildHotEntryFromResult(id string, docs chroma.Documents, metas chroma.DocumentMetadatas, idx int) *Entry {
	now := time.Now()
	entry := &Entry{
		Key:       id,
		Tier:      Hot,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if idx < len(docs) && docs[idx] != nil {
		text := docs[idx].ContentString()
		if text != "" {
			entry.Key = text
		}
		entry.Value = []byte(text)
	}

	if idx < len(metas) && metas[idx] != nil {
		if originalKey, ok := metas[idx].GetString(hotMetaOriginalKey); ok && originalKey != "" {
			entry.Key = originalKey
		}
		if createdAtMS, ok := metas[idx].GetInt(hotMetaCreatedAtMS); ok {
			entry.CreatedAt = time.UnixMilli(createdAtMS)
		}
		if updatedAtMS, ok := metas[idx].GetInt(hotMetaUpdatedAtMS); ok {
			entry.UpdatedAt = time.UnixMilli(updatedAtMS)
		}
		if payloadB64, ok := metas[idx].GetString(hotMetaPayloadB64); ok && payloadB64 != "" {
			if decoded, err := base64.StdEncoding.DecodeString(payloadB64); err == nil {
				entry.Value = decoded
			}
		}
	}

	return entry
}

package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	chroma "github.com/amikos-tech/chroma-go/pkg/api/v2"
	"github.com/amikos-tech/chroma-go/pkg/embeddings"

	"github.com/amar-jay/amaros/pkg/config"
)

// VectorStore wraps a Chroma collection for semantic memory.
type VectorStore struct {
	client     chroma.Client
	collection chroma.Collection
}

func NewVectorStoreFromConfig(ctx context.Context, cfg config.MemoryConfig) (*VectorStore, error) {
	mode := strings.ToLower(cfg.VectorMode)
	if mode == "" {
		mode = "persistent"
	}
	collectionName := cfg.Collection
	if collectionName == "" {
		collectionName = "amaros_memory"
	}

	var (
		client chroma.Client
		err    error
	)

	switch mode {
	case "http":
		client, err = chroma.NewHTTPClient(chroma.WithBaseURL(cfg.VectorURL))
	default:
		path := cfg.PersistentDir
		if path == "" && cfg.RootDir != "" {
			path = cfg.RootDir
		}
		client, err = chroma.NewPersistentClient(chroma.WithPersistentPath(path))
	}
	if err != nil {
		return nil, fmt.Errorf("create chroma client: %w", err)
	}

	col, err := client.GetOrCreateCollection(ctx, collectionName)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("get or create collection: %w", err)
	}

	return &VectorStore{
		client:     client,
		collection: col,
	}, nil
}

func (s *VectorStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

// Upsert stores or updates a note in the vector collection.
func (s *VectorStore) Upsert(ctx context.Context, note Note) error {
	if s == nil || s.collection == nil {
		return fmt.Errorf("vector store is not initialised")
	}

	meta, err := chroma.NewDocumentMetadataFromMap(stringMapToInterface(note.Metadata))
	if err != nil {
		return fmt.Errorf("convert metadata: %w", err)
	}

	text := strings.TrimSpace(note.Title + "\n\n" + note.Content)
	opts := []chroma.CollectionAddOption{
		chroma.WithIDs(chroma.DocumentID(note.ID)),
		chroma.WithTexts(text),
	}
	if meta != nil {
		opts = append(opts, chroma.WithMetadatas(meta))
	}

	return s.collection.Upsert(ctx, opts...)
}

// Query returns the closest notes for a query string.
func (s *VectorStore) Query(ctx context.Context, query string, limit int) ([]Note, error) {
	if s == nil || s.collection == nil {
		return nil, fmt.Errorf("vector store is not initialised")
	}
	if limit <= 0 {
		limit = 5
	}

	result, err := s.collection.Query(
		ctx,
		chroma.WithQueryTexts(query),
		chroma.WithNResults(limit),
		chroma.WithInclude(chroma.IncludeDocuments, chroma.IncludeMetadatas, chroma.IncludeDistances),
	)
	if err != nil {
		return nil, err
	}

	var notes []Note
	docGroups := result.GetDocumentsGroups()
	metaGroups := result.GetMetadatasGroups()
	idGroups := result.GetIDGroups()
	distGroups := result.GetDistancesGroups()

	for g, ids := range idGroups {
		for i, id := range ids {
			n := Note{
				ID:        string(id),
				Source:    "vector",
				Score:     similarity(distGroups, g, i),
				CreatedAt: timeZeroSafe(metaGroups, g, i),
			}
			if g < len(docGroups) && i < len(docGroups[g]) && docGroups[g][i] != nil {
				n.Content = docGroups[g][i].ContentString()
			}
			if g < len(metaGroups) && i < len(metaGroups[g]) && metaGroups[g][i] != nil {
				n.Metadata = metadataToStringMap(metaGroups[g][i])
			}
			if n.Title == "" && n.Metadata != nil {
				if title, ok := n.Metadata["title"]; ok {
					n.Title = title
				}
			}
			if n.Title == "" {
				n.Title = "Memory"
			}
			notes = append(notes, n)
		}
	}

	return notes, nil
}

func stringMapToInterface(in map[string]string) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func metadataToStringMap(meta chroma.DocumentMetadata) map[string]string {
	if meta == nil {
		return nil
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return nil
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	result := make(map[string]string, len(decoded))
	for k, v := range decoded {
		result[k] = fmt.Sprint(v)
	}
	return result
}

func similarity(distances []embeddings.Distances, group, idx int) float64 {
	if group >= len(distances) || idx >= len(distances[group]) {
		return 0
	}
	d := float64(distances[group][idx])
	if d <= 0 {
		return 1
	}
	return 1 / (1 + math.Max(d, 0))
}

func timeZeroSafe(metas []chroma.DocumentMetadatas, g, i int) (t time.Time) {
	if g < len(metas) && i < len(metas[g]) && metas[g][i] != nil {
		raw, err := json.Marshal(metas[g][i])
		if err == nil {
			var decoded map[string]interface{}
			if err := json.Unmarshal(raw, &decoded); err == nil {
				if ts, ok := decoded["created_at"]; ok {
					if parsed, parseErr := time.Parse(time.RFC3339, fmt.Sprint(ts)); parseErr == nil {
						return parsed
					}
				}
			}
		}
	}
	return time.Time{}
}

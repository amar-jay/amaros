package memory

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"

	"github.com/amar-jay/amaros/pkg/config"
)

// Note represents a single memory item stored in markdown or the vector store.
type Note struct {
	ID        string
	Title     string
	Content   string
	Tags      []string
	Metadata  map[string]string
	CreatedAt time.Time
	Source    string
	Score     float64
}

// TaskRecord captures execution details for persistence.
type TaskRecord struct {
	TaskID      string
	Description string
	Summary     string
	Output      string
	Success     bool
	Tags        []string
}

// Manager coordinates markdown (episodic) and vector (semantic) memories.
type Manager struct {
	markdown *MarkdownStore
	vector   *VectorStore
}

// NewManager constructs a Manager using repository configuration.
func NewManager(ctx context.Context, cfg config.MemoryConfig) (*Manager, error) {
	markdownDir := cfg.MarkdownDir
	if markdownDir == "" && cfg.RootDir != "" {
		markdownDir = filepath.Join(cfg.RootDir, "journal")
	}
	markdownStore, err := NewMarkdownStore(markdownDir)
	if err != nil {
		return nil, err
	}

	vectorStore, err := NewVectorStoreFromConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return &Manager{
		markdown: markdownStore,
		vector:   vectorStore,
	}, nil
}

// Close releases underlying resources.
func (m *Manager) Close() error {
	if m == nil || m.vector == nil {
		return nil
	}
	return m.vector.Close()
}

// RecordTask persists the outcome of a task to both memory tiers.
func (m *Manager) RecordTask(ctx context.Context, record TaskRecord) (Note, error) {
	if m == nil {
		return Note{}, fmt.Errorf("memory manager is not initialised")
	}

	note := Note{
		ID:        record.TaskID,
		Title:     buildTitle(record),
		Content:   buildContent(record),
		Tags:      append([]string{"task"}, record.Tags...),
		CreatedAt: time.Now().UTC(),
		Metadata: map[string]string{
			"task_id": record.TaskID,
			"success": fmt.Sprintf("%t", record.Success),
		},
		Source: "markdown",
	}

	if note.ID == "" {
		note.ID = mustID()
	}

	savedNote, err := m.markdown.Save(note)
	if err != nil {
		return savedNote, err
	}

	if m.vector != nil {
		_ = m.vector.Upsert(ctx, savedNote)
	}
	return savedNote, nil
}

// Recall searches semantic memory and falls back to markdown search.
func (m *Manager) Recall(ctx context.Context, query string, limit int) ([]Note, error) {
	if m == nil {
		return nil, fmt.Errorf("memory manager is not initialised")
	}

	var results []Note
	if m.vector != nil && query != "" {
		if hits, err := m.vector.Query(ctx, query, limit); err == nil {
			results = append(results, hits...)
		}
	}

	// Fill remaining slots with markdown matches or return only markdown when vector is unavailable.
	if limit <= 0 {
		limit = 5
	}
	if len(results) < limit {
		mdHits, err := m.markdown.Search(query, limit)
		if err == nil {
			results = append(results, mdHits...)
		}
	}

	deduped := dedupeNotes(results)
	if len(deduped) > limit {
		deduped = deduped[:limit]
	}
	return deduped, nil
}

// Recent returns the most recent episodic notes from markdown.
func (m *Manager) Recent(limit int) ([]Note, error) {
	if m == nil {
		return nil, fmt.Errorf("memory manager is not initialised")
	}
	return m.markdown.Recent(limit)
}

// FormatNotesForPrompt renders notes for LLM consumption.
func FormatNotesForPrompt(notes []Note) string {
	if len(notes) == 0 {
		return "- (no memory available)"
	}

	var builder strings.Builder
	for _, note := range notes {
		builder.WriteString("- ")
		builder.WriteString(note.Title)
		if note.CreatedAt.IsZero() == false {
			builder.WriteString(" @ ")
			builder.WriteString(note.CreatedAt.Format(time.RFC3339))
		}
		if note.Source != "" {
			builder.WriteString(" [")
			builder.WriteString(note.Source)
			builder.WriteString("]")
		}
		if note.Score > 0 {
			builder.WriteString(fmt.Sprintf(" score=%.3f", note.Score))
		}
		if trimmed := strings.TrimSpace(note.Content); trimmed != "" {
			builder.WriteString(": ")
			builder.WriteString(trimmed)
		}
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func buildTitle(record TaskRecord) string {
	if record.TaskID != "" {
		return fmt.Sprintf("Task %s", record.TaskID)
	}
	if record.Description != "" {
		return record.Description
	}
	return "Untitled Task"
}

func buildContent(record TaskRecord) string {
	var parts []string
	if record.Description != "" {
		parts = append(parts, fmt.Sprintf("Description: %s", record.Description))
	}
	if record.Summary != "" {
		parts = append(parts, fmt.Sprintf("Summary: %s", record.Summary))
	}
	if record.Output != "" {
		parts = append(parts, fmt.Sprintf("Output: %s", record.Output))
	}
	if len(parts) == 0 {
		return "No additional details recorded."
	}
	return strings.Join(parts, "\n")
}

func mustID() string {
	id, err := gonanoid.New()
	if err != nil {
		return fmt.Sprintf("note-%d", time.Now().UnixNano())
	}
	return id
}

func dedupeNotes(notes []Note) []Note {
	seen := make(map[string]Note, len(notes))
	for _, note := range notes {
		existing, ok := seen[note.ID]
		if !ok {
			seen[note.ID] = note
			continue
		}
		// Prefer higher score, then latest created time.
		if note.Score > existing.Score || note.CreatedAt.After(existing.CreatedAt) {
			seen[note.ID] = note
		}
	}

	result := make([]Note, 0, len(seen))
	for _, note := range seen {
		result = append(result, note)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			return result[i].CreatedAt.After(result[j].CreatedAt)
		}
		return result[i].Score > result[j].Score
	})
	return result
}

package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type MarkdownStore struct {
	dir string
}

type noteFrontMatter struct {
	ID        string            `yaml:"id"`
	Title     string            `yaml:"title"`
	CreatedAt time.Time         `yaml:"created_at"`
	Tags      []string          `yaml:"tags,omitempty"`
	Metadata  map[string]string `yaml:"metadata,omitempty"`
}

func NewMarkdownStore(dir string) (*MarkdownStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("markdown directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create markdown dir: %w", err)
	}
	return &MarkdownStore{dir: dir}, nil
}

func (s *MarkdownStore) Save(note Note) (Note, error) {
	if note.CreatedAt.IsZero() {
		note.CreatedAt = time.Now().UTC()
	}
	if note.ID == "" {
		note.ID = mustID()
	}

	fm := noteFrontMatter{
		ID:        note.ID,
		Title:     note.Title,
		CreatedAt: note.CreatedAt.UTC(),
		Tags:      note.Tags,
		Metadata:  note.Metadata,
	}
	data, err := yaml.Marshal(fm)
	if err != nil {
		return note, fmt.Errorf("marshal frontmatter: %w", err)
	}

	var builder strings.Builder
	builder.WriteString("---\n")
	builder.Write(data)
	builder.WriteString("---\n\n")
	builder.WriteString(strings.TrimSpace(note.Content))
	builder.WriteString("\n")

	filename := filepath.Join(s.dir, fmt.Sprintf("%s-%s.md", note.CreatedAt.UTC().Format("20060102T150405Z"), safeFileName(note.Title)))
	if err := os.WriteFile(filename, []byte(builder.String()), 0o644); err != nil {
		return note, fmt.Errorf("write note: %w", err)
	}
	return note, nil
}

func (s *MarkdownStore) Recent(limit int) ([]Note, error) {
	notes, err := s.loadAll()
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(notes) > limit {
		return notes[:limit], nil
	}
	return notes, nil
}

func (s *MarkdownStore) Search(query string, limit int) ([]Note, error) {
	notes, err := s.loadAll()
	if err != nil {
		return nil, err
	}
	if query == "" {
		if limit > 0 && len(notes) > limit {
			return notes[:limit], nil
		}
		return notes, nil
	}

	lq := strings.ToLower(query)
	type scored struct {
		note  Note
		score int
	}
	var scoredNotes []scored
	for _, note := range notes {
		content := strings.ToLower(note.Title + " " + note.Content)
		score := 0
		for _, token := range strings.Fields(lq) {
			if strings.Contains(content, token) {
				score++
			}
		}
		if score > 0 {
			note.Source = "markdown"
			note.Score = float64(score)
			scoredNotes = append(scoredNotes, scored{note: note, score: score})
		}
	}

	sort.Slice(scoredNotes, func(i, j int) bool {
		if scoredNotes[i].score == scoredNotes[j].score {
			return scoredNotes[i].note.CreatedAt.After(scoredNotes[j].note.CreatedAt)
		}
		return scoredNotes[i].score > scoredNotes[j].score
	})

	results := make([]Note, 0, len(scoredNotes))
	for _, item := range scoredNotes {
		results = append(results, item.note)
		if limit > 0 && len(results) >= limit {
			break
		}
	}
	return results, nil
}

func (s *MarkdownStore) loadAll() ([]Note, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var notes []Note
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		note, err := s.readFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		notes = append(notes, note)
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].CreatedAt.After(notes[j].CreatedAt)
	})
	return notes, nil
}

func (s *MarkdownStore) readFile(path string) (Note, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Note{}, err
	}
	content := string(data)
	const delimiter = "---"
	parts := strings.SplitN(content, delimiter, 3)
	if len(parts) < 3 {
		return Note{}, fmt.Errorf("invalid markdown note: %s", path)
	}

	var fm noteFrontMatter
	if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
		return Note{}, fmt.Errorf("parse frontmatter: %w", err)
	}
	noteContent := strings.TrimSpace(parts[2])
	note := Note{
		ID:        fm.ID,
		Title:     fm.Title,
		Content:   noteContent,
		Tags:      fm.Tags,
		Metadata:  fm.Metadata,
		CreatedAt: fm.CreatedAt,
		Source:    "markdown",
	}
	if note.CreatedAt.IsZero() {
		info, statErr := os.Stat(path)
		if statErr == nil {
			note.CreatedAt = info.ModTime()
		}
	}
	return note, nil
}

var nonAlphaNumeric = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func safeFileName(title string) string {
	if title == "" {
		return "note"
	}
	slug := nonAlphaNumeric.ReplaceAllString(strings.ToLower(title), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "note"
	}
	return slug
}

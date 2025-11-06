package tools

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/mcp-immich/pkg/immich"
)

const defaultSmartAlbumStorage = "data/smart_albums.json"

// SmartAlbumDefinition represents a persistent smart album rule definition.
type SmartAlbumDefinition struct {
	ID               string                   `json:"id"`
	Name             string                   `json:"name"`
	Description      string                   `json:"description,omitempty"`
	AlbumID          string                   `json:"albumId"`
	AlbumName        string                   `json:"albumName"`
	AlbumDescription string                   `json:"albumDescription,omitempty"`
	Query            immich.SmartSearchParams `json:"query"`
	MaxResults       int                      `json:"maxResults,omitempty"`
	CreatedAt        time.Time                `json:"createdAt"`
	UpdatedAt        time.Time                `json:"updatedAt"`
	LastRunAt        *time.Time               `json:"lastRunAt,omitempty"`
	LastResultCount  int                      `json:"lastResultCount,omitempty"`
	LastAddedCount   int                      `json:"lastAddedCount,omitempty"`
	LastRunError     string                   `json:"lastRunError,omitempty"`
}

// SmartAlbumStore manages smart album definitions persisted on disk.
type SmartAlbumStore struct {
	mu     sync.RWMutex
	path   string
	albums map[string]SmartAlbumDefinition
	byName map[string]string
	loaded bool
}

// NewSmartAlbumStore creates a new store instance backed by the provided file path.
func NewSmartAlbumStore(path string) (*SmartAlbumStore, error) {
	if path == "" {
		path = defaultSmartAlbumStorage
	}

	store := &SmartAlbumStore{
		path:   path,
		albums: make(map[string]SmartAlbumDefinition),
		byName: make(map[string]string),
	}

	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

// Path returns the backing file path.
func (s *SmartAlbumStore) Path() string {
	return s.path
}

// load loads definitions from disk if present.
func (s *SmartAlbumStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loaded {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.loaded = true
			return nil
		}
		return err
	}

	if len(data) == 0 {
		s.loaded = true
		return nil
	}

	var defs []SmartAlbumDefinition
	if err := json.Unmarshal(data, &defs); err != nil {
		return err
	}

	for _, def := range defs {
		s.albums[def.ID] = def
		if def.Name != "" {
			s.byName[strings.ToLower(def.Name)] = def.ID
		}
	}

	s.loaded = true
	return nil
}

// Save persists the definition, assigning IDs and timestamps as needed.
func (s *SmartAlbumStore) Save(def SmartAlbumDefinition) (SmartAlbumDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if def.ID == "" {
		def.ID = uuid.NewString()
	}

	now := time.Now().UTC()
	if def.CreatedAt.IsZero() {
		def.CreatedAt = now
	}
	def.UpdatedAt = now

	s.albums[def.ID] = def
	if def.Name != "" {
		s.byName[strings.ToLower(def.Name)] = def.ID
	}

	if err := s.persistLocked(); err != nil {
		return SmartAlbumDefinition{}, err
	}

	return def, nil
}

// GetByID retrieves a definition by its ID.
func (s *SmartAlbumStore) GetByID(id string) (SmartAlbumDefinition, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	def, ok := s.albums[id]
	return def, ok
}

// GetByName retrieves a definition by its name (case-insensitive).
func (s *SmartAlbumStore) GetByName(name string) (SmartAlbumDefinition, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if name == "" {
		return SmartAlbumDefinition{}, false
	}

	id, ok := s.byName[strings.ToLower(name)]
	if !ok {
		return SmartAlbumDefinition{}, false
	}

	def, ok := s.albums[id]
	return def, ok
}

// List returns all stored definitions sorted by name.
func (s *SmartAlbumStore) List() []SmartAlbumDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	defs := make([]SmartAlbumDefinition, 0, len(s.albums))
	for _, def := range s.albums {
		defs = append(defs, def)
	}

	sort.Slice(defs, func(i, j int) bool {
		return strings.ToLower(defs[i].Name) < strings.ToLower(defs[j].Name)
	})

	return defs
}

// persistLocked writes the current definitions to disk. Caller must hold write lock.
func (s *SmartAlbumStore) persistLocked() error {
	defs := make([]SmartAlbumDefinition, 0, len(s.albums))
	for _, def := range s.albums {
		defs = append(defs, def)
	}

	sort.Slice(defs, func(i, j int) bool {
		return strings.ToLower(defs[i].Name) < strings.ToLower(defs[j].Name)
	})

	data, err := json.MarshalIndent(defs, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmpPath, s.path)
}

// Delete removes a definition by ID.
func (s *SmartAlbumStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	def, ok := s.albums[id]
	if !ok {
		return nil
	}

	delete(s.albums, id)
	if def.Name != "" {
		delete(s.byName, strings.ToLower(def.Name))
	}

	return s.persistLocked()
}

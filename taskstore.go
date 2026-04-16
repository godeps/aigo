package aigo

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrStoreNotConfigured = errors.New("aigo: task store not configured")
	ErrTaskNotFound       = errors.New("aigo: task not found")
	ErrResumeNotSupported = errors.New("aigo: engine does not support resume")
)

// TaskStatus constants.
const (
	TaskStatusPending   = "pending"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
)

// TaskRecord represents a persisted async task.
type TaskRecord struct {
	ID         string    `json:"id"`
	EngineName string    `json:"engine"`
	RemoteID   string    `json:"remote_id"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ResultVal  string    `json:"result_value,omitempty"`
	ResultKind OutputKind `json:"result_kind,omitempty"`
	ErrMsg     string    `json:"error,omitempty"`
}

// TaskStore persists async task records for crash recovery.
type TaskStore interface {
	Save(record TaskRecord) error
	Load(id string) (TaskRecord, error)
	All() ([]TaskRecord, error)
	Delete(id string) error
}

// FileTaskStore implements TaskStore using a single JSON file.
type FileTaskStore struct {
	mu   sync.Mutex
	path string
}

// NewFileTaskStore creates a file-backed task store at the given path.
func NewFileTaskStore(path string) (*FileTaskStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("aigo: create task store dir: %w", err)
	}
	return &FileTaskStore{path: path}, nil
}

// DefaultFileTaskStore creates a file-backed task store at .aigo/tasks.json
// relative to the current working directory.
func DefaultFileTaskStore() (*FileTaskStore, error) {
	return NewFileTaskStore(filepath.Join(".aigo", "tasks.json"))
}

func (s *FileTaskStore) Save(record TaskRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.readAll()
	if err != nil {
		return err
	}

	record.UpdatedAt = time.Now()
	found := false
	for i, r := range records {
		if r.ID == record.ID {
			records[i] = record
			found = true
			break
		}
	}
	if !found {
		records = append(records, record)
	}

	return s.writeAll(records)
}

func (s *FileTaskStore) Load(id string) (TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.readAll()
	if err != nil {
		return TaskRecord{}, err
	}

	for _, r := range records {
		if r.ID == id {
			return r, nil
		}
	}
	return TaskRecord{}, fmt.Errorf("%w: %s", ErrTaskNotFound, id)
}

func (s *FileTaskStore) All() ([]TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readAll()
}

func (s *FileTaskStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.readAll()
	if err != nil {
		return err
	}

	filtered := records[:0]
	for _, r := range records {
		if r.ID != id {
			filtered = append(filtered, r)
		}
	}

	return s.writeAll(filtered)
}

// Purge removes completed and failed records older than maxAge.
func (s *FileTaskStore) Purge(maxAge time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.readAll()
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-maxAge)
	filtered := records[:0]
	for _, r := range records {
		if r.Status == TaskStatusPending || r.UpdatedAt.After(cutoff) {
			filtered = append(filtered, r)
		}
	}

	return s.writeAll(filtered)
}

// readAll reads the JSON file. Returns empty slice if file does not exist.
// Caller must hold s.mu.
func (s *FileTaskStore) readAll() ([]TaskRecord, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("aigo: read task store: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	var records []TaskRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("aigo: decode task store: %w", err)
	}
	return records, nil
}

// writeAll writes all records to the JSON file atomically (tmp + rename).
// Caller must hold s.mu.
func (s *FileTaskStore) writeAll(records []TaskRecord) error {
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("aigo: encode task store: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("aigo: write task store tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("aigo: rename task store: %w", err)
	}
	return nil
}

// newTaskID generates a random 32-character hex ID.
func newTaskID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("aigo: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

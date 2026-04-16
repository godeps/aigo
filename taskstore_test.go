package aigo

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFileTaskStore_SaveAndLoad(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)

	rec := TaskRecord{
		ID:         "test-001",
		EngineName: "runway",
		RemoteID:   "remote-abc",
		Status:     TaskStatusPending,
		CreatedAt:  time.Now(),
	}

	if err := store.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Load("test-001")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.EngineName != "runway" || got.RemoteID != "remote-abc" || got.Status != TaskStatusPending {
		t.Errorf("Load returned wrong record: %+v", got)
	}
}

func TestFileTaskStore_UpdateExisting(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)

	rec := TaskRecord{
		ID:         "test-002",
		EngineName: "flux",
		RemoteID:   "remote-xyz",
		Status:     TaskStatusPending,
		CreatedAt:  time.Now(),
	}
	if err := store.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	rec.Status = TaskStatusCompleted
	rec.ResultVal = "https://example.com/result.png"
	rec.ResultKind = OutputURL
	if err := store.Save(rec); err != nil {
		t.Fatalf("Save update: %v", err)
	}

	got, err := store.Load("test-002")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Status != TaskStatusCompleted || got.ResultVal != "https://example.com/result.png" {
		t.Errorf("expected completed with URL, got: %+v", got)
	}
}

func TestFileTaskStore_Delete(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)

	rec := TaskRecord{ID: "test-003", Status: TaskStatusPending, CreatedAt: time.Now()}
	if err := store.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := store.Delete("test-003"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.Load("test-003")
	if err == nil {
		t.Fatal("expected ErrTaskNotFound after delete")
	}
}

func TestFileTaskStore_All(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)

	for i := 0; i < 5; i++ {
		rec := TaskRecord{
			ID:        newTaskID(),
			Status:    TaskStatusPending,
			CreatedAt: time.Now(),
		}
		if err := store.Save(rec); err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
	}

	all, err := store.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 5 {
		t.Errorf("expected 5 records, got %d", len(all))
	}
}

func TestFileTaskStore_Purge(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)

	old := TaskRecord{
		ID:        "old-001",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now().Add(-48 * time.Hour),
		UpdatedAt: time.Now().Add(-48 * time.Hour),
	}
	pending := TaskRecord{
		ID:        "pending-001",
		Status:    TaskStatusPending,
		CreatedAt: time.Now().Add(-48 * time.Hour),
		UpdatedAt: time.Now().Add(-48 * time.Hour),
	}
	recent := TaskRecord{
		ID:        "recent-001",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Write directly to bypass Save's UpdatedAt override.
	records := []TaskRecord{old, pending, recent}
	if err := store.writeAll(records); err != nil {
		t.Fatalf("writeAll: %v", err)
	}

	if err := store.Purge(24 * time.Hour); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	all, err := store.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}

	// pending-001 survives (pending always kept), recent-001 survives (within maxAge), old-001 purged.
	if len(all) != 2 {
		t.Errorf("expected 2 records after purge, got %d: %+v", len(all), all)
	}
}

func TestFileTaskStore_ConcurrentSave(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := TaskRecord{
				ID:        newTaskID(),
				Status:    TaskStatusPending,
				CreatedAt: time.Now(),
			}
			if err := store.Save(rec); err != nil {
				t.Errorf("concurrent Save: %v", err)
			}
		}()
	}
	wg.Wait()

	all, err := store.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 20 {
		t.Errorf("expected 20 records, got %d", len(all))
	}
}

func TestFileTaskStore_AtomicWrite(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)

	rec := TaskRecord{ID: "atomic-001", Status: TaskStatusPending, CreatedAt: time.Now()}
	if err := store.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}

	tmp := store.path + ".tmp"
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("expected no .tmp file after save, but found one")
	}
}

func TestNewTaskID(t *testing.T) {
	t.Parallel()
	id1 := newTaskID()
	id2 := newTaskID()
	if len(id1) != 32 {
		t.Errorf("expected 32 char ID, got %d: %s", len(id1), id1)
	}
	if id1 == id2 {
		t.Error("two IDs should not be equal")
	}
}

func newTestStore(t *testing.T) *FileTaskStore {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.json")
	store, err := NewFileTaskStore(path)
	if err != nil {
		t.Fatalf("NewFileTaskStore: %v", err)
	}
	return store
}

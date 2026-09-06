package data

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Migration represents a versioned DDL change
type Migration struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	UpSQL   string `json:"up_sql"`
	DownSQL string `json:"down_sql"`
}

// MigrationRecord represents an applied migration in database
type MigrationRecord struct {
	Version   int       `json:"version"`
	Name      string    `json:"name"`
	AppliedAt time.Time `json:"applied_at"`
}

// MigrationRunner executes and tracks database migrations
type MigrationRunner struct {
	migrations []Migration
	applied    map[int]MigrationRecord
	mu         sync.Mutex
}

// NewMigrationRunner creates a runner
func NewMigrationRunner() *MigrationRunner {
	return &MigrationRunner{
		migrations: make([]Migration, 0),
		applied:    make(map[int]MigrationRecord),
	}
}

// Register registers a migration
func (mr *MigrationRunner) Register(m Migration) *MigrationRunner {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.migrations = append(mr.migrations, m)
	sort.Slice(mr.migrations, func(i, j int) bool {
		return mr.migrations[i].Version < mr.migrations[j].Version
	})
	return mr
}

// Up runs all unapplied migrations in version order
func (mr *MigrationRunner) Up() ([]int, error) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	appliedVersions := make([]int, 0)
	for _, m := range mr.migrations {
		if _, exists := mr.applied[m.Version]; !exists {
			// Simulate execution of UpSQL
			if m.UpSQL == "" {
				return nil, fmt.Errorf("migration %d (%s) missing UpSQL", m.Version, m.Name)
			}
			mr.applied[m.Version] = MigrationRecord{
				Version:   m.Version,
				Name:      m.Name,
				AppliedAt: time.Now(),
			}
			appliedVersions = append(appliedVersions, m.Version)
		}
	}

	return appliedVersions, nil
}

// Down rolls back the latest applied migration
func (mr *MigrationRunner) Down() (int, error) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if len(mr.applied) == 0 {
		return 0, fmt.Errorf("no applied migrations to roll back")
	}

	// Find highest applied version
	maxVer := -1
	for v := range mr.applied {
		if v > maxVer {
			maxVer = v
		}
	}

	delete(mr.applied, maxVer)
	return maxVer, nil
}

// Status returns current migration status
func (mr *MigrationRunner) Status() []MigrationRecord {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	res := make([]MigrationRecord, 0, len(mr.applied))
	for _, rec := range mr.applied {
		res = append(res, rec)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Version < res[j].Version
	})
	return res
}

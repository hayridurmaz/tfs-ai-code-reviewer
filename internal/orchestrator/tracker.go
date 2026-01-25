package orchestrator

import (
	"fmt"
	"sync"
)

type Tracker struct {
	mu         sync.Mutex
	inProgress map[string]bool
}

func NewTracker() *Tracker {
	return &Tracker{
		inProgress: make(map[string]bool),
	}
}

func (t *Tracker) Lock(repoID string, prID, iterationID int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := fmt.Sprintf("%s-%d-%d", repoID, prID, iterationID)
	if t.inProgress[key] {
		return false
	}
	t.inProgress[key] = true
	return true
}

func (t *Tracker) Unlock(repoID string, prID, iterationID int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := fmt.Sprintf("%s-%d-%d", repoID, prID, iterationID)
	delete(t.inProgress, key)
}

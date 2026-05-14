package main

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

// TestPoWStore_NoMassWipe verifies the regression that motivated this change:
// the old store wiped usedSolutions wholesale once it crossed 10K entries,
// briefly opening a replay window. The LRU-backed version evicts the
// least-recently-used entries instead, so a recent entry is still rejected
// after sustained churn.
func TestPoWStore_NoMassWipe(t *testing.T) {
	s := newPoWChallengeStore()

	// Mark a known-recent solution.
	const recent = "recent:1"
	if !s.MarkSolutionUsed(recent) {
		t.Fatalf("first MarkSolutionUsed should succeed")
	}

	// Push enough churn to trigger the old wipe threshold.
	for i := 0; i < 20_000; i++ {
		s.MarkSolutionUsed(fmt.Sprintf("churn:%d", i))
	}

	// The recent solution must still be considered used. Old code would have
	// returned false here (wiped map -> replay accepted).
	if !s.IsSolutionUsed(recent) {
		t.Fatalf("recent solution wiped after churn — LRU regression")
	}
	if s.MarkSolutionUsed(recent) {
		t.Fatalf("recent solution accepted as new after churn — replay window")
	}
}

// TestPoWStore_LRUEviction verifies that under sustained pressure the cap is
// honored and old entries are evicted (not new ones).
func TestPoWStore_LRUEviction(t *testing.T) {
	// Override the package-level cap with a small one for the test by
	// constructing a store directly.
	s := &PoWChallengeStore{
		challenges:    make(map[string]*PoWChallenge),
		usedSolutions: expirable.NewLRU[string, struct{}](16, nil, time.Hour),
	}

	const oldKey = "old:1"
	if !s.MarkSolutionUsed(oldKey) {
		t.Fatalf("first insert failed")
	}

	for i := 0; i < 100; i++ {
		s.MarkSolutionUsed(fmt.Sprintf("k:%d", i))
	}

	if s.IsSolutionUsed(oldKey) {
		t.Fatalf("oldest entry not evicted under cap pressure")
	}

	// Most recent entry must still be present.
	if !s.IsSolutionUsed("k:99") {
		t.Fatalf("most-recent entry evicted — LRU policy inverted")
	}
}

// TestTokenStore_TTLExpiry verifies that entries naturally expire and free
// their cap slot rather than living forever as the old impl did until the
// inline cleanup happened to fire.
func TestTokenStore_TTLExpiry(t *testing.T) {
	s := &TokenStore{
		cache: expirable.NewLRU[string, struct{}](100, nil, 50*time.Millisecond),
	}

	const sig = "tok:abc"
	if !s.MarkUsed(sig) {
		t.Fatalf("first MarkUsed should succeed")
	}
	if !s.IsUsed(sig) {
		t.Fatalf("token immediately reported as unused")
	}

	time.Sleep(120 * time.Millisecond)

	if s.IsUsed(sig) {
		t.Fatalf("token still reported used after TTL")
	}
	if !s.MarkUsed(sig) {
		t.Fatalf("post-expiry MarkUsed should succeed (slot freed)")
	}
}

// TestTokenStore_ConcurrentMarkUsed exercises the test-and-set under
// contention. Exactly one of N goroutines hammering the same signature must
// see the "newly inserted" return; the rest must see "replay".
func TestTokenStore_ConcurrentMarkUsed(t *testing.T) {
	s := newTokenStore()
	const goroutines = 64
	const sig = "race:tok"

	var wg sync.WaitGroup
	wg.Add(goroutines)

	winners := make(chan struct{}, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if s.MarkUsed(sig) {
				winners <- struct{}{}
			}
		}()
	}
	wg.Wait()
	close(winners)

	count := 0
	for range winners {
		count++
	}
	if count != 1 {
		t.Fatalf("MarkUsed not atomic under contention: %d winners (want 1)", count)
	}
}

// TestPoWStore_DeleteChallenge verifies the now-locked DeleteChallenge no
// longer races with cleanup or other writers.
func TestPoWStore_DeleteChallenge(t *testing.T) {
	s := newPoWChallengeStore()
	id := "ch-1"
	s.mu.Lock()
	s.challenges[id] = &PoWChallenge{ID: id, ExpiresAt: time.Now().Add(time.Minute).UnixMilli()}
	s.mu.Unlock()

	s.DeleteChallenge(id)

	s.mu.RLock()
	_, exists := s.challenges[id]
	s.mu.RUnlock()
	if exists {
		t.Fatalf("DeleteChallenge did not remove entry")
	}
}

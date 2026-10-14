package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"software-exam/config"
	"software-exam/pkg/redisclient"
)

type memEntry struct {
	value  int64
	expire time.Time
}

type memoryLoginLockStore struct {
	mu   sync.Mutex
	data map[string]memEntry
	now  time.Time
}

func newMemoryLoginLockStore() *memoryLoginLockStore {
	return &memoryLoginLockStore{
		data: make(map[string]memEntry),
		now:  time.Now(),
	}
}

func (m *memoryLoginLockStore) current() time.Time {
	if m.now.IsZero() {
		return time.Now()
	}
	return m.now
}

func (m *memoryLoginLockStore) purgeLocked(key string) {
	if e, ok := m.data[key]; ok && !e.expire.IsZero() && !e.expire.After(m.current()) {
		delete(m.data, key)
	}
}

func (m *memoryLoginLockStore) IncrFail(ctx context.Context, failKey, lockKey string, windowSec, maxFails int) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.purgeLocked(failKey)
	m.purgeLocked(lockKey)

	e := m.data[failKey]
	e.value++
	if e.value == 1 || e.expire.IsZero() {
		e.expire = m.current().Add(time.Duration(windowSec) * time.Second)
	}
	m.data[failKey] = e

	if e.value >= int64(maxFails) {
		m.data[lockKey] = memEntry{value: 1, expire: m.current().Add(time.Duration(windowSec) * time.Second)}
	}
	return e.value, nil
}

func (m *memoryLoginLockStore) TTL(ctx context.Context, key string) (time.Duration, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.purgeLocked(key)
	e, ok := m.data[key]
	if !ok {
		return 0, false, nil
	}
	if e.expire.IsZero() {
		return time.Second, true, nil
	}
	return e.expire.Sub(m.current()), true, nil
}

func (m *memoryLoginLockStore) Del(ctx context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

func setupLoginLockTest(t *testing.T) (*LoginLockService, *memoryLoginLockStore) {
	t.Helper()
	store := newMemoryLoginLockStore()
	config.AppConfig.AuthLock = config.AuthLockConfig{
		Enable:       true,
		MaxFailsUser: 3,
		MaxFailsIP:   5,
		WindowSec:    60,
	}
	t.Cleanup(func() {
		redisclient.Client = nil
		config.AppConfig.AuthLock = config.AuthLockConfig{}
	})
	return NewLoginLockServiceWithStore(store), store
}

func TestLoginLockFailOpenWithoutStore(t *testing.T) {
	redisclient.Client = nil
	config.AppConfig.AuthLock = config.AuthLockConfig{Enable: true, MaxFailsUser: 3, MaxFailsIP: 5, WindowSec: 60}
	svc := NewLoginLockService()

	if locked, _ := svc.Check("alice", "1.1.1.1"); locked {
		t.Fatal("expected fail-open when store/redis is nil")
	}
	if justLocked, _ := svc.RecordFailure("alice", "1.1.1.1"); justLocked {
		t.Fatal("expected no lock when store/redis is nil")
	}
	svc.ClearUser("alice")
}

func TestLoginLockDisabled(t *testing.T) {
	svc, _ := setupLoginLockTest(t)
	config.AppConfig.AuthLock.Enable = false

	for i := 0; i < 10; i++ {
		svc.RecordFailure("alice", "1.1.1.1")
	}
	if locked, _ := svc.Check("alice", "1.1.1.1"); locked {
		t.Fatal("expected no lock when disabled")
	}
}

func TestLoginLockUserThreshold(t *testing.T) {
	svc, _ := setupLoginLockTest(t)

	for i := 0; i < 2; i++ {
		justLocked, _ := svc.RecordFailure("Alice", "1.1.1.1")
		if justLocked {
			t.Fatalf("should not lock before threshold, attempt %d", i+1)
		}
		if locked, _ := svc.Check("alice", "1.1.1.1"); locked {
			t.Fatalf("should not be locked before threshold, attempt %d", i+1)
		}
	}

	justLocked, retryAfter := svc.RecordFailure("ALICE", "1.1.1.1")
	if !justLocked {
		t.Fatal("expected lock on 3rd failure")
	}
	if retryAfter <= 0 {
		t.Fatalf("expected positive retryAfter, got %d", retryAfter)
	}

	locked, ra := svc.Check(" alice ", "9.9.9.9")
	if !locked {
		t.Fatal("expected username-normalized lock to hit")
	}
	if ra <= 0 {
		t.Fatalf("expected positive retryAfter on check, got %d", ra)
	}
}

func TestLoginLockClearsUserOnSuccess(t *testing.T) {
	svc, _ := setupLoginLockTest(t)

	for i := 0; i < 3; i++ {
		svc.RecordFailure("bob", "2.2.2.2")
	}
	if locked, _ := svc.Check("bob", "8.8.8.8"); !locked {
		t.Fatal("expected user locked")
	}

	svc.ClearUser("Bob")
	if locked, _ := svc.Check("bob", "8.8.8.8"); locked {
		t.Fatal("expected user lock cleared")
	}

	svc.RecordFailure("other1", "2.2.2.2")
	justLocked, _ := svc.RecordFailure("other2", "2.2.2.2")
	if !justLocked {
		t.Fatal("expected IP lock to remain progressive after ClearUser")
	}
	if locked, _ := svc.Check("someone", "2.2.2.2"); !locked {
		t.Fatal("expected IP still lockable / locked")
	}
}

func TestLoginLockIPThreshold(t *testing.T) {
	svc, _ := setupLoginLockTest(t)

	for i := 0; i < 4; i++ {
		svc.RecordFailure(fmt.Sprintf("user%d", i), "3.3.3.3")
	}
	if locked, _ := svc.Check("fresh", "3.3.3.3"); locked {
		t.Fatal("IP should not lock before threshold")
	}
	justLocked, _ := svc.RecordFailure("u4", "3.3.3.3")
	if !justLocked {
		t.Fatal("expected IP lock on 5th failure")
	}
	if locked, _ := svc.Check("fresh", "3.3.3.3"); !locked {
		t.Fatal("expected IP locked for any username")
	}
}

func TestLoginLockDefaults(t *testing.T) {
	config.AppConfig.AuthLock = config.AuthLockConfig{Enable: true}
	cfg := lockCfg()
	if cfg.MaxFailsUser != 5 || cfg.MaxFailsIP != 20 || cfg.WindowSec != 900 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoginLockTTLExpires(t *testing.T) {
	svc, store := setupLoginLockTest(t)
	for i := 0; i < 3; i++ {
		svc.RecordFailure("carol", "4.4.4.4")
	}
	if locked, _ := svc.Check("carol", "4.4.4.4"); !locked {
		t.Fatal("expected locked before expire")
	}
	store.now = store.now.Add(61 * time.Second)
	if locked, _ := svc.Check("carol", "4.4.4.4"); locked {
		t.Fatal("expected lock expired after window")
	}
}

package service

import (
	"bytes"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// stubSiteRepo 内存版仓储。
//
// 自带锁：生产里的仓储（GORM + 连接池）本身对并发调用是安全的，而这个桩的
// 字段是会跨 goroutine 共享的 —— SetMaintenance 刻意在**拿锁之前**就调
// repo.UpdateMaintenance（写库不该占着读缓存的锁），所以写入者的 repo 调用
// 与读者的 repo 调用天然并发。桩不加锁的话，-race 会把桩自己的数据竞争
// 报成被测代码的竞争，把「单飞」这条断言淹掉。
type stubSiteRepo struct {
	mu      sync.Mutex
	enabled bool
	err     error
	reads   int
	writes  int
}

func (s *stubSiteRepo) MaintenanceEnabled() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads++
	if s.err != nil {
		return false, s.err
	}
	return s.enabled, nil
}

func (s *stubSiteRepo) UpdateMaintenance(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes++
	if s.err != nil {
		return s.err
	}
	s.enabled = enabled
	return nil
}

// TTL 内重复读只查一次库 —— 维护中间件每个请求都会调它，
// 不缓存就是每请求一次 DB 往返。
func TestSiteServiceCachesWithinTTL(t *testing.T) {
	repo := &stubSiteRepo{enabled: true}
	svc := NewSiteService(repo)

	for i := 0; i < 5; i++ {
		if !svc.MaintenanceEnabled(t.Context()) {
			t.Fatal("should report maintenance enabled")
		}
	}
	if repo.reads != 1 {
		t.Fatalf("expected 1 db read within ttl, got %d", repo.reads)
	}
}

// 缓存过期后必须重查 —— 否则开关在别的实例上被改掉，本实例永远看不到。
func TestSiteServiceRefetchesAfterTTL(t *testing.T) {
	repo := &stubSiteRepo{enabled: false}
	svc := NewSiteService(repo)
	svc.ttl = 0 // 让每次读都算过期，测试不必真 sleep 5 秒

	svc.MaintenanceEnabled(t.Context())
	repo.enabled = true
	if !svc.MaintenanceEnabled(t.Context()) {
		t.Fatal("should pick up the new value after ttl expiry")
	}
	if repo.reads != 2 {
		t.Fatalf("expected 2 db reads, got %d", repo.reads)
	}
}

// 写入后缓存必须立刻生效（而不是 5 秒后才变）。
// 管理员点完开关去无痕窗口验收，看到旧的会以为没生效。
func TestSiteServiceSetTakesEffectImmediately(t *testing.T) {
	repo := &stubSiteRepo{enabled: true}
	svc := NewSiteService(repo)
	// 这里刻意 **不** 把 ttl 调成 0（与相邻两个用例相反）：本用例验的是
	// 「写入把缓存置成了新值」，不是「过期后重查」。ttl=0 会让下面每次读都
	// 必然回查一次库，reads 恒为 2，反而验不出缓存有没有被写入刷新 ——
	// 那正是本用例唯一要证明的事。

	if !svc.MaintenanceEnabled(t.Context()) {
		t.Fatal("precondition: should start enabled")
	}
	if err := svc.SetMaintenance(false); err != nil {
		t.Fatal(err)
	}
	if svc.MaintenanceEnabled(t.Context()) {
		t.Fatal("should be disabled right after write")
	}
	if repo.reads != 1 {
		t.Fatalf("write should refresh the cache, not force another read; got %d reads", repo.reads)
	}
}

// 写完缓存直接置成新值，而不是清掉缓存等下次回读 —— 少一次 DB 往返，
// 也避免「写成功、下次读因 DB 抖动失败」时读到旧值。
func TestSiteServiceSetFailureLeavesCacheUntouched(t *testing.T) {
	repo := &stubSiteRepo{enabled: false}
	svc := NewSiteService(repo)

	repo.err = errors.New("connection refused")
	if err := svc.SetMaintenance(true); err == nil {
		t.Fatal("write error must surface, admin needs to know it did not take")
	}
	if svc.MaintenanceEnabled(t.Context()) {
		t.Fatal("cache must not change when the write failed")
	}
}

// DB 读不到、但之前读到过 → 沿用上次的值（保守沿用已知意图）。
func TestSiteServiceKeepsLastKnownValueOnDBError(t *testing.T) {
	repo := &stubSiteRepo{enabled: true}
	svc := NewSiteService(repo)
	svc.ttl = 0

	if !svc.MaintenanceEnabled(t.Context()) {
		t.Fatal("precondition: should start enabled")
	}
	repo.err = errors.New("connection refused")
	if !svc.MaintenanceEnabled(t.Context()) {
		t.Fatal("must fall back to the last known value, not silently exit maintenance")
	}
}

// DB 读不到、且从没读到过 → 放行。
// fail-closed 会把一次数据库抖动放大成整站维护页，而这时候管理员
// 也进不去后台把它关掉——等于把自己锁在门外。
//
// 关键是**放行一次不够**：这次失败必须也被负缓存。不缓存的话 hasCache 永远
// 是 false，之后每个请求都会重新掉进锁内那次查询 —— DB 卡住（而不是拒绝连接）
// 时，每个请求都串行堵在锁上直到驱动超时。日志同理，应当每 TTL 一条。
func TestSiteServiceFailsOpenWhenNeverRead(t *testing.T) {
	repo := &stubSiteRepo{err: errors.New("connection refused")}
	svc := NewSiteService(repo)
	svc.ttl = time.Second

	// 捕获标准 logger，数一数失败日志打了几条
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	if svc.MaintenanceEnabled(t.Context()) {
		t.Fatal("must fail open when the flag was never readable")
	}
	if repo.reads != 1 {
		t.Fatalf("expected 1 db read, got %d", repo.reads)
	}

	// 紧接着的第二次读必须走负缓存：不再查库、也不再打日志。
	// 没有负缓存的话 hasCache 仍是 false，它会原样再走一遍锁内查询，reads 变成 2。
	if svc.MaintenanceEnabled(t.Context()) {
		t.Fatal("must keep failing open")
	}
	if repo.reads != 1 {
		t.Fatalf("从没读到过时的失败也必须负缓存；期望仍是 1 次查询，实际 %d 次", repo.reads)
	}
	if n := strings.Count(logs.String(), "[site]"); n != 1 {
		t.Fatalf("失败日志应每 TTL 一条（1 条），实际 %d 条：%s", n, logs.String())
	}
}

// 过期瞬间的并发读必须被 mutex 收口成一次回查 —— 这把锁存在的唯一理由，
// 单 goroutine 的用例（哪怕开着 -race）证明不了任何事。
//
// 可证伪性（实测过）：删掉 MaintenanceEnabled 里的 s.mu 加解锁，
// 不开 -race 时 repo.reads 从 ≤2 涨到接近 readers+1（`reads > 2` 失败），
// 开 -race 时 repo.reads++ 与 cached/cachedAt/hasCache 的并发读写直接报数据竞争。
// 两种跑法都拦得住，不会「删了锁还照样绿」。
func TestSiteServiceConcurrentReadsAreSingleFlight(t *testing.T) {
	repo := &stubSiteRepo{enabled: false}
	svc := NewSiteService(repo)
	// TTL 取得比整批 goroutine 的生命周期长得多：「先抢到锁的那一个回查、
	// 其余人全吃缓存」于是是确定性的，而不是跟调度器赛跑。
	svc.ttl = 2 * time.Second

	if svc.MaintenanceEnabled(t.Context()) {
		t.Fatal("precondition: 开关初始应为关闭")
	}
	// 人为把缓存时间戳推回过去，制造「所有读者同时看到缓存已过期」的起点 ——
	// 那正是单飞要收口的瞬间。此刻还没有并发，直接写字段是安全的。
	svc.mu.Lock()
	svc.cachedAt = time.Now().Add(-time.Minute)
	svc.mu.Unlock()

	const readers = 50
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(readers + 1)
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			<-start
			svc.MaintenanceEnabled(t.Context())
		}()
	}
	// 并发的写入者：它与读者抢同一把锁，但不能把读者的回查拆成多次
	go func() {
		defer wg.Done()
		if err := svc.SetMaintenance(true); err != nil {
			t.Errorf("SetMaintenance: %v", err)
		}
	}()
	close(start)
	wg.Wait()

	// 预置的 1 次 + 过期后最多 1 次回查 = 2。收口失败时会接近 readers+1。
	if repo.reads > 2 {
		t.Fatalf("%d 个并发读应被收口成 1 次回查（合计最多 2 次），实际 %d 次 —— 单飞没生效",
			readers, repo.reads)
	}
	if repo.writes != 1 {
		t.Fatalf("expected exactly 1 write, got %d", repo.writes)
	}
	// 写入必须被后续读观察到（写者已经把缓存置成新值）
	if !svc.MaintenanceEnabled(t.Context()) {
		t.Fatal("写入后必须观察到新值")
	}
}

// DB 读不到时，缓存时间戳也必须刷新（负缓存）。
// 不刷新的话缓存从此永远「已过期」，之后每个请求都会重新掉进那次查询 ——
// 而它在锁里。DB 卡住时每个请求都串行堵在锁上，正是 fail-open 想避免的
// 「一次抖动拖垮整站」。
func TestSiteServiceDoesNotRequeryEveryRequestDuringDBOutage(t *testing.T) {
	repo := &stubSiteRepo{enabled: true}
	svc := NewSiteService(repo)
	// 用一个小而真实的 TTL：本用例要观察「过期→失败→刷新时间戳→下次走缓存」
	// 这一整条路径，需要一个能真的过期、又不至于让测试睡太久的窗口。
	svc.ttl = 100 * time.Millisecond

	if !svc.MaintenanceEnabled(t.Context()) {
		t.Fatal("precondition: should start enabled")
	}
	if repo.reads != 1 {
		t.Fatalf("precondition: expected 1 read, got %d", repo.reads)
	}

	time.Sleep(120 * time.Millisecond) // 让缓存过期
	repo.err = errors.New("i/o timeout")
	if !svc.MaintenanceEnabled(t.Context()) {
		t.Fatal("must keep serving the last known value")
	}
	if repo.reads != 2 {
		t.Fatalf("expired cache should re-query once, got %d reads", repo.reads)
	}

	// 关键的一步：紧接着的这次读必须走缓存，不能再查库。
	// 没有负缓存的话它会再次掉进那次查询，reads 变成 3。
	if !svc.MaintenanceEnabled(t.Context()) {
		t.Fatal("must keep serving the last known value")
	}
	if repo.reads != 2 {
		t.Fatalf("a failing db must not be re-queried on every request; got %d reads", repo.reads)
	}
}

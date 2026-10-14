package service

import (
	"context"
	"log"
	"sync"
	"time"

	"software-exam/internal/domain/repository"
)

// siteCacheTTL 维护开关的进程内缓存时长。
// 这个值直接决定「开关打开后，别的实例最多多久跟上」，多实例部署时就是这个不一致窗口。
const siteCacheTTL = 5 * time.Second

// SiteService 站点级开关。
//
// 维护中间件挂在全局，每个请求都会问一次「在维护吗」。直接查库等于给每个请求
// 加一次 DB 往返，所以这里做进程内缓存：TTL 内只读内存，过期后才回查。
type SiteService struct {
	repo repository.SiteSettingsRepository

	mu       sync.Mutex
	cached   bool
	cachedAt time.Time
	hasCache bool

	// ttl 抽成字段而非直接用常量：TTL 相关的测试要能立刻制造过期，
	// 否则每个用例都得真 sleep 5 秒。生产路径永远用 siteCacheTTL。
	ttl time.Duration
}

func NewSiteService(repo repository.SiteSettingsRepository) *SiteService {
	return &SiteService{repo: repo, ttl: siteCacheTTL}
}

// MaintenanceEnabled 当前是否处于维护中。
//
// 永不返回错误：调用方是全局中间件，它必须有个确定的答案。读不到库时的取舍见下面注释。
func (s *SiteService) MaintenanceEnabled(_ context.Context) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.hasCache && time.Since(s.cachedAt) < s.ttl {
		return s.cached
	}

	// 整个「查库 + 更新缓存」在锁内完成：缓存过期的瞬间多个 goroutine 会同时冲进来，
	// 锁把它们收口成一次查询。单行主键查询很快，串行的代价远小于每 5 秒一轮惊群。
	enabled, err := s.repo.MaintenanceEnabled()
	if err != nil {
		// 有上次的值就沿用：保守沿用已知意图，比凭空猜一个更接近管理员的本意。
		if s.hasCache {
			// 时间戳必须一并刷新（负缓存）。不刷新的话缓存从此永远处于「已过期」，
			// 之后每个请求都会重新掉进上面那次查询 —— 而它就在锁里。DB 卡住
			// （连接池耗尽、网络黑洞）时，每个请求都会串行堵在锁上直到驱动超时，
			// 整个进程不再服务任何请求。那正是 fail-open 想避免的「一次抖动拖垮
			// 整站」，只是换成了延迟的形态。
			// 代价：DB 恢复后最多晚一个 TTL 才重新探测到，与缓存本身的不一致
			// 窗口同量级，可以接受。
			s.cachedAt = time.Now()
			return s.cached
		}
		// 从没读到过：放行。开关自己就存在 DB 里，DB 挂了无从得知意图；
		// fail-closed 会把一次数据库抖动放大成整站维护页，而且此时管理员
		// 也进不去后台把它关掉——等于把自己锁在门外。
		//
		// 这次失败同样要负缓存（与上个分支同理）：不缓存的话 hasCache 永远是
		// false，下面每个请求都会重新掉进上面那次查询 —— 而它在锁里。冷启动
		// （刚发版/重启）正好撞上故障时，DB 若是在**卡住**而不是拒绝连接
		// （连接池耗尽、网络黑洞），每个请求都会串行堵在锁上直到驱动超时。
		// 返回的本来就是 fail-open 的 false，把它缓存下来是一致的。
		// 日志随之变成每 TTL 一条，而不是每请求一条。
		log.Printf("[site] 读取维护开关失败，按未维护放行: %v", err)
		s.cached, s.cachedAt, s.hasCache = false, time.Now(), true
		return false
	}

	s.cached, s.cachedAt, s.hasCache = enabled, time.Now(), true
	return enabled
}

// SetMaintenance 写维护开关，并把缓存直接置成新值。
//
// 不清缓存、等下次回读：那样一来管理员点完开关到生效之间有最长 5 秒的空窗，
// 二来「写成功但紧接着的回读撞上 DB 抖动」会读到旧值。直接置新值两个问题都没有。
func (s *SiteService) SetMaintenance(enabled bool) error {
	if err := s.repo.UpdateMaintenance(enabled); err != nil {
		return err
	}
	s.mu.Lock()
	s.cached, s.cachedAt, s.hasCache = enabled, time.Now(), true
	s.mu.Unlock()
	return nil
}

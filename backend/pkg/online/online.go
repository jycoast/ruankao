// Package online 在线用户统计。
//
// 思路：AuthMiddleware 里对每个已登录请求做一次 Redis ZSET 心跳
// （member=userID，score=活跃 unix 秒），"在线"定义为统计窗口内
// （默认 5 分钟）有过任意请求的登录用户。无 WebSocket，靠窗口过期
// 自然下线的近似方案 —— 看不到用户关闭标签页，但管理后台看趋势足够。
//
// Key 布局：
//
//	online:users   ZSET。读侧先 ZREMRANGEBYSCORE 清掉窗口外过期成员再取基数
//
// Redis 不可用时一律静默降级（fail-open），在线统计是锦上添花，
// 不值得为它拖慢请求或打断主业务。
package online

import (
	"context"
	"strconv"
	"time"

	"software-exam/pkg/redisclient"

	"github.com/redis/go-redis/v9"
)

const (
	// key 在线用户 ZSET。member=userID，score=最近活跃的 unix 秒。
	key = "online:users"

	// OnlineWindow 在线统计窗口。窗口内无请求即视为离线。
	OnlineWindow = 5 * time.Minute
)

// Heartbeat 记录一次已登录用户的活跃。AuthMiddleware 对每个请求调用。
// Redis 不可用或 userID 为空时静默忽略。
func Heartbeat(ctx context.Context, userID uint64) {
	if userID == 0 || redisclient.Client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	redisclient.Client.ZAdd(ctx, key, redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: userID,
	})
	// 写失败静默：统计失败不应打断正常请求
}

// Member 在线成员。Score 为最近一次活跃的 unix 秒（Heartbeat 每次请求刷新），
// 管理后台用来展示「N 秒前活跃」。
type Member struct {
	ID    uint64 `json:"id"`
	Score int64  `json:"score"`
}

// Snapshot 返回当前在线数（先清除窗口外过期成员）与最近活跃的至多 20 个成员
// （按活跃时间倒序）。Redis 不可用时返回 (0, nil, nil)：管理后台展示 0 而非报错。
func Snapshot(ctx context.Context) (int64, []Member, error) {
	rdb := redisclient.Client
	if rdb == nil {
		return 0, nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	// 窗口外的成员已离线，读侧顺手清掉，避免 ZSET 无限膨胀
	cutoff := strconv.FormatFloat(float64(time.Now().Add(-OnlineWindow).Unix()), 'f', 0, 64)
	if err := rdb.ZRemRangeByScore(ctx, key, "-inf", cutoff).Err(); err != nil {
		return 0, nil, err
	}

	count, err := rdb.ZCard(ctx, key).Result()
	if err != nil {
		return 0, nil, err
	}

	pairs, err := rdb.ZRevRangeWithScores(ctx, key, 0, 19).Result() // 最近活跃的 20 个
	if err != nil {
		return 0, nil, err
	}
	members := make([]Member, 0, len(pairs))
	for _, z := range pairs {
		id, err := strconv.ParseUint(z.Member.(string), 10, 64)
		if err != nil {
			continue
		}
		members = append(members, Member{ID: id, Score: int64(z.Score)})
	}
	return count, members, nil
}

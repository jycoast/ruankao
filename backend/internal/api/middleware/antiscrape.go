package middleware

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"software-exam/pkg/redisclient"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// —— 行为检测：识别批量爬取模式并自动降速（不封号） ——
//
// 两条检测规则：
//  1. 广度：24h 内访问的不同题目数超阈值（HyperLogLog 去重计数）
//  2. 顺序遍历：最近 50 次题目详情访问中，ID 严格递增的次数超阈值
//
// 命中任一规则 → 写入 as:suspect:{user_id}（TTL 24h），
// ratelimit.go 检测到该标记后将限额降为 suspect_per_min，到期自动恢复。

const (
	suspectTTL     = 24 * time.Hour
	seqWindowSize  = 50 // 顺序遍历检测的滑动窗口大小
	hllKeyTTL      = 24 * time.Hour
)

// markSuspect 标记可疑用户并记录结构化日志。
func markSuspect(ctx context.Context, rdb *redis.Client, userID uint64, reason string) {
	key := fmt.Sprintf("as:suspect:%d", userID)
	// 仅首次标记时打日志，避免刷屏
	ok, err := rdb.SetNX(ctx, key, reason, suspectTTL).Result()
	if err == nil && ok {
		log.Printf("[anticrawl] user=%d marked suspect, reason=%s (throttled for 24h)", userID, reason)
	}
}

// TrackQuestionAccess 记录题目访问行为并执行检测。
// 挂在题目详情路由（/questions/:id）上，须在 AuthMiddleware 之后。
func TrackQuestionAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next() // 先放行业务，检测逻辑在响应后异步执行

		rdb := redisclient.Client
		if rdb == nil {
			return
		}
		cfg := acConfig()
		if !cfg.Enable {
			return
		}
		userID := c.GetUint64("user_id")
		if userID == 0 {
			return
		}
		qid, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			return
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			defer func() { _ = recover() }() // 检测协程不允许影响主流程

			// 规则1：24h 不同题目数（HyperLogLog）
			hllKey := fmt.Sprintf("as:qhll:%d", userID)
			pipe := rdb.Pipeline()
			pipe.PFAdd(ctx, hllKey, qid)
			pipe.Expire(ctx, hllKey, hllKeyTTL)
			pfCount := pipe.PFCount(ctx, hllKey)
			if _, err := pipe.Exec(ctx); err != nil {
				return
			}
			if n, err := pfCount.Result(); err == nil && n > int64(cfg.DistinctQuestions24h) {
				markSuspect(ctx, rdb, userID, fmt.Sprintf("distinct_questions_24h=%d", n))
				return
			}

			// 规则2：顺序遍历检测。维护最近访问的 ID 列表，
			// 统计相邻严格递增的次数占比。
			seqKey := fmt.Sprintf("as:qseq:%d", userID)
			pipe = rdb.Pipeline()
			pipe.RPush(ctx, seqKey, qid)
			pipe.LTrim(ctx, seqKey, -seqWindowSize, -1)
			pipe.Expire(ctx, seqKey, time.Hour)
			lrange := pipe.LRange(ctx, seqKey, 0, -1)
			if _, err := pipe.Exec(ctx); err != nil {
				return
			}
			ids, err := lrange.Result()
			if err != nil || len(ids) < seqWindowSize {
				return
			}
			ascending := 0
			for i := 1; i < len(ids); i++ {
				prev, _ := strconv.ParseUint(ids[i-1], 10, 64)
				cur, _ := strconv.ParseUint(ids[i], 10, 64)
				if cur > prev {
					ascending++
				}
			}
			if ascending >= cfg.SequentialThreshold {
				markSuspect(ctx, rdb, userID, fmt.Sprintf("sequential_scan ascending=%d/%d", ascending, len(ids)-1))
			}
		}()
	}
}

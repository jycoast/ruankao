package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"software-exam/pkg/redisclient"
	"software-exam/pkg/response"

	"github.com/gin-gonic/gin"
)

// AntiCrawlHandler 反爬管理接口：查看/解除被降速的可疑用户
type AntiCrawlHandler struct{}

func NewAntiCrawlHandler() *AntiCrawlHandler {
	return &AntiCrawlHandler{}
}

type suspectInfo struct {
	UserID uint64 `json:"user_id"`
	Reason string `json:"reason"`
	TTLSec int64  `json:"ttl_seconds"` // 剩余降速时间
}

// ListSuspects GET /api/admin/anticrawl/suspects
func (h *AntiCrawlHandler) ListSuspects(c *gin.Context) {
	rdb := redisclient.Client
	if rdb == nil {
		response.ServerError(c, "Redis 不可用")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	suspects := make([]suspectInfo, 0)
	iter := rdb.Scan(ctx, 0, "as:suspect:*", 200).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		idStr := strings.TrimPrefix(key, "as:suspect:")
		userID, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			continue
		}
		reason, _ := rdb.Get(ctx, key).Result()
		ttl, _ := rdb.TTL(ctx, key).Result()
		suspects = append(suspects, suspectInfo{UserID: userID, Reason: reason, TTLSec: int64(ttl.Seconds())})
	}
	if err := iter.Err(); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, suspects)
}

// RemoveSuspect DELETE /api/admin/anticrawl/suspects/:user_id
func (h *AntiCrawlHandler) RemoveSuspect(c *gin.Context) {
	rdb := redisclient.Client
	if rdb == nil {
		response.ServerError(c, "Redis 不可用")
		return
	}
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	// 同时清掉行为统计，避免解封后立即再次触发
	rdb.Del(ctx,
		fmt.Sprintf("as:suspect:%d", userID),
		fmt.Sprintf("as:qhll:%d", userID),
		fmt.Sprintf("as:qseq:%d", userID),
	)
	response.Success(c, nil)
}

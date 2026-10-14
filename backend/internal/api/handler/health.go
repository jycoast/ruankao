package handler

import (
	"context"
	"net/http"
	"time"

	"software-exam/pkg/database"
	"software-exam/pkg/redisclient"

	"github.com/gin-gonic/gin"
)

// Health GET /api/health —— 真实探活，供容器健康检查与负载均衡摘流使用。
// DB 不可用返回 503；Redis 故障只标记 degraded 仍返回 200，
// 因为限流是 fail-open 设计（见 redisclient.Init），Redis 挂了主业务照常服务。
// 故意不返回底层错误详情，避免把 DSN / 内网地址泄露给公网。
func Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	checks := gin.H{}
	dbOK := false
	if sqlDB, err := database.DB.DB(); err == nil && sqlDB.PingContext(ctx) == nil {
		dbOK = true
		checks["database"] = "ok"
	} else {
		checks["database"] = "down"
	}

	redisOK := true
	if redisclient.Client == nil {
		checks["redis"] = "disabled"
	} else if redisclient.Client.Ping(ctx).Err() != nil {
		redisOK = false
		checks["redis"] = "down"
	} else {
		checks["redis"] = "ok"
	}

	switch {
	case !dbOK:
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "checks": checks})
	case !redisOK:
		c.JSON(http.StatusOK, gin.H{"status": "degraded", "checks": checks})
	default:
		c.JSON(http.StatusOK, gin.H{"status": "ok", "checks": checks})
	}
}

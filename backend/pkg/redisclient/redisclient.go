package redisclient

import (
	"context"
	"fmt"
	"log"
	"time"

	"software-exam/config"

	"github.com/redis/go-redis/v9"
)

// Client 全局 Redis 客户端。Init 失败时保持为 nil，
// 依赖方须自行判空并降级（fail-open），避免 Redis 故障影响主业务。
var Client *redis.Client

func Init() {
	cfg := config.AppConfig.Redis
	Client = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := Client.Ping(ctx).Err(); err != nil {
		log.Printf("[redis] connect failed (%v), rate limiting will fail-open", err)
		// 保留客户端实例：Redis 恢复后限流自动生效
	}
}

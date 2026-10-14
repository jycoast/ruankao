// main.go 入口。
//
// 重构前 536 行:装配 91 个 New(...)+ 路由注册 + server lifecycle 全部平铺。
// 重构后 ~80 行:本文件只剩「config + infra + server lifecycle」三件事。
//   - 装配 → cmd/server/wire.NewApp
//   - 路由 → internal/api/router.NewRouter
//   - 详细路由/中间件装配见各 package
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"software-exam/cmd/server/wire"
	"software-exam/config"
	"software-exam/internal/api/router"
	"software-exam/pkg/database"
	"software-exam/pkg/redisclient"

	"github.com/gin-gonic/gin"
)

func main() {
	loadConfig()
	database.InitDB()
	redisclient.Init()

	// 装配全栈:repo → service → handler → middleware → 路由。
	// 任何一处装配失败（如 Redis 连不上）会让 wire 内返回的 NewApp 为 nil——
	// 当前实现里 NewApp 不会返回 error,基础设施问题由 InitDB/Init 提前 fail-fast。
	app := wire.NewApp(database.DB, redisclient.Client)
	r := router.NewRouter(app)

	port := config.AppConfig.Server.Port
	if port == 0 {
		port = 8080
	}

	// 超时分层，别让服务端自己变成流式响应的隐形死线：
	//   ReadTimeout  —— 只覆盖读请求头+体；AI 接口的请求体就是一小段 JSON，60s 足够。
	//   WriteTimeout —— 整条响应的总时长上限。SSE 在 sseSetup 里已单独置零；
	//                   这里取 > aiclient 的流式兜底(10min)，是防 SetWriteDeadline
	//                   万一没生效：那样也该由客户端超时先报错给用户，而不是被服务端
	//                   静默切断 socket —— 后者前端只会看到「输出到一半停了」。
	//   IdleTimeout  —— keep-alive 空闲连接，与生成时长无关。
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 15 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		fmt.Printf("Server running on port %d\n", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// 优雅关闭:收到 SIGTERM/SIGINT 后停止接受新连接,
	// 给在途请求(AI 补全最长 120s)留出 130s 处理完再退出。
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 130*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}
	if sqlDB, err := database.DB.DB(); err == nil {
		sqlDB.Close()
	}
	if redisclient.Client != nil {
		redisclient.Client.Close()
	}
	log.Println("Server exited")
}

// loadConfig 加载 yaml 并设置 gin mode。未显式配置时默认 release,
// 避免生产环境误开 debug 打印路由与请求详情。
func loadConfig() {
	if err := config.Load("config/config.yaml"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	switch config.AppConfig.Server.Mode {
	case "debug":
		gin.SetMode(gin.DebugMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.ReleaseMode)
	}
}

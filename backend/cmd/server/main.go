package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/Garl404/IT_Forum/backend/internal/auth"
	"github.com/Garl404/IT_Forum/backend/internal/config"
	"github.com/Garl404/IT_Forum/backend/internal/database"
	"github.com/Garl404/IT_Forum/backend/internal/moderation"
	"github.com/Garl404/IT_Forum/backend/internal/monitor"
	"github.com/Garl404/IT_Forum/backend/internal/router"
	"github.com/Garl404/IT_Forum/backend/internal/version"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	if err := database.Open(cfg); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	adminHash, err := auth.HashPassword("caluse_665544Xx", cfg)
	if err != nil {
		log.Fatalf("站长账号初始化失败: %v", err)
	}
	if err := database.SeedAdmin(adminHash); err != nil {
		log.Fatalf("站长账号初始化失败: %v", err)
	}

	if _, err := moderation.Load(cfg.Content.BannedWordsFile); err != nil {
		log.Fatalf("违禁词库加载失败: %v", err)
	}
	log.Printf("离线违禁词库已加载：%d 条", len(moderation.Default.List()))

	mon := monitor.New(cfg)
	mon.Start()

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:           addr,
		Handler:        router.New(cfg),
		ReadTimeout:    time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout:   time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:    time.Duration(cfg.Server.IdleTimeoutSec) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go func() {
		log.Printf("CALUSE 论坛 v%s 已启动: http://localhost:%d（Go %s, GOMAXPROCS=%d）", version.Version, cfg.Server.Port, runtime.Version(), runtime.GOMAXPROCS(0))
		log.Printf("DDoS 防护: %v | 触发阈值 %d RPS | 盾状态: %v", cfg.DDOS.Enabled, cfg.DDOS.TriggerRPS, mon.IsShielded())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在优雅关闭...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("关闭异常: %v", err)
	}
	_ = auth.OTP
	log.Println("已退出")
}

// Command open-school 是独立的公开网校前台（Go 模板 + 本地内嵌 htmx）。
//
// 为什么是独立进程（#2747 技术栈改判 / #2752 路由归属）：
//   - 公开面要能被搜索引擎与 IM 预览直接抓（服务端直出 title/og:*），
//     又不能把现有学员 SPA 的登录态带上——独立进程是「不转发任何凭据」最硬的保证；
//   - 它只消费三条只读公开接口（见 upstream.go 的契约注释），不写库、不写画布状态。
//
// 用法：
//
//	go run ./cmd/open-school
//	OPEN_SCHOOL_LISTEN=127.0.0.1:18090 \
//	OPEN_SCHOOL_API=http://127.0.0.1:18080 \
//	OPEN_SCHOOL_LEARN_BASE=http://training.localhost:18091 \
//	go run ./cmd/open-school
package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// isLoopbackListen 判断监听地址是否落在回环上（启动日志用，判定口径与
// config.validate 的默认拒绝一致）。
func isLoopbackListen(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return false
	}
	return isLoopbackHost(host)
}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("初始化日志失败：%v", err)
	}
	defer func() { _ = logger.Sync() }()

	cfg, err := envConfig(os.Getenv)
	if err != nil {
		logger.Fatal("配置无效", zap.Error(err))
	}

	application := &app{
		upstream:  newUpstream(cfg.APIBase),
		renderer:  newRenderer(logger),
		learnBase: cfg.LearnBase,
		log:       logger,
	}

	server := &http.Server{
		Addr:    cfg.Listen,
		Handler: application.routes(),
		// 公开面只有三个动作（读目录/读详情/读静态），不需要长连接场景；
		// 超时给紧一点，避免慢客户端占住 goroutine。
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Info("公开网校启动",
		zap.String("listen", cfg.Listen),
		zap.String("api", cfg.APIBase),
		zap.String("learn_base", cfg.LearnBase),
	)
	if cfg.AllowNonLoopback && !isLoopbackListen(cfg.Listen) {
		// 偏离默认安全姿态必须留痕：这条日志是事后追「端口什么时候开出去的」的唯一线索。
		logger.Warn("监听地址不是回环地址（已由 OPEN_SCHOOL_ALLOW_NON_LOOPBACK 显式放行）："+
			"请确保前面有按 Host 分流的反向代理，且该端口没有被映射到宿主机公网",
			zap.String("listen", cfg.Listen),
			zap.Bool("all_interfaces", cfg.bindsAllInterfaces()),
		)
	}

	// 优雅退出：容器滚动更新时先把在途请求发完，而不是让连接被 RST。
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatal("HTTP 服务退出", zap.Error(err))
	}
	logger.Info("公开网校已停止")
}

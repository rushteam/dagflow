package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rushteam/dagflow/internal/application/dag"
	"github.com/rushteam/dagflow/internal/application/executor"
	appscheduler "github.com/rushteam/dagflow/internal/application/scheduler"
	"github.com/rushteam/dagflow/internal/application/worker"
	"github.com/rushteam/dagflow/internal/infrastructure/auth"
	infraconfig "github.com/rushteam/dagflow/internal/infrastructure/config"
	infradatabase "github.com/rushteam/dagflow/internal/infrastructure/database"
	"github.com/rushteam/dagflow/internal/infrastructure/election"
	infrahttp "github.com/rushteam/dagflow/internal/infrastructure/http"
	ihandler "github.com/rushteam/dagflow/internal/interface/http/handler"
)

//go:embed frontend/dist/*
var frontendFS embed.FS

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := infraconfig.Load(*configPath)
	if err != nil {
		slog.Warn("加载配置文件失败，使用默认配置", "error", err)
	}

	setupLogLevel(cfg.Server.LogLevel)

	db, err := infradatabase.New(ctx, infradatabase.Config{
		DSN:             cfg.Database.DSN,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	})
	if err != nil {
		slog.ErrorContext(ctx, "数据库连接失败", "error", err)
		return
	}
	defer db.Close()

	jwtManager := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.TokenExpiration)

	// 执行器注册表 + DAG 执行器 + Worker + 调度引擎
	kinds := executor.NewRegistry()
	executor.RegisterBuiltin(kinds)

	dagExec := dag.NewExecutor(db.DB)
	kinds.Register(dagExec.Info())

	localWorker := worker.NewLocalWorker(kinds)
	sched := appscheduler.New(db.DB, kinds, localWorker)
	dagExec.SetRunner(sched)
	defer sched.Stop()

	// Leader Election：由 elector 驱动调度引擎的启停
	elector, err := election.NewElector(cfg.Election, db.DB)
	if err != nil {
		slog.ErrorContext(ctx, "创建 Leader Elector 失败", "error", err)
		return
	}
	go elector.Run(ctx, election.LeaderCallbacks{
		OnStartedLeading: func(leaderCtx context.Context) {
			slog.Info("当选 Leader，启动调度引擎")
			if err := sched.Start(leaderCtx); err != nil {
				slog.Error("调度引擎启动失败", "error", err)
			}
		},
		OnStoppedLeading: func() {
			slog.Info("失去 Leader，停止调度引擎")
			sched.Stop()
		},
	})

	authHandler := ihandler.NewAuthHandler(db.DB, jwtManager)
	userHandler := ihandler.NewUserHandler(db.DB, jwtManager)
	taskHandler := ihandler.NewTaskHandler(db.DB, jwtManager, kinds, sched)
	scheduleHandler := ihandler.NewScheduleHandler(db.DB, jwtManager, sched)

	r := infrahttp.NewRouter()
	authHandler.RegisterRoutes(r)
	userHandler.RegisterRoutes(r)
	taskHandler.RegisterRoutes(r)
	scheduleHandler.RegisterRoutes(r)

	// /leader 端点：返回当前 Pod 是否为 Leader
	r.Get("/leader", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"leader": elector.IsLeader()})
	})

	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		slog.ErrorContext(ctx, "加载前端静态文件失败", "error", err)
		return
	}
	infrahttp.ServeSPA(r, distFS)

	srv := infrahttp.NewServer(cfg.Server.Port, r)
	srv.Start()

	<-ctx.Done()
	slog.Info("收到关闭信号，开始优雅关闭...")

	if err := srv.GracefulShutdown(30 * time.Second); err != nil {
		slog.Error("服务关闭过程中出现错误", "error", err)
	}
	slog.Info("服务已完全关闭")
}

func setupLogLevel(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))
}

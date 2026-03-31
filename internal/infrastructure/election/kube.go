package election

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

type kubeElector struct {
	leaseName      string
	leaseNamespace string
	id             string
	leader         atomic.Bool
}

func newKubeElector(cfg KubeConfig) *kubeElector {
	name := cfg.LeaseName
	if name == "" {
		name = "dash-leader"
	}
	ns := cfg.LeaseNamespace
	if ns == "" {
		ns = os.Getenv("POD_NAMESPACE")
	}
	if ns == "" {
		ns = "default"
	}
	return &kubeElector{
		leaseName:      name,
		leaseNamespace: ns,
		id:             podID(),
	}
}

func (e *kubeElector) Run(ctx context.Context, cb LeaderCallbacks) error {
	clientset, err := e.buildClient()
	if err != nil {
		return fmt.Errorf("创建 K8s 客户端失败: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := e.runElection(ctx, clientset, cb); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("K8s leader election 出错，重试中", "error", err)
		}

		sleepCtx(ctx, 2*time.Second)
	}
}

func (e *kubeElector) IsLeader() bool {
	return e.leader.Load()
}

func (e *kubeElector) buildClient() (*kubernetes.Clientset, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		home, _ := os.UserHomeDir()
		kubeconfig := filepath.Join(home, ".kube", "config")
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("无法获取 K8s 配置（in-cluster 和 kubeconfig 均失败）: %w", err)
		}
	}
	return kubernetes.NewForConfig(cfg)
}

func (e *kubeElector) runElection(ctx context.Context, clientset *kubernetes.Clientset, cb LeaderCallbacks) error {
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      e.leaseName,
			Namespace: e.leaseNamespace,
		},
		Client: clientset.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: e.id,
		},
	}

	le, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:            lock,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		ReleaseOnCancel: true,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(leaderCtx context.Context) {
				e.leader.Store(true)
				slog.Info("获得 K8s Lease，当选 Leader", "lease", e.leaseName, "id", e.id)
				cb.OnStartedLeading(leaderCtx)
				<-leaderCtx.Done()
			},
			OnStoppedLeading: func() {
				e.leader.Store(false)
				slog.Info("失去 K8s Lease，不再是 Leader", "lease", e.leaseName)
				cb.OnStoppedLeading()
			},
		},
	})
	if err != nil {
		return fmt.Errorf("创建 LeaderElector 失败: %w", err)
	}

	le.Run(ctx)
	return nil
}

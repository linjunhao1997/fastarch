// Package app 提供基于 cobra 的服务启动引导：统一注册 -c/--config flag、
// 在命令执行前加载配置、为每个命令注入信号感知的 context。
//
// 典型用法：
//
//	a := app.New[conf.Config]("iam-service",
//	    app.WithConfigDefaults(map[string]any{"grpc.port": 9090}),
//	)
//	a.AddCommand("grpc", "启动gRPC服务器", server.RunGrpcServer)
//	a.AddCommand("http", "启动HTTP服务器", server.RunHttpServer)
//	a.Execute()
package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/linjunhao1997/contrib/pkg/config"
	"github.com/spf13/cobra"
)

// RunFunc 是子命令的业务入口：接收信号感知 context 和已加载的配置，返回错误。
type RunFunc[T any] func(ctx context.Context, cfg *T) error

// App 封装一个服务的根命令和配置加载逻辑。
type App[T any] struct {
	name   string
	root   *cobra.Command
	cfg    *T
	config configOptions
}

type configOptions struct {
	path        string
	defaults    map[string]any
	searchPaths []string
}

// Option 引导选项。
type Option func(*configOptions)

// WithConfigPath 设置 -c/--config 的默认值，默认 "conf/config.yaml"。
func WithConfigPath(path string) Option {
	return func(o *configOptions) {
		o.path = path
	}
}

// WithConfigDefaults 透传给 config.Load 的默认值。
func WithConfigDefaults(defaults map[string]any) Option {
	return func(o *configOptions) {
		o.defaults = defaults
	}
}

// WithConfigSearchPaths 透传给 config.Load 的额外兜底查找路径。
func WithConfigSearchPaths(paths ...string) Option {
	return func(o *configOptions) {
		o.searchPaths = append(o.searchPaths, paths...)
	}
}

// New 创建服务应用。serviceName 同时用作根命令名和配置文件查找的目录前缀
// （monorepo 根目录启动时会查找 <serviceName>/conf/config.yaml）。
func New[T any](serviceName string, opts ...Option) *App[T] {
	cfgOpts := configOptions{path: "conf/config.yaml"}
	for _, opt := range opts {
		opt(&cfgOpts)
	}

	a := &App[T]{
		name:   serviceName,
		config: cfgOpts,
	}
	a.root = &cobra.Command{
		Use:           serviceName,
		Short:         serviceName,
		SilenceUsage:  true, // 运行期错误不打印命令用法，由 Execute 统一输出错误
		SilenceErrors: true,
	}

	var configPath string
	a.root.PersistentFlags().StringVarP(&configPath, "config", "c", cfgOpts.path, "配置文件路径")
	cobra.OnInitialize(func() {
		cfg, err := config.Load[T](serviceName, configPath,
			config.WithDefaults(cfgOpts.defaults),
			config.WithSearchPaths(cfgOpts.searchPaths...),
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load config: %s\n", err)
			os.Exit(1)
		}
		a.cfg = cfg
	})

	return a
}

// AddCommand 注册一个子命令。run 在信号感知 context 中执行，
// 收到 SIGINT/SIGTERM 时 context 取消（再次收到信号则恢复默认行为直接终止进程）。
func (a *App[T]) AddCommand(use, short string, run RunFunc[T]) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			go func() {
				<-ctx.Done()
				stop() // 第二次信号恢复默认行为（直接终止）
			}()
			return run(ctx, a.cfg)
		},
	}
	a.root.AddCommand(cmd)
	return cmd
}

// Command 返回底层 cobra 根命令，用于添加自定义 flag/命令等逃生场景。
func (a *App[T]) Command() *cobra.Command {
	return a.root
}

// Execute 执行根命令，出错时打印错误并非零退出。
func (a *App[T]) Execute() {
	if err := a.root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

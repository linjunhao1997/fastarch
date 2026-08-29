package config

import (
	"os"
	"path/filepath"
	"testing"
)

type testConfig struct {
	App  App  `mapstructure:"app"`
	GRpc GRpc `mapstructure:"grpc"`
}

const yamlContent = `
app:
  name: "test-service"
grpc:
  addr: "127.0.0.1"
  port: 19090
`

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func TestLoadExplicitPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yaml")
	writeFile(t, path)

	cfg, err := Load[testConfig]("whatever", path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.GRpc.Port != 19090 || cfg.App.Name != "test-service" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

// chdir 切换工作目录并在测试结束后恢复。
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func TestLoadServiceNamePrefixFallback(t *testing.T) {
	// 工作目录为模拟的 monorepo 根：conf/config.yaml 不存在，
	// <serviceName>/conf/config.yaml 存在。
	root := t.TempDir()
	chdir(t, root)
	writeFile(t, filepath.Join(root, "foo-service", "conf", "config.yaml"))

	cfg, err := Load[testConfig]("foo-service", "conf/config.yaml")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.GRpc.Port != 19090 {
		t.Fatalf("expected port 19090 from service-name-prefixed config, got %d", cfg.GRpc.Port)
	}
}

func TestLoadMissingFile(t *testing.T) {
	root := t.TempDir()
	chdir(t, root)

	if _, err := Load[testConfig]("foo-service", "conf/config.yaml"); err == nil {
		t.Fatal("expected error when config file does not exist")
	}
}

func TestGRpcAddrWithPort(t *testing.T) {
	if got := (GRpc{Addr: "0.0.0.0", Port: 80}).AddrWithPort(); got != "0.0.0.0:80" {
		t.Fatalf("AddrWithPort = %q", got)
	}
}

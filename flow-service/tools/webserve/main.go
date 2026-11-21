// webserve 是本地联调用的静态站点 + 反向代理：
//   - 静态资源取 admin-web/dist，未知路径回落 index.html（SPA）
//   - /flow/** 反代到 flow 网关 8082
//   - IAM 相关前缀反代到 8080
//   - /seed?token=xxx&to=/flow/tasks 注入登录态后跳转（给无头浏览器截图用）
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	dir := flag.String("dir", `D:\fastarch\frontend-service\admin-web\dist`, "前端构建产物目录")
	addr := flag.String("addr", "127.0.0.1:8090", "监听地址")
	flag.Parse()

	mux := http.NewServeMux()

	flowProxy := proxyTo("http://127.0.0.1:8082")
	iamProxy := proxyTo("http://127.0.0.1:8080")

	fileServer := http.FileServer(http.Dir(*dir))

	// /flow/tasks 既是页面路由又是 API 前缀：HTML 导航交给 SPA，其余走网关。
	mux.HandleFunc("/flow/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept"), "text/html") && !strings.HasSuffix(r.URL.Path, ".json") {
			http.ServeFile(w, r, filepath.Join(*dir, "index.html"))
			return
		}
		flowProxy.ServeHTTP(w, r)
	})
	for _, p := range []string{"/authentication/", "/account/", "/authorization/", "/application/", "/iam/"} {
		mux.Handle(p, iamProxy)
	}

	mux.HandleFunc("/seed", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		to := r.URL.Query().Get("to")
		if to == "" {
			to = "/flow/tasks"
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><script>
localStorage.setItem('access_token', %q);
localStorage.setItem('app_console', 'flow');
location.replace(%q);
</script>`, token, to)
	})

	fileServer = http.FileServer(http.Dir(*dir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(*dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		if strings.Contains(r.URL.Path, ".") && !strings.HasSuffix(r.URL.Path, ".html") {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(*dir, "index.html"))
	})

	log.Printf("web dev server on http://%s (dist=%s)", *addr, *dir)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func proxyTo(target string) *httputil.ReverseProxy {
	u, err := url.Parse(target)
	if err != nil {
		panic(err)
	}
	p := httputil.NewSingleHostReverseProxy(u)
	orig := p.Director
	p.Director = func(r *http.Request) {
		orig(r)
		r.Host = u.Host
	}
	return p
}

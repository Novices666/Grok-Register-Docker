package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/grok-free-register/grok-reg/internal/home"
	"github.com/grok-free-register/grok-reg/internal/webui"
)

func main() {
	addr := strings.TrimSpace(os.Getenv("WEB_ADDR"))
	if addr == "" {
		addr = ":8090"
	}
	user := strings.TrimSpace(os.Getenv("WEB_USERNAME"))
	if user == "" {
		user = "admin"
	}
	pass := os.Getenv("WEB_PASSWORD")
	if strings.TrimSpace(pass) == "" {
		fmt.Fprintln(os.Stderr, "错误: WEB_PASSWORD 不能为空")
		os.Exit(1)
	}
	paths, err := home.Resolve()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
	if err := paths.EnsureBase(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	app := webui.New(webui.AppConfig{
		Home:     paths.Root,
		Username: user,
		Password: pass,
		GrokBin:  strings.TrimSpace(os.Getenv("GROK_BIN")),
	})
	fmt.Printf("[web] listening on %s\n", addr)
	if err := http.ListenAndServe(addr, app.Handler()); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

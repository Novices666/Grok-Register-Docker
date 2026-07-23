package webui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestRequiresBasicAuth(t *testing.T) {
	dir := t.TempDir()
	app := New(AppConfig{
		Home:     dir,
		Username: "admin",
		Password: "secret",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	res := httptest.NewRecorder()

	app.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
	if got := res.Header().Get("WWW-Authenticate"); !strings.Contains(got, "Grok Register") {
		t.Fatalf("WWW-Authenticate = %q, want realm", got)
	}
}

func TestIndexShowsRedesignedHistoryUI(t *testing.T) {
	dir := t.TempDir()
	app := New(AppConfig{
		Home:     dir,
		Username: "admin",
		Password: "secret",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "secret")
	res := httptest.NewRecorder()

	app.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	body := res.Body.String()
	for _, want := range []string{"历史记录包含", "删除记录和文件", "更多下载", "查看日志", "logDialog", "file-group-body", "grok2api SSO", "CPA 输出"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q", want)
		}
	}
	if strings.Contains(body, `class="history-log"`) {
		t.Fatal("history detail should not embed the black history-log panel")
	}
}

func TestIndexExposesEveryEditableConfigKey(t *testing.T) {
	dir := t.TempDir()
	app := New(AppConfig{Home: dir, Username: "admin", Password: "secret"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "secret")
	res := httptest.NewRecorder()

	app.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	body := res.Body.String()
	for _, key := range editableConfigKeys {
		if !strings.Contains(body, `key: "`+key+`"`) {
			t.Errorf("config key %s is editable by the API but missing from the UI schema", key)
		}
	}
}

func TestConfigEndpointExposesDefaultTarget(t *testing.T) {
	dir := t.TempDir()
	app := New(AppConfig{Home: dir, Username: "admin", Password: "secret", DefaultTarget: 37})
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.SetBasicAuth("admin", "secret")
	res := httptest.NewRecorder()

	app.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if got := res.Body.String(); !strings.Contains(got, `"GROK_TARGET":"37"`) {
		t.Fatalf("body = %s, want GROK_TARGET 37", got)
	}
}

func TestAuthorizedLogEndpointReturnsLatestLog(t *testing.T) {
	dir := t.TempDir()
	logs := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logs, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "run-20260722-100000.log"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	latest := filepath.Join(logs, "run-20260722-100100.log")
	if err := os.WriteFile(latest, []byte("line one\nline two\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := New(AppConfig{
		Home:     dir,
		Username: "admin",
		Password: "secret",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/logs?tail=1", nil)
	req.SetBasicAuth("admin", "secret")
	res := httptest.NewRecorder()

	app.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", res.Code, http.StatusOK, res.Body.String())
	}
	if got := res.Body.String(); !strings.Contains(got, "line two") || strings.Contains(got, "line one") {
		t.Fatalf("body = %q, want only latest tail line", got)
	}
	if got := res.Body.String(); !strings.Contains(got, `"size":`) || !strings.Contains(got, `"mtime":`) {
		t.Fatalf("body = %q, want size and mtime fields", got)
	}
}

func TestLogEndpointUnchangedSkipsLines(t *testing.T) {
	dir := t.TempDir()
	logs := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logs, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(logs, "run-20260722-100200.log")
	if err := os.WriteFile(path, []byte("alpha\nbeta\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	app := New(AppConfig{Home: dir, Username: "admin", Password: "secret"})
	url := "/api/logs?tail=10&since_size=" + itoa64(st.Size()) + "&since_mtime=" + itoa64(st.ModTime().Unix())
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.SetBasicAuth("admin", "secret")
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	if !strings.Contains(body, `"unchanged":true`) {
		t.Fatalf("body = %s, want unchanged true", body)
	}
	if strings.Contains(body, "beta") {
		t.Fatalf("body = %s, unchanged response should omit lines", body)
	}
}

func itoa64(n int64) string {
	return strconv.FormatInt(n, 10)
}

func TestCheckCPAUsesBodyWithoutSaving(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.env"), []byte("CPA_MANAGEMENT_BASE=\nCPA_MANAGEMENT_KEY=\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := New(AppConfig{Home: dir, Username: "admin", Password: "secret"})
	// Invalid URL still proves body overrides empty saved config (not "please fill first").
	req := httptest.NewRequest(http.MethodPost, "/api/check/cpa", strings.NewReader(`{"CPA_MANAGEMENT_BASE":"http://127.0.0.1:1","CPA_MANAGEMENT_KEY":"probe-key"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "secret")
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	if strings.Contains(body, "请先填写") {
		t.Fatalf("body = %s, should use request body not saved empty config", body)
	}
	if !strings.Contains(body, `"ok":false`) {
		t.Fatalf("body = %s, want ok false for unreachable endpoint", body)
	}
	// Config file must remain empty for those keys.
	raw, err := os.ReadFile(filepath.Join(dir, "config.env"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "probe-key") {
		t.Fatalf("config.env was modified by check: %s", raw)
	}
}

func TestEmptyRunsEndpointReturnsArray(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "outputs"), 0o700); err != nil {
		t.Fatal(err)
	}
	app := New(AppConfig{Home: dir, Username: "admin", Password: "secret"})
	req := httptest.NewRequest(http.MethodGet, "/api/runs", nil)
	req.SetBasicAuth("admin", "secret")
	res := httptest.NewRecorder()

	app.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if got := strings.TrimSpace(res.Body.String()); got != "[]" {
		t.Fatalf("body = %s, want []", got)
	}
}

func TestStartEndpointPassesTargetAndThread(t *testing.T) {
	dir := t.TempDir()
	var fake string
	if runtime.GOOS == "windows" {
		fake = filepath.Join(dir, "fake-grok.bat")
		if err := os.WriteFile(fake, []byte("@echo %* > \"%~dp0args.txt\"\r\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	} else {
		fake = filepath.Join(dir, "fake-grok")
		if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' \"$*\" > \"$(dirname \"$0\")/args.txt\"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	app := New(AppConfig{
		Home:     dir,
		Username: "admin",
		Password: "secret",
		GrokBin:  fake,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/start", strings.NewReader("target=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", "secret")
	res := httptest.NewRecorder()

	app.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", res.Code, http.StatusOK, res.Body.String())
	}
	raw, err := os.ReadFile(filepath.Join(dir, "args.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(raw))
	if got != "start -t 1 -j 2" {
		t.Fatalf("args = %q, want %q", got, "start -t 1 -j 2")
	}
}

func TestStartEndpointUsesConfiguredDefaultTarget(t *testing.T) {
	dir := t.TempDir()
	var fake string
	if runtime.GOOS == "windows" {
		fake = filepath.Join(dir, "fake-grok.bat")
		if err := os.WriteFile(fake, []byte("@echo %* > \"%~dp0args.txt\"\r\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	} else {
		fake = filepath.Join(dir, "fake-grok")
		if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' \"$*\" > \"$(dirname \"$0\")/args.txt\"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	app := New(AppConfig{
		Home:          dir,
		Username:      "admin",
		Password:      "secret",
		GrokBin:       fake,
		DefaultTarget: 37,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/start", strings.NewReader("thread=3"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", "secret")
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", res.Code, http.StatusOK, res.Body.String())
	}
	raw, err := os.ReadFile(filepath.Join(dir, "args.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(raw)); got != "start -t 37 -j 3" {
		t.Fatalf("args = %q, want %q", got, "start -t 37 -j 3")
	}
}

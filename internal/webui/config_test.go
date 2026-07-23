package webui

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigRoundTripPreservesEditableValues(t *testing.T) {
	dir := t.TempDir()
	app := New(AppConfig{Home: dir, Username: "admin", Password: "secret"})
	body := strings.NewReader(`{"CPA_UPLOAD_ENABLED":"1","CPA_MANAGEMENT_BASE":"http://host.docker.internal:8317/v0/management","CPA_MANAGEMENT_KEY":"key-123","CPA_UPLOAD_NAME_TEMPLATE":"{email}.json","EMAIL_MODE":"testmail","EMAIL_DOMAIN":"example.com","EMAIL_API":"http://mail:8080","TESTMAIL_API_KEY":"tm-key","TESTMAIL_NAMESPACE":"tm-ns","TESTMAIL_DOMAIN":"inbox.testmail.app","CLEARANCE_ENABLED":"1","CLEARANCE_MODE":"auto","CLEARANCE_AUTO_STOP":"0","CF_IMPERSONATE":"chrome_131","CF_IMPERSONATE_FALLBACK":"chrome_124,chrome_120","TURNSTILE_PROVIDER":"browser","TURNSTILE_MODE":"offscreen","LITE_SOLVER_URL":"http://solver:5072","OUTPUT_SSO_ENABLED":"0","OUTPUT_GROK2API_SSO_ENABLED":"0","OUTPUT_CPA_ENABLED":"0","HTTP_POOL_SIZE":"12","PHYSICAL_CAP":"2","TURNSTILE_WORKERS":"3","TEMPMAIL_LOL_RETRIES":"42","TEMPMAIL_LOL_MIN_INTERVAL_MS":"2100","OAUTH_MIN_INTERVAL_SEC":"5","OAUTH_RETRY_SEC":"45","OAUTH_WORKERS":"1","PROBE_ENABLED":"0","PROBE_WARMUP_SEC":"2.5"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/config", body)
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "secret")
	res := httptest.NewRecorder()

	app.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	raw, err := os.ReadFile(filepath.Join(dir, "config.env"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{
		"CPA_MANAGEMENT_KEY=key-123",
		"CPA_UPLOAD_ENABLED=1",
		"CPA_UPLOAD_NAME_TEMPLATE={email}.json",
		"EMAIL_MODE=testmail",
		"CLEARANCE_MODE=auto",
		"CLEARANCE_AUTO_STOP=0",
		"CF_IMPERSONATE=chrome_131",
		"CF_IMPERSONATE_FALLBACK=chrome_124,chrome_120",
		"TURNSTILE_PROVIDER=browser",
		"TURNSTILE_MODE=offscreen",
		"LITE_SOLVER_URL=http://solver:5072",
		"TESTMAIL_API_KEY=tm-key",
		"TESTMAIL_NAMESPACE=tm-ns",
		"TESTMAIL_DOMAIN=inbox.testmail.app",
		"OUTPUT_SSO_ENABLED=0",
		"OUTPUT_GROK2API_SSO_ENABLED=0",
		"OUTPUT_CPA_ENABLED=0",
		"HTTP_POOL_SIZE=12",
		"PHYSICAL_CAP=2",
		"TURNSTILE_WORKERS=3",
		"TEMPMAIL_LOL_RETRIES=42",
		"TEMPMAIL_LOL_MIN_INTERVAL_MS=2100",
		"OAUTH_MIN_INTERVAL_SEC=5",
		"OAUTH_RETRY_SEC=45",
		"OAUTH_WORKERS=1",
		"PROBE_ENABLED=0",
		"PROBE_WARMUP_SEC=2.5",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config missing %q in:\n%s", want, text)
		}
	}
}

func TestDownloadRunReturnsZip(t *testing.T) {
	dir := t.TempDir()
	cpaDir := filepath.Join(dir, "outputs", "20260722-010203", "CPA")
	if err := os.MkdirAll(cpaDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cpaDir, "xai-test.json"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	app := New(AppConfig{Home: dir, Username: "admin", Password: "secret"})
	req := httptest.NewRequest(http.MethodGet, "/api/runs/download?id=20260722-010203", nil)
	req.SetBasicAuth("admin", "secret")
	res := httptest.NewRecorder()

	app.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	zr, err := zip.NewReader(bytes.NewReader(res.Body.Bytes()), int64(res.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, file := range zr.File {
		if file.Name == "20260722-010203/CPA/xai-test.json" {
			rc, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			found = string(b) == `{"ok":true}`
		}
	}
	if !found {
		t.Fatalf("zip did not contain expected CPA file")
	}
}

func TestRunDetailIncludesFilesAndLog(t *testing.T) {
	dir := t.TempDir()
	runID := "20260722-010203"
	ssoDir := filepath.Join(dir, "outputs", runID, "SSO")
	g2Dir := filepath.Join(dir, "outputs", runID, "grok2api")
	cpaDir := filepath.Join(dir, "outputs", runID, "CPA")
	logDir := filepath.Join(dir, "logs")
	for _, path := range []string{ssoDir, g2Dir, cpaDir, logDir} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(g2Dir, "tokens.txt"), []byte("sso-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cpaDir, "xai-test.json"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "run-"+runID+".log"), []byte("line one\nline two\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := New(AppConfig{Home: dir, Username: "admin", Password: "secret"})
	req := httptest.NewRequest(http.MethodGet, "/api/runs/detail?id="+runID, nil)
	req.SetBasicAuth("admin", "secret")
	res := httptest.NewRecorder()

	app.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, want := range []string{"tokens.txt", "xai-test.json", "line two", `"grok2api_sso_count":1`} {
		if !strings.Contains(body, want) {
			t.Fatalf("detail missing %q in %s", want, body)
		}
	}
}

func TestDeleteRunRemovesOutputAndLog(t *testing.T) {
	dir := t.TempDir()
	runID := "20260722-010203"
	outDir := filepath.Join(dir, "outputs", runID, "CPA")
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "xai-test.json"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(logDir, "run-"+runID+".log")
	if err := os.WriteFile(logPath, []byte("log\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := New(AppConfig{Home: dir, Username: "admin", Password: "secret"})
	req := httptest.NewRequest(http.MethodPost, "/api/runs/delete", strings.NewReader(`{"id":"`+runID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "secret")
	res := httptest.NewRecorder()

	app.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "outputs", runID)); !os.IsNotExist(err) {
		t.Fatalf("output dir still exists or stat error: %v", err)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("log still exists or stat error: %v", err)
	}
}

func TestDeleteRunRecordOnlyHidesHistoryAndKeepsFiles(t *testing.T) {
	dir := t.TempDir()
	runID := "20260722-010203"
	outDir := filepath.Join(dir, "outputs", runID, "CPA")
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(outDir, "xai-test.json")
	if err := os.WriteFile(outputPath, []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(logDir, "run-"+runID+".log")
	if err := os.WriteFile(logPath, []byte("log\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := New(AppConfig{Home: dir, Username: "admin", Password: "secret"})
	req := httptest.NewRequest(http.MethodPost, "/api/runs/delete", strings.NewReader(`{"id":"`+runID+`","mode":"record"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "secret")
	res := httptest.NewRecorder()

	app.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("output file should remain: %v", err)
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("log should remain: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/runs", nil)
	req.SetBasicAuth("admin", "secret")
	res = httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), runID) {
		t.Fatalf("hidden run still appears in history: %s", res.Body.String())
	}
}

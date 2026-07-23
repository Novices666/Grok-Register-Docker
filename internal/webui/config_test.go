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
	"time"
)

func TestWriteConfigMergesPreservesCommentsAndUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.env")
	initial := "# keep this header\nEMAIL_MODE=tempmail\nCUSTOM_HAND_EDITED=1\n# note about workers\nOUTPUT_CPA_ENABLED=1\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	app := New(AppConfig{Home: dir, Username: "admin", Password: "secret"})
	body := strings.NewReader(`{"EMAIL_MODE":"testmail","OUTPUT_CPA_ENABLED":"0","OUTPUT_SSO_ENABLED":"1","OUTPUT_GROK2API_SSO_ENABLED":"1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/config", body)
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "secret")
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{
		"# keep this header",
		"CUSTOM_HAND_EDITED=1",
		"# note about workers",
		"EMAIL_MODE=testmail",
		"OUTPUT_CPA_ENABLED=0",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("merged config missing %q in:\n%s", want, text)
		}
	}
}

func TestRunSummarySSOCountUsesAccountsLines(t *testing.T) {
	dir := t.TempDir()
	runID := "20260723-120000"
	ssoDir := filepath.Join(dir, "outputs", runID, "SSO")
	if err := os.MkdirAll(ssoDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Two SSO files would have made old counter return 2; accounts has 3 lines.
	accounts := "a@x.com:p:sso1\nb@x.com:p:sso2\nc@x.com:p:sso3\n"
	if err := os.WriteFile(filepath.Join(ssoDir, "accounts.txt"), []byte(accounts), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ssoDir, "auth-sessions.jsonl"), []byte("{}\n{}\n{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := New(AppConfig{Home: dir, Username: "admin", Password: "secret"})
	sum := app.summarizeRun(runID, filepath.Join(dir, "outputs", runID), time.Now())
	if sum.SSOCount != 3 {
		t.Fatalf("SSOCount = %d, want 3 (accounts lines)", sum.SSOCount)
	}
}

func TestConfigRoundTripPreservesEditableValues(t *testing.T) {
	dir := t.TempDir()
	app := New(AppConfig{Home: dir, Username: "admin", Password: "secret"})
	body := strings.NewReader(`{"CPA_UPLOAD_ENABLED":"1","CPA_MANAGEMENT_BASE":"http://host.docker.internal:8317/v0/management","CPA_MANAGEMENT_KEY":"key-123","CPA_UPLOAD_TIMEOUT_SEC":"25","CPA_UPLOAD_RETRIES":"4","CPA_UPLOAD_NAME_TEMPLATE":"{email}.json","CPA_UPLOAD_VERIFY":"0","CPA_UPLOAD_MODE":"json","EMAIL_MODE":"testmail","EMAIL_DOMAIN":"example.com","EMAIL_API":"http://mail:8080","TESTMAIL_API_KEY":"tm-key","TESTMAIL_NAMESPACE":"tm-ns","TESTMAIL_DOMAIN":"inbox.testmail.app","CLEARANCE_ENABLED":"1","CLEARANCE_MODE":"auto","CLEARANCE_AUTO_STOP":"0","CLEARANCE_COMPOSE_DIR":"C:/clearance","CF_IMPERSONATE":"chrome_131","CF_IMPERSONATE_FALLBACK":"chrome_124,chrome_120","REGISTER_PROXY":"http://register-proxy:8118","FLARESOLVERR_URL":"http://flaresolverr:8191","CLEARANCE_PROXY":"http://clearance-proxy:8118","CLEARANCE_URLS":"https://accounts.x.ai,https://x.ai","TURNSTILE_PROVIDER":"browser","TURNSTILE_MODE":"offscreen","LITE_SOLVER_URL":"http://solver:5072","CHROME_PATH":"/usr/bin/chromium","PROTOCOL_HTTP":"0","OUTPUT_SSO_ENABLED":"0","OUTPUT_GROK2API_SSO_ENABLED":"0","OUTPUT_CPA_ENABLED":"0","HTTP_POOL_SIZE":"12","PHYSICAL_CAP":"2","TURNSTILE_WORKERS":"3","TEMPMAIL_LOL_RETRIES":"42","TEMPMAIL_LOL_MIN_INTERVAL_MS":"2100","OAUTH_MIN_INTERVAL_SEC":"5","OAUTH_RETRY_SEC":"45","OAUTH_WORKERS":"1","PROBE_ENABLED":"0","PROBE_WARMUP_SEC":"2.5","HTTPS_PROXY":"http://https-proxy:8118","HTTP_PROXY":"http://http-proxy:8118","NO_PROXY":"localhost,solver"}`)
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
		"CPA_UPLOAD_TIMEOUT_SEC=25",
		"CPA_UPLOAD_RETRIES=4",
		"CPA_UPLOAD_NAME_TEMPLATE={email}",
		"CPA_UPLOAD_VERIFY=0",
		"CPA_UPLOAD_MODE=json",
		"EMAIL_MODE=testmail",
		"CLEARANCE_MODE=auto",
		"CLEARANCE_AUTO_STOP=0",
		"CLEARANCE_COMPOSE_DIR=C:/clearance",
		"CF_IMPERSONATE=chrome_131",
		"CF_IMPERSONATE_FALLBACK=chrome_124,chrome_120",
		"REGISTER_PROXY=http://register-proxy:8118",
		"FLARESOLVERR_URL=http://flaresolverr:8191",
		"CLEARANCE_PROXY=http://clearance-proxy:8118",
		"CLEARANCE_URLS=https://accounts.x.ai,https://x.ai",
		"TURNSTILE_PROVIDER=browser",
		"TURNSTILE_MODE=offscreen",
		"LITE_SOLVER_URL=http://solver:5072",
		"CHROME_PATH=/usr/bin/chromium",
		"PROTOCOL_HTTP=0",
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
		"HTTPS_PROXY=http://https-proxy:8118",
		"HTTP_PROXY=http://http-proxy:8118",
		"NO_PROXY=localhost,solver",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config missing %q in:\n%s", want, text)
		}
	}
}

func TestRuntimeTemplateKeysAreEditableInWeb(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "config.env.example"))
	if err != nil {
		t.Fatal(err)
	}
	editable := make(map[string]bool, len(editableConfigKeys))
	for _, key := range editableConfigKeys {
		editable[key] = true
	}
	for _, key := range envKeys(string(raw)) {
		if !editable[key] {
			t.Errorf("runtime config key %s is missing from Web editableConfigKeys", key)
		}
	}
}

func TestDockerTemplateKeysAreReferencedByCompose(t *testing.T) {
	root := filepath.Join("..", "..")
	envRaw, err := os.ReadFile(filepath.Join(root, ".env.docker.example"))
	if err != nil {
		t.Fatal(err)
	}
	composeRaw, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	compose := string(composeRaw)
	for _, key := range envKeys(string(envRaw)) {
		if !strings.Contains(compose, "${"+key) {
			t.Errorf("Docker template key %s is not referenced by docker-compose.yml", key)
		}
	}
}

func envKeys(content string) []string {
	seen := map[string]bool{}
	var keys []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
		key, _, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !isEnvKey(key) || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}

func isEnvKey(value string) bool {
	if value == "" || value[0] < 'A' || value[0] > 'Z' {
		return false
	}
	for _, r := range value {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
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

func TestDownloadGrok2APIReturnsRawTokensFile(t *testing.T) {
	dir := t.TempDir()
	runID := "20260722-010203"
	grok2apiDir := filepath.Join(dir, "outputs", runID, "grok2api")
	if err := os.MkdirAll(grok2apiDir, 0o700); err != nil {
		t.Fatal(err)
	}
	want := "sso-token-one\nsso-token-two\n"
	if err := os.WriteFile(filepath.Join(grok2apiDir, "tokens.txt"), []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}
	app := New(AppConfig{Home: dir, Username: "admin", Password: "secret"})
	req := httptest.NewRequest(http.MethodGet, "/api/runs/download?id="+runID+"&category=grok2api", nil)
	req.SetBasicAuth("admin", "secret")
	res := httptest.NewRecorder()

	app.Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("Content-Type = %q, want text/plain", got)
	}
	if got := res.Header().Get("Content-Disposition"); got != `attachment; filename="tokens.txt"` {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := res.Body.String(); got != want {
		t.Fatalf("body = %q, want %q", got, want)
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
	for _, want := range []string{"tokens.txt", "xai-test.json", "line two", `"grok2api_sso_count":1`, `"download_grok2api_url"`} {
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

package webui

import (
	"archive/zip"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grok-free-register/grok-reg/internal/state"
)

//go:embed templates/index.html
var templateFS embed.FS

type AppConfig struct {
	Home     string
	Username string
	Password string
	GrokBin  string
}

type App struct {
	cfg AppConfig
}

type runSummary struct {
	ID             string `json:"id"`
	SSOCount       int    `json:"sso_count"`
	Grok2APISSO    int    `json:"grok2api_sso_count"`
	CPACount       int    `json:"cpa_count"`
	Discarded      int    `json:"discarded_count"`
	TotalSize      int64  `json:"total_size"`
	LogExists      bool   `json:"log_exists"`
	LogSize        int64  `json:"log_size"`
	ModifiedAt     string `json:"modified_at"`
	Download       string `json:"download_url"`
	DownloadSSO    string `json:"download_sso_url"`
	DownloadCPA    string `json:"download_cpa_url"`
	DownloadBad    string `json:"download_discarded_url"`
	DownloadLog    string `json:"download_log_url"`
	DeleteDisabled bool   `json:"delete_disabled"`
}

type runDetail struct {
	runSummary
	Files map[string][]runFile `json:"files"`
	Log   logPreview           `json:"log"`
}

type runFile struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Size     int64  `json:"size"`
	Modified string `json:"modified_at"`
}

type logPreview struct {
	Exists bool     `json:"exists"`
	Path   string   `json:"path"`
	Lines  []string `json:"lines"`
}

var editableConfigKeys = []string{
	"EMAIL_MODE",
	"EMAIL_DOMAIN",
	"EMAIL_API",
	"TESTMAIL_API_KEY",
	"TESTMAIL_NAMESPACE",
	"TESTMAIL_DOMAIN",
	"CLEARANCE_ENABLED",
	"CLEARANCE_MODE",
	"CLEARANCE_AUTO_STOP",
	"CF_IMPERSONATE",
	"CF_IMPERSONATE_FALLBACK",
	"REGISTER_PROXY",
	"FLARESOLVERR_URL",
	"CLEARANCE_PROXY",
	"CLEARANCE_URLS",
	"TURNSTILE_PROVIDER",
	"TURNSTILE_MODE",
	"LITE_SOLVER_URL",
	"PROTOCOL_HTTP",
	"HTTP_POOL_SIZE",
	"TEMPMAIL_LOL_RETRIES",
	"TEMPMAIL_LOL_MIN_INTERVAL_MS",
	"OAUTH_MIN_INTERVAL_SEC",
	"OAUTH_RETRY_SEC",
	"OAUTH_WORKERS",
	"PROBE_ENABLED",
	"PROBE_WARMUP_SEC",
	"HTTPS_PROXY",
	"HTTP_PROXY",
	"NO_PROXY",
	"OUTPUT_SSO_ENABLED",
	"OUTPUT_GROK2API_SSO_ENABLED",
	"OUTPUT_CPA_ENABLED",
	"PHYSICAL_CAP",
	"TURNSTILE_WORKERS",
	"CPA_UPLOAD_ENABLED",
	"CPA_MANAGEMENT_BASE",
	"CPA_MANAGEMENT_KEY",
	"CPA_UPLOAD_TIMEOUT_SEC",
	"CPA_UPLOAD_RETRIES",
	"CPA_UPLOAD_NAME_TEMPLATE",
	"CPA_UPLOAD_VERIFY",
	"CPA_UPLOAD_MODE",
}

func New(cfg AppConfig) *App {
	if cfg.Home == "" {
		cfg.Home = "/data/grok"
	}
	if cfg.Username == "" {
		cfg.Username = "admin"
	}
	if cfg.GrokBin == "" {
		cfg.GrokBin = "grok"
	}
	return &App{cfg: cfg}
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.index)
	mux.HandleFunc("/api/status", a.status)
	mux.HandleFunc("/api/start", a.start)
	mux.HandleFunc("/api/stop", a.stop)
	mux.HandleFunc("/api/logs", a.logs)
	mux.HandleFunc("/api/runs", a.runs)
	mux.HandleFunc("/api/runs/detail", a.runDetail)
	mux.HandleFunc("/api/runs/download", a.downloadRun)
	mux.HandleFunc("/api/runs/delete", a.deleteRun)
	mux.HandleFunc("/api/config", a.config)
	mux.HandleFunc("/api/check/cpa", a.checkCPA)
	return a.auth(mux)
}

func (a *App) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(a.cfg.Username)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(a.cfg.Password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Grok Register"`)
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = redesignedPageTemplate.Execute(w, map[string]string{
		"Title": "Grok Register 控制台",
	})
}

func (a *App) status(w http.ResponseWriter, r *http.Request) {
	snap, err := a.loadSnapshot()
	if err != nil && !errors.Is(err, fs.ErrNotExist) && !os.IsNotExist(err) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (a *App) start(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	target := strings.TrimSpace(r.FormValue("target"))
	if target == "" {
		target = "10"
	}
	if n, err := strconv.Atoi(target); err != nil || n < 1 || n > 10000 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "目标数量必须是 1 到 10000"})
		return
	}
	thread := strings.TrimSpace(r.FormValue("thread"))
	if thread == "" {
		thread = a.readConfig()["TURNSTILE_WORKERS"]
	}
	if thread == "" {
		thread = "2"
	}
	if n, err := strconv.Atoi(thread); err != nil || n < 1 || n > 8 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "并发线程必须是 1 到 8"})
		return
	}
	out, err := a.runCommand("start", "-t", target, "-j", thread)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "output": out})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": out})
}

func (a *App) stop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	out, err := a.runCommand("stop")
	if err != nil && !strings.Contains(out, "未在运行") {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "output": out})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": out})
}

func (a *App) logs(w http.ResponseWriter, r *http.Request) {
	tail := 300
	if raw := strings.TrimSpace(r.URL.Query().Get("tail")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 5000 {
			tail = n
		}
	}
	path := a.latestLogPath()
	if path == "" {
		writeJSON(w, http.StatusOK, map[string]any{"path": "", "lines": []string{}})
		return
	}
	lines, err := tailFile(path, tail)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": path, "lines": lines})
}

func (a *App) runs(w http.ResponseWriter, r *http.Request) {
	out, err := a.listRuns()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) runDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if !safeRunID(id) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid run id"})
		return
	}
	detail, err := a.loadRunDetail(id, 260)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (a *App) config(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, a.readConfig())
	case http.MethodPost:
		var next map[string]string
		if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "配置格式不正确"})
			return
		}
		cfg := a.readConfig()
		for _, key := range editableConfigKeys {
			if value, ok := next[key]; ok {
				cfg[key] = strings.TrimSpace(value)
			}
		}
		if err := a.writeConfig(cfg); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "config": cfg})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) checkCPA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg := a.readConfig()
	base := strings.TrimRight(cfg["CPA_MANAGEMENT_BASE"], "/")
	key := strings.TrimSpace(cfg["CPA_MANAGEMENT_KEY"])
	if base == "" || key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "请先填写 CPA_MANAGEMENT_BASE 和 CPA_MANAGEMENT_KEY"})
		return
	}
	req, err := http.NewRequest(http.MethodGet, base+"/auth-files", nil)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("X-Management-Key", key)
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{Proxy: nil},
	}
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     resp.StatusCode >= 200 && resp.StatusCode < 300,
		"status": resp.StatusCode,
		"body":   string(body),
	})
}

func (a *App) downloadRun(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if !safeRunID(id) {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	if category == "log" {
		a.downloadLog(w, r, id)
		return
	}
	root := filepath.Join(a.cfg.Home, "outputs", id)
	zipRoot := root
	namePrefix := id
	if category != "" {
		dirName, ok := runCategoryDir(category)
		if !ok {
			http.Error(w, "invalid category", http.StatusBadRequest)
			return
		}
		zipRoot = filepath.Join(root, dirName)
		namePrefix = id + "-" + category
	}
	if st, err := os.Stat(zipRoot); err != nil || !st.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, namePrefix))
	zw := zip.NewWriter(w)
	defer zw.Close()
	_ = filepath.WalkDir(zipRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		base := filepath.Dir(root)
		if category != "" {
			base = filepath.Dir(zipRoot)
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		info, err := d.Info()
		if err != nil {
			return nil
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return nil
		}
		header.Name = rel
		header.Method = zip.Deflate
		dst, err := zw.CreateHeader(header)
		if err != nil {
			return nil
		}
		src, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer src.Close()
		_, _ = io.Copy(dst, src)
		return nil
	})
}

func (a *App) downloadLog(w http.ResponseWriter, r *http.Request, id string) {
	path := a.runLogPath(id)
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="run-%s.log"`, id))
	http.ServeFile(w, r, path)
}

func (a *App) deleteRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(r.FormValue("id"))
	mode := strings.TrimSpace(r.FormValue("mode"))
	if id == "" {
		var body struct {
			ID   string `json:"id"`
			Mode string `json:"mode"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		id = strings.TrimSpace(body.ID)
		mode = strings.TrimSpace(body.Mode)
	}
	if mode == "" {
		mode = "all"
	}
	if !safeRunID(id) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid run id"})
		return
	}
	if mode != "record" && mode != "all" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid delete mode"})
		return
	}
	if snap, err := a.loadSnapshot(); err == nil && snap.Status == state.StatusRunning && snap.RunID == id {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "当前 run 正在运行，停止后才能删除"})
		return
	}
	if mode == "record" {
		if err := a.hideRun(id); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "mode": mode})
		return
	}
	outputDir := filepath.Join(a.cfg.Home, "outputs", id)
	logPath := a.runLogPath(id)
	var errs []string
	if err := removeIfExists(outputDir); err != nil {
		errs = append(errs, err.Error())
	}
	if err := removeIfExists(logPath); err != nil {
		errs = append(errs, err.Error())
	}
	if err := removeIfExists(a.hiddenRunPath(id)); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": strings.Join(errs, "; ")})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) loadSnapshot() (state.Snapshot, error) {
	path := filepath.Join(a.cfg.Home, "state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return state.Snapshot{Status: state.StatusStopped}, err
	}
	var snap state.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return state.Snapshot{Status: state.StatusStopped}, err
	}
	return snap, nil
}

func (a *App) latestLogPath() string {
	entries, err := os.ReadDir(filepath.Join(a.cfg.Home, "logs"))
	if err != nil {
		return ""
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "run-") || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		names = append(names, entry.Name())
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return filepath.Join(a.cfg.Home, "logs", names[len(names)-1])
}

func (a *App) listRuns() ([]runSummary, error) {
	root := filepath.Join(a.cfg.Home, "outputs")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return []runSummary{}, nil
	}
	if err != nil {
		return nil, err
	}
	var runs []runSummary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if a.isRunHidden(entry.Name()) {
			continue
		}
		info, _ := entry.Info()
		dir := filepath.Join(root, entry.Name())
		summary := a.summarizeRun(entry.Name(), dir, info.ModTime())
		runs = append(runs, summary)
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].ID > runs[j].ID })
	if len(runs) > 60 {
		runs = runs[:60]
	}
	return runs, nil
}

func (a *App) summarizeRun(id, dir string, mod time.Time) runSummary {
	logInfo, logErr := os.Stat(a.runLogPath(id))
	logSize := int64(0)
	logExists := false
	if logErr == nil && !logInfo.IsDir() {
		logExists = true
		logSize = logInfo.Size()
	}
	current := false
	if snap, err := a.loadSnapshot(); err == nil && snap.Status == state.StatusRunning && snap.RunID == id {
		current = true
	}
	return runSummary{
		ID:             id,
		SSOCount:       countRunFiles(filepath.Join(dir, "SSO"), true, ".json", ".txt", ".jsonl"),
		Grok2APISSO:    countTokenLines(filepath.Join(dir, "grok2api", "tokens.txt")),
		CPACount:       countRunFiles(filepath.Join(dir, "CPA"), false, ".json"),
		Discarded:      countRunFiles(filepath.Join(dir, "discarded"), false, ".json"),
		TotalSize:      dirSize(dir),
		LogExists:      logExists,
		LogSize:        logSize,
		ModifiedAt:     mod.Format(time.RFC3339),
		Download:       "/api/runs/download?id=" + id,
		DownloadSSO:    "/api/runs/download?id=" + id + "&category=sso",
		DownloadCPA:    "/api/runs/download?id=" + id + "&category=cpa",
		DownloadBad:    "/api/runs/download?id=" + id + "&category=discarded",
		DownloadLog:    "/api/runs/download?id=" + id + "&category=log",
		DeleteDisabled: current,
	}
}

func (a *App) loadRunDetail(id string, logTail int) (runDetail, error) {
	root := filepath.Join(a.cfg.Home, "outputs", id)
	info, err := os.Stat(root)
	if err != nil {
		return runDetail{}, err
	}
	if !info.IsDir() {
		return runDetail{}, os.ErrNotExist
	}
	summary := a.summarizeRun(id, root, info.ModTime())
	detail := runDetail{
		runSummary: summary,
		Files: map[string][]runFile{
			"sso":       listRunFiles(filepath.Join(root, "SSO"), "sso"),
			"grok2api":  listNamedRunFiles(filepath.Join(root, "grok2api"), "grok2api", "tokens.txt"),
			"cpa":       listRunFiles(filepath.Join(root, "CPA"), "cpa"),
			"discarded": listRunFiles(filepath.Join(root, "discarded"), "discarded"),
		},
	}
	logPath := a.runLogPath(id)
	if st, err := os.Stat(logPath); err == nil && !st.IsDir() {
		lines, _ := tailFile(logPath, logTail)
		detail.Log = logPreview{Exists: true, Path: logPath, Lines: lines}
	}
	return detail, nil
}

func (a *App) runLogPath(id string) string {
	return filepath.Join(a.cfg.Home, "logs", "run-"+id+".log")
}

func (a *App) hiddenRunPath(id string) string {
	return filepath.Join(a.cfg.Home, "history", "deleted-runs", id+".json")
}

func (a *App) isRunHidden(id string) bool {
	if !safeRunID(id) {
		return true
	}
	st, err := os.Stat(a.hiddenRunPath(id))
	return err == nil && !st.IsDir()
}

func (a *App) hideRun(id string) error {
	if !safeRunID(id) {
		return fmt.Errorf("invalid run id")
	}
	path := a.hiddenRunPath(id)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	payload := map[string]string{
		"id":         id,
		"deleted_at": time.Now().Format(time.RFC3339),
		"mode":       "record",
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func runCategoryDir(category string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "sso":
		return "SSO", true
	case "grok2api":
		return "grok2api", true
	case "cpa":
		return "CPA", true
	case "discarded":
		return "discarded", true
	default:
		return "", false
	}
}

func removeIfExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.RemoveAll(path)
}

func listRunFiles(dir, kind string) []runFile {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []runFile{}
	}
	var files []runFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, runFile{
			Name:     entry.Name(),
			Kind:     kind,
			Size:     info.Size(),
			Modified: info.ModTime().Format(time.RFC3339),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files
}

func listNamedRunFiles(dir, kind, name string) []runFile {
	var out []runFile
	for _, file := range listRunFiles(dir, kind) {
		if strings.EqualFold(file.Name, name) {
			out = append(out, file)
		}
	}
	return out
}

func countRunFiles(dir string, excludeGrok2API bool, suffixes ...string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if excludeGrok2API && name == "grok2api-sso.txt" {
			continue
		}
		for _, suffix := range suffixes {
			if strings.HasSuffix(name, suffix) {
				n++
				break
			}
		}
	}
	return n
}

func countNamedFile(dir, name string) int {
	if st, err := os.Stat(filepath.Join(dir, name)); err == nil && !st.IsDir() {
		return 1
	}
	return 0
}

func countTokenLines(path string) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func (a *App) readConfig() map[string]string {
	cfg := defaultWebConfig()
	data, err := os.ReadFile(filepath.Join(a.cfg.Home, "config.env"))
	if err != nil {
		return cfg
	}
	for key, value := range parseEnvFile(string(data)) {
		cfg[key] = value
	}
	return cfg
}

func (a *App) writeConfig(cfg map[string]string) error {
	var b strings.Builder
	b.WriteString("# grok-reg config written by web console\n")
	for _, key := range editableConfigKeys {
		b.WriteString(key)
		b.WriteString("=")
		b.WriteString(cfg[key])
		b.WriteString("\n")
	}
	path := filepath.Join(a.cfg.Home, "config.env")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func defaultWebConfig() map[string]string {
	return map[string]string{
		"EMAIL_MODE":                   "tempmail",
		"EMAIL_DOMAIN":                 "",
		"EMAIL_API":                    "http://127.0.0.1:8080",
		"TESTMAIL_API_KEY":             "",
		"TESTMAIL_NAMESPACE":           "",
		"TESTMAIL_DOMAIN":              "inbox.testmail.app",
		"CLEARANCE_ENABLED":            "1",
		"CLEARANCE_MODE":               "auto",
		"CLEARANCE_AUTO_STOP":          "0",
		"CF_IMPERSONATE":               "chrome_131",
		"CF_IMPERSONATE_FALLBACK":      "chrome_124,chrome_120",
		"REGISTER_PROXY":               "http://privoxy:8118",
		"FLARESOLVERR_URL":             "http://flaresolverr:8191",
		"CLEARANCE_PROXY":              "http://privoxy:8118",
		"CLEARANCE_URLS":               "https://accounts.x.ai,https://x.ai,https://status.x.ai,https://console.x.ai,https://auth.x.ai",
		"TURNSTILE_PROVIDER":           "browser",
		"TURNSTILE_MODE":               "offscreen",
		"LITE_SOLVER_URL":              "http://127.0.0.1:5072",
		"PROTOCOL_HTTP":                "1",
		"HTTP_POOL_SIZE":               "8",
		"TEMPMAIL_LOL_RETRIES":         "30",
		"TEMPMAIL_LOL_MIN_INTERVAL_MS": "1500",
		"OAUTH_MIN_INTERVAL_SEC":       "4",
		"OAUTH_RETRY_SEC":              "45",
		"OAUTH_WORKERS":                "0",
		"PROBE_ENABLED":                "1",
		"PROBE_WARMUP_SEC":             "1.5",
		"HTTPS_PROXY":                  "http://privoxy:8118",
		"HTTP_PROXY":                   "http://privoxy:8118",
		"NO_PROXY":                     "127.0.0.1,localhost,privoxy,flaresolverr,warp-proxy",
		"OUTPUT_SSO_ENABLED":           "1",
		"OUTPUT_GROK2API_SSO_ENABLED":  "1",
		"OUTPUT_CPA_ENABLED":           "1",
		"PHYSICAL_CAP":                 "0",
		"TURNSTILE_WORKERS":            "2",
		"CPA_UPLOAD_ENABLED":           "0",
		"CPA_MANAGEMENT_BASE":          "http://host.docker.internal:8317/v0/management",
		"CPA_MANAGEMENT_KEY":           "",
		"CPA_UPLOAD_TIMEOUT_SEC":       "30",
		"CPA_UPLOAD_RETRIES":           "2",
		"CPA_UPLOAD_NAME_TEMPLATE":     "{email}.json",
		"CPA_UPLOAD_VERIFY":            "1",
		"CPA_UPLOAD_MODE":              "multipart",
	}
}

func parseEnvFile(content string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return out
}

func safeRunID(id string) bool {
	if len(id) < 15 || strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return false
	}
	for _, r := range id {
		if (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}

func (a *App) runCommand(args ...string) (string, error) {
	cmd := exec.Command(a.cfg.GrokBin, args...)
	cmd.Env = append(os.Environ(), "GROK_HOME="+a.cfg.Home)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func tailFile(path string, maxLines int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return []string{}, nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		_, _ = fmt.Fprintf(w, `{"error":%q}`, err.Error())
	}
}

var redesignedPageTemplate = template.Must(template.ParseFS(templateFS, "templates/index.html"))

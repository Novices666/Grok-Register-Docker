package cpa

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/grok-free-register/grok-reg/internal/oauth"
)

const (
	CliproxyBase = "https://cli-chat-proxy.grok.com/v1"
)

var CliproxyHeaders = map[string]string{
	"x-grok-client-version":    "0.2.93",
	"x-xai-token-auth":         "xai-grok-cli",
	"X-XAI-Token-Auth":         "xai-grok-cli",
	"x-authenticateresponse":   "authenticate-response",
	"x-grok-client-identifier": "grok-shell",
	"x-compaction-at":          "400000",
	"User-Agent":               "grok-shell/0.2.93 (linux; x86_64)",
}

// Document is CPA-ready JSON.
type Document struct {
	Type          string            `json:"type"`
	AccessToken   string            `json:"access_token"`
	RefreshToken  string            `json:"refresh_token"`
	IDToken       string            `json:"id_token,omitempty"`
	TokenType     string            `json:"token_type,omitempty"`
	ExpiresIn     int               `json:"expires_in"`
	Expired       string            `json:"expired"`
	LastRefresh   string            `json:"last_refresh"`
	Sub           string            `json:"sub,omitempty"`
	Email         string            `json:"email,omitempty"`
	BaseURL       string            `json:"base_url"`
	TokenEndpoint string            `json:"token_endpoint"`
	AuthKind      string            `json:"auth_kind"`
	Headers       map[string]string `json:"headers"`
}

func FromCredential(cred oauth.Credential, email string) Document {
	em := email
	if em == "" {
		em = cred.Email
	}
	return Document{
		Type:          "xai",
		AccessToken:   cred.AccessToken,
		RefreshToken:  cred.RefreshToken,
		IDToken:       cred.IDToken,
		TokenType:     cred.TokenType,
		ExpiresIn:     cred.ExpiresIn,
		Expired:       cred.ExpiresAt,
		LastRefresh:   cred.LastRefresh,
		Sub:           cred.Subject,
		Email:         em,
		BaseURL:       CliproxyBase,
		TokenEndpoint: cred.TokenEndpoint,
		AuthKind:      "oauth",
		Headers:       cloneHeaders(CliproxyHeaders),
	}
}

func Filename(doc Document, secret []byte) string {
	subject := doc.Sub
	if subject == "" {
		subject = doc.RefreshToken
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(subject))
	return "xai-" + hex.EncodeToString(mac.Sum(nil))[:16] + ".json"
}

func WriteAtomic(dir string, doc Document, secret []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	name := Filename(doc, secret)
	path := filepath.Join(dir, name)
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	raw = append(raw, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	_ = os.Chmod(path, 0o600)
	return path, nil
}

// Probe hits cli-chat-proxy with minimal responses call.
// Returns nil if alive.
func Probe(doc Document, proxy string) error {
	client := &http.Client{Timeout: 30 * time.Second}
	if proxy != "" {
		// rely on env proxy or leave direct — use Transport if needed
		_ = proxy
	}
	payload := map[string]any{
		"model": "grok-4.5",
		"input": []map[string]any{
			{"role": "user", "content": "ping"},
		},
		"max_output_tokens": 1,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(doc.BaseURL, "/")+"/responses", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+doc.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range doc.Headers {
		req.Header.Set(k, v)
	}
	// session-ish
	req.Header.Set("x-grok-session-id", "probe-"+doc.Sub)
	req.Header.Set("x-grok-req-id", fmt.Sprintf("%d", time.Now().UnixNano()))
	req.Header.Set("x-grok-model-override", "grok-4.5")
	if doc.Email != "" {
		req.Header.Set("x-email", doc.Email)
	}
	if doc.Sub != "" {
		req.Header.Set("x-userid", doc.Sub)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == 200 {
		return nil
	}
	return fmt.Errorf("probe http=%d body=%s", resp.StatusCode, truncate(string(body), 160))
}

func AppendSSO(accountsPath, email, password, sso string) error {
	if err := os.MkdirAll(filepath.Dir(accountsPath), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(accountsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s:%s:%s\n", email, password, sso)
	return err
}

func AppendAuthSession(path, email, sso string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	doc := map[string]any{
		"email": email,
		"cookies": []map[string]string{
			{"name": "sso", "value": sso, "domain": ".x.ai", "path": "/"},
		},
	}
	raw, _ := json.Marshal(doc)
	_, err = f.Write(append(raw, '\n'))
	return err
}

func cloneHeaders(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// DefaultSecret for filename hmac (local only).
func DefaultSecret() []byte {
	return []byte("grok-reg-local-cpa-name-secret")
}

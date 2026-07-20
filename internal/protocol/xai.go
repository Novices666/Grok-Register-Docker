package protocol

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/grok-free-register/grok-reg/internal/clearance"
)

const (
	SiteURL              = "https://accounts.x.ai"
	ConnectCreate        = SiteURL + "/auth_mgmt.AuthManagement/CreateEmailValidationCode"
	ConnectVerify        = SiteURL + "/auth_mgmt.AuthManagement/VerifyEmailValidationCode"
	SignupURLGrok        = SiteURL + "/sign-up?redirect=grok-com"
	DefaultUserAgent     = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"
)

var (
	siteKeyRe  = regexp.MustCompile(`0x4AAAAAAA[a-zA-Z0-9_-]+`)
	jsSrcRe    = regexp.MustCompile(`src="(/_next/static/[^"]+\.js)"`)
	hex40Re    = regexp.MustCompile(`[a-fA-F0-9]{40,50}`)
	flightRe   = regexp.MustCompile(`self\.__next_f\.push\(\[1,"(.*?)"\]\)`)
)

type SignupConfig struct {
	SiteKey   string
	ActionID  string
	StateTree string
	Source    string
}

type Client struct {
	http    *http.Client
	proxy   string
	clear   *clearance.Manager
	ua      string
	mu      sync.Mutex
	cfg     SignupConfig
}

func NewClient(proxy string, cm *clearance.Manager) (*Client, error) {
	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	}
	if proxy != "" {
		u, err := url.Parse(proxy)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(u)
	}
	c := &Client{
		http: &http.Client{
			Timeout:   45 * time.Second,
			Jar:       jar,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 8 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		proxy: proxy,
		clear: cm,
		ua:    DefaultUserAgent,
	}
	if cm != nil {
		c.ua = cm.UserAgent()
		c.applyClearanceCookies()
	}
	return c, nil
}

func (c *Client) applyClearanceCookies() {
	if c.clear == nil {
		return
	}
	b := c.clear.Get()
	u, _ := url.Parse(SiteURL)
	var cookies []*http.Cookie
	for _, ck := range b.Cookies {
		cookies = append(cookies, &http.Cookie{
			Name:   ck.Name,
			Value:  ck.Value,
			Domain: ck.Domain,
			Path:   ck.Path,
		})
	}
	if len(cookies) > 0 {
		c.http.Jar.SetCookies(u, cookies)
	}
	if b.UserAgent != "" {
		c.ua = b.UserAgent
	}
}

func (c *Client) Config() SignupConfig {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg
}

func (c *Client) FetchConfig() (SignupConfig, error) {
	c.applyClearanceCookies()
	req, err := http.NewRequest(http.MethodGet, SignupURLGrok, nil)
	if err != nil {
		return SignupConfig{}, err
	}
	c.setBrowserHeaders(req)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Referer", "https://grok.com/")
	resp, err := c.http.Do(req)
	if err != nil {
		return SignupConfig{}, err
	}
	defer resp.Body.Close()
	html, err := readBody(resp)
	if err != nil {
		return SignupConfig{}, err
	}
	cfg := SignupConfig{Source: fmt.Sprintf("http status=%d", resp.StatusCode)}
	if resp.StatusCode != 200 || isCloudflare(resp.StatusCode, html, resp.Header) {
		cfg.Source += " (blocked_or_empty)"
		return cfg, fmt.Errorf("signup page blocked status=%d", resp.StatusCode)
	}
	if m := siteKeyRe.FindString(html); m != "" {
		cfg.SiteKey = m
	}
	cfg.StateTree = scrapeStateTree(html)
	jsURLs := unique(jsSrcRe.FindAllStringSubmatch(html, -1))
	for _, path := range jsURLs {
		if cfg.ActionID != "" {
			break
		}
		js, err := c.fetchJS(path)
		if err != nil || js == "" {
			continue
		}
		if !strings.Contains(js, "createUser") && !strings.Contains(js, "registerUser") && !strings.Contains(js, "emailValidation") {
			continue
		}
		if hexes := hex40Re.FindAllString(js, -1); len(hexes) > 0 {
			cfg.ActionID = hexes[0]
		}
	}
	if cfg.SiteKey == "" || cfg.ActionID == "" || cfg.StateTree == "" {
		return cfg, fmt.Errorf("config incomplete site_key=%v action=%v state=%v", cfg.SiteKey != "", cfg.ActionID != "", cfg.StateTree != "")
	}
	c.mu.Lock()
	c.cfg = cfg
	c.mu.Unlock()
	return cfg, nil
}

func (c *Client) fetchJS(path string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, SiteURL+path, nil)
	if err != nil {
		return "", err
	}
	c.setBrowserHeaders(req)
	req.Header.Set("Referer", SignupURLGrok)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return readBody(resp)
}

func (c *Client) CreateEmailCode(email string) error {
	inner := pbStr(1, email)
	frame := grpcWebFrame(inner)
	req, err := http.NewRequest(http.MethodPost, ConnectCreate, bytes.NewReader(frame))
	if err != nil {
		return err
	}
	c.setGRPCHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	st := resp.Header.Get("grpc-status")
	if st == "" {
		st = "0"
	}
	if resp.StatusCode != 200 || (st != "0" && st != "") {
		return fmt.Errorf("create email http=%d grpc=%s", resp.StatusCode, st)
	}
	return nil
}

func (c *Client) VerifyEmailCode(email, code string) error {
	inner := append(pbStr(1, email), pbStr(2, code)...)
	frame := grpcWebFrame(inner)
	req, err := http.NewRequest(http.MethodPost, ConnectVerify, bytes.NewReader(frame))
	if err != nil {
		return err
	}
	c.setGRPCHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	st := resp.Header.Get("grpc-status")
	if st == "" {
		st = "0"
	}
	if resp.StatusCode != 200 || (st != "0" && st != "") {
		return fmt.Errorf("verify email http=%d grpc=%s", resp.StatusCode, st)
	}
	return nil
}

// SignupServerAction posts Next.js server action body; returns response text and SSO cookie if set.
func (c *Client) SignupServerAction(body []byte, actionID, stateTree string) (string, string, error) {
	req, err := http.NewRequest(http.MethodPost, SiteURL+"/sign-up", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	c.setBrowserHeaders(req)
	req.Header.Set("Accept", "text/x-component")
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	req.Header.Set("Next-Action", actionID)
	req.Header.Set("Next-Router-State-Tree", stateTree)
	req.Header.Set("Origin", SiteURL)
	req.Header.Set("Referer", SignupURLGrok)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	text, _ := readBody(resp)
	sso := ""
	u, _ := url.Parse(SiteURL)
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == "sso" {
			sso = ck.Value
		}
	}
	// Also scan Set-Cookie
	for _, sc := range resp.Cookies() {
		if sc.Name == "sso" && sc.Value != "" {
			sso = sc.Value
		}
	}
	if resp.StatusCode >= 400 {
		return text, sso, fmt.Errorf("signup http=%d body=%s", resp.StatusCode, truncate(text, 120))
	}
	return text, sso, nil
}

func (c *Client) ClearAuthCookies() {
	u, _ := url.Parse(SiteURL)
	var keep []*http.Cookie
	for _, ck := range c.http.Jar.Cookies(u) {
		ln := strings.ToLower(ck.Name)
		if ln == "sso" || ln == "sso-rw" {
			continue
		}
		keep = append(keep, ck)
	}
	// Reset jar for host by setting empty — cookiejar doesn't delete easily;
	// re-apply clearance only.
	jar, _ := cookiejar.New(nil)
	c.http.Jar = jar
	c.applyClearanceCookies()
	_ = keep
}

func (c *Client) setBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("sec-ch-ua", `"Chromium";v="146", "Google Chrome";v="146", "Not_A Brand";v="99"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Linux"`)
	if h := c.clearCookieHeader(); h != "" && req.Header.Get("Cookie") == "" {
		req.Header.Set("Cookie", h)
	}
}

func (c *Client) setGRPCHeaders(req *http.Request) {
	c.setBrowserHeaders(req)
	req.Header.Set("Content-Type", "application/grpc-web+proto")
	req.Header.Set("X-Grpc-Web", "1")
	req.Header.Set("X-User-Agent", "connect-es/2.1.1")
	req.Header.Set("Origin", SiteURL)
	req.Header.Set("Referer", SignupURLGrok)
	req.Header.Set("Accept", "*/*")
}

func (c *Client) clearCookieHeader() string {
	if c.clear == nil {
		return ""
	}
	return c.clear.CookieHeader()
}

// BuildSignupBody builds a minimal multipart-ish plain body used by Next server actions.
// Real format is complex; we encode email/password/code/token as text fields in RSC action style.
func BuildSignupBody(email, password, code, turnstileToken string) []byte {
	// Common pattern: URL-encoded-ish plain fields concatenated for text/plain server actions.
	// Keep compatible with browser path used by original project.
	var b strings.Builder
	// Field order approximated from observed Next actions.
	fmt.Fprintf(&b, `["%s","%s","%s","%s"]`,
		escapeJSON(email), escapeJSON(password), escapeJSON(code), escapeJSON(turnstileToken))
	return []byte(b.String())
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

func pbStr(field int, s string) []byte {
	tag := byte(field<<3 | 2)
	b := []byte(s)
	out := []byte{tag}
	out = append(out, pbVarint(len(b))...)
	out = append(out, b...)
	return out
}

func pbVarint(n int) []byte {
	var parts []byte
	for n > 0x7f {
		parts = append(parts, byte(n&0x7f)|0x80)
		n >>= 7
	}
	parts = append(parts, byte(n))
	return parts
}

func grpcWebFrame(inner []byte) []byte {
	frame := make([]byte, 5+len(inner))
	frame[0] = 0
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(inner)))
	copy(frame[5:], inner)
	return frame
}

func scrapeStateTree(html string) string {
	chunks := flightRe.FindAllStringSubmatch(html, -1)
	for _, ch := range chunks {
		if len(ch) < 2 {
			continue
		}
		decoded := strings.ReplaceAll(ch[1], `\"`, `"`)
		if !strings.Contains(decoded, "sign-up") {
			continue
		}
		idx := strings.Index(decoded, `"f":[[[`)
		if idx < 0 {
			continue
		}
		fStart := idx + 5
		end := strings.Index(decoded[fStart:], `"$undefined"`)
		if end < 0 {
			continue
		}
		raw := decoded[fStart : fStart+end]
		raw = strings.ReplaceAll(raw, `\\"`, `"`)
		raw = strings.ReplaceAll(raw, `\`, "")
		return url.QueryEscape(raw)
	}
	return ""
}

func isCloudflare(status int, body string, h http.Header) bool {
	if status == 403 || status == 503 {
		low := strings.ToLower(body)
		if strings.Contains(low, "cf-") || strings.Contains(low, "cloudflare") || strings.Contains(low, "just a moment") {
			return true
		}
	}
	if strings.Contains(strings.ToLower(h.Get("Server")), "cloudflare") && status >= 400 {
		return true
	}
	return false
}

func readBody(resp *http.Response) (string, error) {
	var r io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(resp.Body)
		if err == nil {
			defer gz.Close()
			r = gz
		}
	}
	b, err := io.ReadAll(io.LimitReader(r, 8<<20))
	return string(b), err
}

func unique(matches [][]string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		if _, ok := seen[m[1]]; ok {
			continue
		}
		seen[m[1]] = struct{}{}
		out = append(out, m[1])
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ExtractSSOFromText tries to find sso JWT in response body.
func ExtractSSOFromText(text string) string {
	// JWT-ish
	re := regexp.MustCompile(`eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`)
	for _, m := range re.FindAllString(text, -1) {
		// Prefer session-looking tokens
		if strings.Contains(m, "session") || len(m) > 80 {
			return m
		}
	}
	if m := re.FindString(text); m != "" {
		return m
	}
	_ = base64.StdEncoding // keep import used if needed later
	return ""
}

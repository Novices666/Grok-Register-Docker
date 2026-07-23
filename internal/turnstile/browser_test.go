package turnstile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/grok-free-register/grok-reg/internal/browser"
	"github.com/grok-free-register/grok-reg/internal/turnstile"
)

func TestBrowserProviderDefault(t *testing.T) {
	chromePath := filepath.Join(t.TempDir(), "chrome")
	if err := os.WriteFile(chromePath, []byte("test browser"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CHROME_PATH", chromePath)
	if got := browser.FindChrome(); got != chromePath {
		t.Fatalf("chrome path = %q, want %q", got, chromePath)
	}
	pr := turnstile.New(turnstile.Options{Provider: "browser"})
	if pr.Name() != "browser" {
		t.Fatalf("provider name=%s", pr.Name())
	}
}

func TestNewDefaultsToBrowser(t *testing.T) {
	pr := turnstile.New(turnstile.Options{})
	if pr.Name() != "browser" {
		t.Fatalf("default provider=%s want browser", pr.Name())
	}
}

package identity

import (
	"net/http"
	"testing"
)

func TestResolve_JWT(t *testing.T) {
	r := NewResolver("test-secret")

	// We'll test that invald tokens fall through gracefully
	req, _ := http.NewRequest("GET", "/data", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	info := r.Resolve(req)
	// Invalid JWT should fall through to fingerprint
	if info.Method == "jwt" {
		t.Error("invalid JWT should not resolve to jwt method")
	}
}

func TestResolve_APIKey(t *testing.T) {
	r := NewResolver("")

	req, _ := http.NewRequest("GET", "/data", nil)
	req.Header.Set("X-API-Key", "my-secret-api-key-123")

	info := r.Resolve(req)
	if info.Method != "api_key" {
		t.Errorf("expected api_key method, got %s", info.Method)
	}
	if info.UserID == "" {
		t.Error("expected non-empty UserID for API key")
	}
	if info.UserID[:7] != "apikey:" {
		t.Errorf("expected apikey: prefix, got %s", info.UserID)
	}
}

func TestResolve_Fingerprint(t *testing.T) {
	r := NewResolver("")

	req, _ := http.NewRequest("GET", "/data", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	info := r.Resolve(req)
	if info.Method != "fingerprint" {
		t.Errorf("expected fingerprint method, got %s", info.Method)
	}
	if info.UserID[:3] != "fp:" {
		t.Errorf("expected fp: prefix, got %s", info.UserID)
	}
}

// Test that fingerprint includes Accept-Language for uniqueness
func TestResolve_FingerprintIncludesAcceptLanguage(t *testing.T) {
	r := NewResolver("")

	req1, _ := http.NewRequest("GET", "/data", nil)
	req1.RemoteAddr = "192.168.1.100:12345"
	req1.Header.Set("User-Agent", "Mozilla/5.0")
	req1.Header.Set("Accept-Language", "en-US")

	req2, _ := http.NewRequest("GET", "/data", nil)
	req2.RemoteAddr = "192.168.1.100:12345"
	req2.Header.Set("User-Agent", "Mozilla/5.0")
	req2.Header.Set("Accept-Language", "fr-FR")

	info1 := r.Resolve(req1)
	info2 := r.Resolve(req2)

	if info1.UserID == info2.UserID {
		t.Error("different Accept-Language should produce different fingerprints")
	}
}

func TestResolve_MissingHeaders(t *testing.T) {
	r := NewResolver("")

	req, _ := http.NewRequest("GET", "/data", nil)
	req.RemoteAddr = "10.0.0.1:9999"

	info := r.Resolve(req)
	if info.Method != "fingerprint" {
		t.Errorf("expected fingerprint for missing headers, got %s", info.Method)
	}
	if info.UserID == "" {
		t.Error("should produce a non-empty UserID even with no headers")
	}
}

func TestExtractClientIP_XForwardedFor(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18")

	ip := extractClientIP(req)
	if ip != "203.0.113.50" {
		t.Errorf("expected first XFF IP, got %s", ip)
	}
}

func TestExtractClientIP_XRealIP(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Real-IP", "203.0.113.99")

	ip := extractClientIP(req)
	if ip != "203.0.113.99" {
		t.Errorf("expected X-Real-IP, got %s", ip)
	}
}

package whatsapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestAuthPageServesHTML(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(authPage))
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html" {
		t.Fatalf("expected text/html, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Link your WhatsApp") {
		t.Fatal("expected page to contain 'Link your WhatsApp'")
	}
	if !strings.Contains(body, "qrcodejs") {
		t.Fatal("expected page to reference qrcodejs CDN")
	}
	if !strings.Contains(body, "/api/qr") {
		t.Fatal("expected page to poll /api/qr")
	}
}

func TestQREndpointReturnsJSON(t *testing.T) {
	var mu sync.Mutex
	currentQR := "test-qr-code-data"
	linking := false
	authenticated := false

	mux := http.NewServeMux()
	mux.HandleFunc("/api/qr", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		resp := map[string]any{
			"qr":            currentQR,
			"linking":       linking,
			"authenticated": authenticated,
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	req := httptest.NewRequest("GET", "/api/qr", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if resp["qr"] != "test-qr-code-data" {
		t.Fatalf("expected qr code data, got %v", resp["qr"])
	}
	if resp["linking"] != false {
		t.Fatalf("expected linking=false, got %v", resp["linking"])
	}
	if resp["authenticated"] != false {
		t.Fatalf("expected authenticated=false, got %v", resp["authenticated"])
	}
}

func TestQREndpointAuthenticated(t *testing.T) {
	var mu sync.Mutex
	currentQR := ""
	linking := false
	authenticated := true

	mux := http.NewServeMux()
	mux.HandleFunc("/api/qr", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		resp := map[string]any{
			"qr":            currentQR,
			"linking":       linking,
			"authenticated": authenticated,
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	req := httptest.NewRequest("GET", "/api/qr", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if resp["authenticated"] != true {
		t.Fatalf("expected authenticated=true, got %v", resp["authenticated"])
	}
	if resp["linking"] != false {
		t.Fatalf("expected linking=false when authenticated, got %v", resp["linking"])
	}
	if resp["qr"] != "" {
		t.Fatalf("expected empty qr when authenticated, got %v", resp["qr"])
	}
}

func TestQREndpointLinking(t *testing.T) {
	var mu sync.Mutex
	currentQR := ""
	linking := true
	authenticated := false

	mux := http.NewServeMux()
	mux.HandleFunc("/api/qr", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		resp := map[string]any{
			"qr":            currentQR,
			"linking":       linking,
			"authenticated": authenticated,
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	req := httptest.NewRequest("GET", "/api/qr", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if resp["linking"] != true {
		t.Fatalf("expected linking=true, got %v", resp["linking"])
	}
	if resp["authenticated"] != false {
		t.Fatalf("expected authenticated=false during linking, got %v", resp["authenticated"])
	}
	if resp["qr"] != "" {
		t.Fatalf("expected empty qr during linking, got %v", resp["qr"])
	}
}

func TestAuthPageContainsQRPolling(t *testing.T) {
	if !strings.Contains(authPage, "setTimeout(poll,hasQR?2000:3000)") {
		t.Fatal("expected page to poll every 2-3 seconds")
	}
	if !strings.Contains(authPage, "d.linking") {
		t.Fatal("expected page to handle linking state in poll")
	}
}

func TestAuthPageContainsLinkingMessage(t *testing.T) {
	if !strings.Contains(authPage, "Linking your device, please wait...") {
		t.Fatal("expected page to show linking message during device linking")
	}
	if !strings.Contains(authPage, `id="linking"`) {
		t.Fatal("expected page to have a linking element")
	}
}

func TestAuthPageContainsSuccessMessage(t *testing.T) {
	if !strings.Contains(authPage, "WhatsApp linked successfully!") {
		t.Fatal("expected page to show success message after authentication")
	}
}

func TestAuthPageContainsInstructions(t *testing.T) {
	instructions := []string{
		"Open <strong>WhatsApp</strong>",
		"Linked Devices",
		"Link a Device",
		"Point your camera",
	}
	for _, s := range instructions {
		if !strings.Contains(authPage, s) {
			t.Fatalf("expected page to contain instruction %q", s)
		}
	}
}

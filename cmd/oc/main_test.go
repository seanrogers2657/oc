package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckOllamaTools(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		want       bool
	}{
		{
			name:       "model with tools capability",
			response:   `{"capabilities":["completion","tools"]}`,
			statusCode: 200,
			want:       true,
		},
		{
			name:       "model without tools capability",
			response:   `{"capabilities":["completion"]}`,
			statusCode: 200,
			want:       false,
		},
		{
			name:       "empty capabilities",
			response:   `{"capabilities":[]}`,
			statusCode: 200,
			want:       false,
		},
		{
			name:       "no capabilities field",
			response:   `{"modelfile":"..."}`,
			statusCode: 200,
			want:       false,
		},
		{
			name:       "server error",
			response:   `{"error":"model not found"}`,
			statusCode: 404,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				fmt.Fprint(w, tt.response)
			}))
			defer srv.Close()

			got := checkOllamaTools(srv.URL, "test-model")
			if got != tt.want {
				t.Errorf("checkOllamaTools() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckOllamaToolsConnectionError(t *testing.T) {
	// Unreachable server — should return false, not panic
	got := checkOllamaTools("http://127.0.0.1:1", "test-model")
	if got {
		t.Error("expected false for connection error")
	}
}

func TestCheckOllamaToolsStripsV1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/show" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		fmt.Fprint(w, `{"capabilities":["completion","tools"]}`)
	}))
	defer srv.Close()

	// Pass URL with /v1 suffix — should still hit /api/show correctly
	got := checkOllamaTools(srv.URL+"/v1", "test-model")
	if !got {
		t.Error("expected true when /v1 suffix is stripped correctly")
	}
}

// --- doctor() tests ---
// All tests use dummy providerCheck closures so no real provider logic runs.

func TestDoctorMarksOkCheck(t *testing.T) {
	checks := []providerCheck{
		{name: "fake", check: func() (bool, string) { return true, "all good" }},
	}
	var buf bytes.Buffer
	if err := doctor(&buf, "other", checks); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "✓") {
		t.Errorf("expected ✓ in output, got: %s", out)
	}
	if !strings.Contains(out, "all good") {
		t.Errorf("expected message in output, got: %s", out)
	}
}

func TestDoctorMarksFailingCheck(t *testing.T) {
	checks := []providerCheck{
		{name: "fake", check: func() (bool, string) { return false, "something wrong" }},
	}
	var buf bytes.Buffer
	_ = doctor(&buf, "other", checks)
	out := buf.String()
	if !strings.Contains(out, "✗") {
		t.Errorf("expected ✗ in output, got: %s", out)
	}
	if strings.Contains(out, "✓") {
		t.Errorf("unexpected ✓ in output, got: %s", out)
	}
}

func TestDoctorActiveProviderSuffix(t *testing.T) {
	checks := []providerCheck{
		{name: "alpha", check: func() (bool, string) { return true, "ok" }},
		{name: "beta", check: func() (bool, string) { return true, "ok" }},
	}
	var buf bytes.Buffer
	_ = doctor(&buf, "beta", checks)
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, "alpha") && strings.Contains(line, "(active)") {
			t.Errorf("alpha line should not be marked active: %s", line)
		}
		if strings.Contains(line, "beta") && !strings.Contains(line, "(active)") {
			t.Errorf("beta line should be marked active: %s", line)
		}
	}
}

func TestDoctorNoActiveWhenNoneMatch(t *testing.T) {
	checks := []providerCheck{
		{name: "alpha", check: func() (bool, string) { return true, "ok" }},
	}
	var buf bytes.Buffer
	_ = doctor(&buf, "other", checks)
	if strings.Contains(buf.String(), "(active)") {
		t.Errorf("expected no (active) suffix when active provider not in list")
	}
}

func TestDoctorMultipleChecks(t *testing.T) {
	checks := []providerCheck{
		{name: "p1", check: func() (bool, string) { return true, "msg1" }},
		{name: "p2", check: func() (bool, string) { return false, "msg2" }},
		{name: "p3", check: func() (bool, string) { return true, "msg3" }},
	}
	var buf bytes.Buffer
	_ = doctor(&buf, "p1", checks)
	out := buf.String()
	for _, name := range []string{"p1", "p2", "p3"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected %s in output", name)
		}
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// header line + blank line + 3 provider lines = at least 5 non-empty segments
	if len(lines) < 5 {
		t.Errorf("expected at least 5 lines, got %d: %s", len(lines), out)
	}
}

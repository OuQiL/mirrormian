package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaskKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"sk-abcdefgh1234", "sk-abc****h1234"},
		{"short", "****"},
		{"", "****"},
	}
	for _, c := range cases {
		if got := maskKey(c.in); got != c.want {
			t.Fatalf("maskKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSettingsGetMasked(t *testing.T) {
	// 在临时目录写 .env，并在该目录下调用 handler（envFilePath 用相对路径 .env）
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	if err := os.WriteFile(".env", []byte("LLM_API_KEY=sk-abcdefgh1234\nLLM_MODEL=mimo-v2.5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := &Server{}
	rec := httptest.NewRecorder()
	srv.handleSettingsGet(rec, httptest.NewRequest(http.MethodGet, "/api/settings", nil))

	var out map[string]providerSettings
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse: %v body=%s", err, rec.Body.String())
	}
	llm := out["llm"]
	if !llm.KeyConfigured {
		t.Fatal("key should be configured")
	}
	// 本地设置页：完整 key 可见可复制（用户决策）
	if llm.APIKey != "sk-abcdefgh1234" {
		t.Fatalf("full key should be returned: %q", llm.APIKey)
	}
	if llm.APIKeyMasked == "" || llm.APIKeyMasked == "sk-abcdefgh1234" {
		t.Fatalf("masked = %q", llm.APIKeyMasked)
	}
}

func TestSettingsSaveMerges(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	if err := os.WriteFile(".env", []byte("LLM_BASE_URL=https://old/v1\nKEEP=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := `{"llm":{"base_url":"https://new/v1","api_key":"sk-newkey","model":"new-model"}}`
	srv := &Server{}
	rec := httptest.NewRecorder()
	srv.handleSettingsSave(rec, httptest.NewRequest(http.MethodPost, "/api/settings/save", strings.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	raw, _ := os.ReadFile(filepath.Join(dir, ".env"))
	content := strings.ReplaceAll(string(raw), `"`, "") // godotenv.Write 会加引号包裹值
	if !strings.Contains(content, "LLM_BASE_URL=https://new/v1") || !strings.Contains(content, "KEEP=1") {
		t.Fatalf("merge failed:\n%s", content)
	}
	if !strings.Contains(content, "sk-newkey") {
		t.Fatalf("key not saved:\n%s", content)
	}
}

func TestSettingsSaveValidation(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	// 空 base_url 拒绝且不写文件
	body := `{"llm":{"base_url":"","model":"m"}}`
	srv := &Server{}
	rec := httptest.NewRecorder()
	srv.handleSettingsSave(rec, httptest.NewRequest(http.MethodPost, "/api/settings/save", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("should reject empty base_url: %d", rec.Code)
	}
	if _, err := os.Stat(".env"); !os.IsNotExist(err) {
		t.Fatal(".env should not be created on validation failure")
	}
}

func TestSettingsTestMock(t *testing.T) {
	// mock 外部服务：chat 与 embeddings 都返回 200
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"pong"}}]}`))
		case strings.HasSuffix(r.URL.Path, "/embeddings"):
			_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2,0.3]}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	body := `{"llm":{"base_url":"` + upstream.URL + `","api_key":"k","model":"m"},
		"embedding":{"base_url":"` + upstream.URL + `","api_key":"k","model":"e"}}`
	srv := &Server{}
	rec := httptest.NewRecorder()
	srv.handleSettingsTest(rec, httptest.NewRequest(http.MethodPost, "/api/settings/test", strings.NewReader(body)))

	var out map[string]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if out["llm"]["ok"] != true || out["embedding"]["ok"] != true {
		t.Fatalf("test result: %+v", out)
	}
	if out["embedding"]["dim"] != float64(3) {
		t.Fatalf("dim = %v", out["embedding"]["dim"])
	}
}

func TestSettingsTestFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer upstream.Close()

	body := `{"llm":{"base_url":"` + upstream.URL + `","api_key":"bad","model":"m"}}`
	srv := &Server{}
	rec := httptest.NewRecorder()
	srv.handleSettingsTest(rec, httptest.NewRequest(http.MethodPost, "/api/settings/test", strings.NewReader(body)))
	var out map[string]map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["llm"]["ok"] != false {
		t.Fatalf("should fail: %+v", out)
	}
	if errStr, _ := out["llm"]["error"].(string); !strings.Contains(errStr, "401") {
		t.Fatalf("error should mention status: %v", out["llm"]["error"])
	}
}

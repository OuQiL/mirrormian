package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadEnvFileMissing(t *testing.T) {
	env, err := ReadEnvFile(filepath.Join(t.TempDir(), "nope.env"))
	if err != nil || len(env) != 0 {
		t.Fatalf("missing file should be empty map: %v %v", env, err)
	}
}

func TestWriteEnvFileMerge(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := WriteEnvFile(path, map[string]string{"A": "1", "B": "2"}); err != nil {
		t.Fatal(err)
	}
	// 合并写：保留 B，更新 A，删除 C
	env, _ := ReadEnvFile(path)
	env["A"] = "new"
	delete(env, "B")
	env["C"] = "3"
	if err := WriteEnvFile(path, env); err != nil {
		t.Fatal(err)
	}
	got, _ := ReadEnvFile(path)
	if got["A"] != "new" || got["B"] != "" || got["C"] != "3" {
		t.Fatalf("merge failed: %+v", got)
	}
}

func TestWriteEnvFileContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := WriteEnvFile(path, map[string]string{"LLM_BASE_URL": "https://x/v1", "LLM_MODEL": "mimo-v2.5"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	content := strings.ReplaceAll(string(raw), `"`, "") // godotenv.Write 会加引号
	if !strings.Contains(content, "LLM_BASE_URL=https://x/v1") {
		t.Fatalf("env file content: %s", raw)
	}
}

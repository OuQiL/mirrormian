package llm

import (
	"context"
	"errors"
	"testing"
)

// fakeClient 测试桩：按脚本返回内容，验证 Client 契约。
type fakeClient struct {
	responses map[string]string // user 前缀 -> 回复
	err       error
}

func (f *fakeClient) Generate(_ context.Context, _, user string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if v, ok := f.responses[user]; ok {
		return v, nil
	}
	return `{"ok":true}`, nil
}

func (f *fakeClient) GenerateJSON(ctx context.Context, system, user string, out any) error {
	text, err := f.Generate(ctx, system, user)
	if err != nil {
		return err
	}
	return parseJSON(text, out)
}

func (f *fakeClient) ModelName() string { return "fake" }

func TestStripCodeFence(t *testing.T) {
	got := stripCodeFence("```json\n{\"a\":1}\n```")
	if got != `{"a":1}` {
		t.Fatalf("stripCodeFence = %q", got)
	}
}

func TestGenerateJSON(t *testing.T) {
	c := &fakeClient{responses: map[string]string{
		"ask-json": "```json\n{\"score\": 8, \"judge\": \"advance\"}\n```",
	}}
	var out struct {
		Score float64 `json:"score"`
		Judge string  `json:"judge"`
	}
	if err := c.GenerateJSON(context.Background(), "", "ask-json", &out); err != nil {
		t.Fatalf("GenerateJSON: %v", err)
	}
	if out.Score != 8 || out.Judge != "advance" {
		t.Fatalf("parsed = %+v", out)
	}
}

func TestGenerateJSONInvalid(t *testing.T) {
	c := &fakeClient{responses: map[string]string{"x": "not json at all"}}
	var out struct{ Score float64 `json:"score"` }
	if err := c.GenerateJSON(context.Background(), "", "x", &out); err == nil {
		t.Fatal("expected error for invalid json")
	}
}

func TestGenerateError(t *testing.T) {
	c := &fakeClient{err: errors.New("boom")}
	if _, err := c.Generate(context.Background(), "", "x"); err == nil {
		t.Fatal("expected error")
	}
}

package resume

import (
	"path/filepath"
	"strings"
	"testing"

	"mirror-mian/internal/store"
)

func newTestResume(t *testing.T) (*Service, store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "r.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return NewService(st), st
}

func TestAddTxtAndList(t *testing.T) {
	svc, _ := newTestResume(t)
	r, err := svc.Add("张三.txt", []byte("姓名：张三\n技能：Go、Redis"))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !strings.Contains(r.Text, "Go") {
		t.Fatalf("text = %q", r.Text)
	}
	list, err := svc.List()
	if err != nil || len(list) != 1 || list[0].Filename != "张三.txt" {
		t.Fatalf("list = %+v err=%v", list, err)
	}
	// Get 含文本
	got, _ := svc.Get(r.ID)
	if !strings.Contains(got.Text, "Redis") {
		t.Fatalf("get text = %q", got.Text)
	}
}

func TestAddUnsupported(t *testing.T) {
	svc, _ := newTestResume(t)
	if _, err := svc.Add("a.exe", []byte("x")); err == nil {
		t.Fatal("exe should be rejected")
	}
	if _, err := svc.Add("a.txt", make([]byte, MaxSize+1)); err == nil {
		t.Fatal("oversize should be rejected")
	}
}

func TestParseTextTxt(t *testing.T) {
	text, err := ParseText("txt", []byte("hello"))
	if err != nil || text != "hello" {
		t.Fatalf("txt parse: %q %v", text, err)
	}
	// 损坏 PDF
	if _, err := ParseText("pdf", []byte("not a pdf")); err == nil {
		t.Fatal("corrupt pdf should error")
	}
}

func TestDelete(t *testing.T) {
	svc, _ := newTestResume(t)
	r, _ := svc.Add("a.txt", []byte("x"))
	if err := svc.Delete(r.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(r.ID); err == nil {
		t.Fatal("should be gone")
	}
}

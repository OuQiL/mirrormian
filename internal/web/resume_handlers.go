// 简历管理接口（Web 交互层，无业务逻辑）。
package web

import (
	"io"
	"net/http"
	"strings"

	"mirror-mian/internal/resume"
)

// handleResumeList 简历列表（不含文本）。
func (s *Server) handleResumeList(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	list, err := uc.resume.List()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	type item struct {
		ID        string `json:"id"`
		Filename  string `json:"filename"`
		Ext       string `json:"ext"`
		SizeBytes int    `json:"size_bytes"`
		TextLen   int    `json:"text_len"`
		CreatedAt string `json:"created_at"`
	}
	out := make([]item, 0, len(list))
	for _, r := range list {
		out = append(out, item{
			ID: r.ID, Filename: r.Filename, Ext: r.Ext,
			SizeBytes: r.SizeBytes, TextLen: len(r.Text),
			CreatedAt: r.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	writeJSON(w, map[string]any{"resumes": out})
}

// handleResumeDetail 简历详情（含解析文本）。
func (s *Server) handleResumeDetail(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/resume/")
	if id == "" || strings.Contains(id, "/") {
		writeErr(w, http.StatusBadRequest, "缺少简历 ID")
		return
	}
	res, err := uc.resume.Get(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, res)
}

// handleResumeUpload 上传简历（multipart 字段 file）。
func (s *Server) handleResumeUpload(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	if err := r.ParseMultipartForm(resume.MaxSize + 1<<20); err != nil {
		writeErr(w, http.StatusBadRequest, "上传内容过大或格式错误")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "缺少 file 字段")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, resume.MaxSize+1))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := uc.resume.Add(header.Filename, data)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "id": res.ID, "filename": res.Filename})
}

// handleResumeText 粘贴文本上传简历（JSON：name/text）。
func (s *Server) handleResumeText(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	var req struct {
		Name string `json:"name"`
		Text string `json:"text"`
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := uc.resume.AddText(req.Name, req.Text)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "id": res.ID, "filename": res.Filename})
}

// handleResumeDelete 删除简历。
func (s *Server) handleResumeDelete(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/resume/")
	if id == "" || strings.Contains(id, "/") {
		writeErr(w, http.StatusBadRequest, "缺少简历 ID")
		return
	}
	if err := uc.resume.Delete(id); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// 领域管理接口（Web 交互层，无业务逻辑）。
package web

import (
	"context"
	"net/http"
	"time"
)

// handleDomainList 领域列表（统计）。
func (s *Server) handleDomainList(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	domains, err := uc.domain.List()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"domains": domains})
}

type nameReq struct {
	Name string `json:"name"`
}

// handleDomainCreate 创建领域。
func (s *Server) handleDomainCreate(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	var req nameReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := uc.domain.Create(req.Name); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "name": req.Name})
}

type topicReq struct {
	Topic string `json:"topic"`
}

// handleDomainDelete 删除领域。
func (s *Server) handleDomainDelete(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	var req topicReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := uc.domain.Delete(req.Topic); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

type renameReq struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

// handleDomainRename 重命名领域。
func (s *Server) handleDomainRename(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	var req renameReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := uc.domain.Rename(req.OldName, req.NewName); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// handleDomainDetail 领域详情：文件列表 + 文件内容。
func (s *Server) handleDomainDetail(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	var req topicReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	files, err := uc.domain.Files(req.Topic)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	contents := map[string]string{}
	for _, f := range files {
		c, err := uc.domain.ReadFile(req.Topic, f)
		if err == nil {
			contents[f] = c
		}
	}
	// 统计
	domains, _ := uc.domain.List()
	var stat any
	for _, d := range domains {
		if d.Name == req.Topic {
			stat = d
		}
	}
	writeJSON(w, map[string]any{"topic": req.Topic, "files": files, "contents": contents, "stats": stat})
}

type saveFileReq struct {
	Topic   string `json:"topic"`
	File    string `json:"file"`
	Content string `json:"content"`
}

// handleDomainSaveFile 保存领域文件。
func (s *Server) handleDomainSaveFile(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	var req saveFileReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := uc.domain.WriteFile(req.Topic, req.File, req.Content); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "saved": true, "need_sync": true})
}

// handleDomainGenerateCore 用 LLM 生成领域核心知识梳理（提示词对齐 TechSpar generateCore）。
func (s *Server) handleDomainGenerateCore(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	var req topicReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	content, err := uc.domain.GenerateCore(ctx, req.Topic)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "content": content})
}

// handleDomainSync 同步领域向量（可能较慢，超时放宽）。
func (s *Server) handleDomainSync(w http.ResponseWriter, r *http.Request) {
	uc := s.ucFor(w, r)
	if uc == nil {
		return
	}
	var req topicReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	n, err := uc.domain.Sync(ctx, req.Topic)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "chunks": n})
}

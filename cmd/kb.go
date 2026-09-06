// 知识库命令：kb import/list/search（交互层，无业务逻辑）。
package main

import (
	"context"
	"fmt"

	"mirror-mian/internal/config"
	"mirror-mian/internal/domain"
	"mirror-mian/internal/embedding"
	"mirror-mian/internal/llm"
	"mirror-mian/internal/rag"
	"mirror-mian/internal/store"
	"mirror-mian/internal/vector"
)

func cmdKB(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("kb 需要子命令：import <topic> <file> | list | search <topic> <query>")
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()
	emb := embedding.New(cfg)

	switch args[0] {
	case "import":
		return kbImport(cfg, st, emb, args[1:])
	case "list":
		return kbList(st)
	case "search":
		return kbSearch(st, emb, args[1:])
	default:
		return fmt.Errorf("未知 kb 子命令 %q", args[0])
	}
}

// kbImport 导入 Markdown/文本材料：写入领域目录（kb/<topic>/）+ 同步重建向量。
func kbImport(cfg *config.Config, st store.Store, emb embedding.Embedder, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("kb import 需要 <topic> <file>")
	}
	topic, path := args[0], args[1]
	if !emb.Available() {
		return fmt.Errorf("Embedding 未配置：请设置 LLM_EMBEDDING_BASE_URL/LLM_EMBEDDING_API_KEY/LLM_EMBEDDING_MODEL（或复用 LLM_API_KEY），参考 .env.example")
	}
	llmClient, _ := llm.New(cfg)
	milvus := vector.New(cfg.MilvusAddr)
	defer milvus.Close()
	svc := domain.NewService(cfg.KBPath, st, emb, llmClient)
	svc.SetMilvus(milvus)
	return kbImportFile(svc, topic, path)
}

func kbList(st store.Store) error {
	topics, err := st.ListAllTopics()
	if err != nil {
		return err
	}
	if len(topics) == 0 {
		fmt.Println("知识库为空。导入：mian kb import <topic> <file.md>")
		return nil
	}
	fmt.Println("知识库主题：")
	for topic, n := range topics {
		fmt.Printf("  %s: %d 块\n", topic, n)
	}
	return nil
}

func kbSearch(st store.Store, emb embedding.Embedder, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("kb search 需要 <topic> <query>")
	}
	topic, query := args[0], args[1]
	svc := rag.NewService(emb, st)
	results, err := svc.Retrieve(context.Background(), topic, query, 5)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Printf("主题 %q 无检索结果（空库或相似度不足）\n", topic)
		return nil
	}
	for i, r := range results {
		fmt.Printf("%d. [%.2f] %.200s\n", i+1, r.Score, r.Content)
	}
	return nil
}

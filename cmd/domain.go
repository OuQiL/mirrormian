// 领域管理命令：domain create/delete/rename/list/sync/files（交互层，无业务逻辑）。
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"mirror-mian/internal/config"
	"mirror-mian/internal/domain"
	"mirror-mian/internal/embedding"
	"mirror-mian/internal/llm"
	"mirror-mian/internal/store"
	"mirror-mian/internal/vector"
)

func cmdDomain(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("domain 需要子命令：create <name> | delete <name> | rename <old> <new> | list | sync <topic> | files <topic>")
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()
	llmClient, _ := llm.New(cfg)
	milvus := vector.New(cfg.MilvusAddr)
	defer milvus.Close()
	svc := domain.NewService(cfg.KBPath, st, embedding.New(cfg), llmClient)
	svc.SetMilvus(milvus)

	switch args[0] {
	case "create":
		if len(args) < 2 {
			return fmt.Errorf("domain create 需要领域名")
		}
		if err := svc.Create(args[1]); err != nil {
			return err
		}
		fmt.Printf("✓ 已创建领域 %q（kb/%s/），可用 domain files %s 查看或编辑内容后 domain sync %s 同步\n",
			args[1], args[1], args[1], args[1])
	case "delete":
		if len(args) < 2 {
			return fmt.Errorf("domain delete 需要领域名")
		}
		if err := svc.Delete(args[1]); err != nil {
			return err
		}
		fmt.Printf("✓ 已删除领域 %q（含知识库与画像数据）\n", args[1])
	case "rename":
		if len(args) < 3 {
			return fmt.Errorf("domain rename 需要 <旧名> <新名>")
		}
		if err := svc.Rename(args[1], args[2]); err != nil {
			return err
		}
		fmt.Printf("✓ 已重命名 %q → %q（知识库/画像已同步更新）\n", args[1], args[2])
	case "list":
		return domainList(svc)
	case "sync":
		if len(args) < 2 {
			return fmt.Errorf("domain sync 需要领域名")
		}
		n, err := svc.Sync(context.Background(), args[1])
		if err != nil {
			return err
		}
		fmt.Printf("✓ 已同步领域 %q：%d 块\n", args[1], n)
	case "files":
		if len(args) < 2 {
			return fmt.Errorf("domain files 需要领域名")
		}
		files, err := svc.Files(args[1])
		if err != nil {
			return err
		}
		fmt.Printf("领域 %q 的文件（kb/%s/）：\n", args[1], args[1])
		for _, f := range files {
			fmt.Printf("  %s\n", f)
		}
		fmt.Println("编辑文件后执行 domain sync 重建向量")
	case "generate":
		if len(args) < 2 {
			return fmt.Errorf("domain generate 需要领域名")
		}
		fmt.Printf("正在为领域 %q 生成核心知识梳理（TechSpar 提示词）...\n", args[1])
		content, err := svc.GenerateCore(context.Background(), args[1])
		if err != nil {
			return err
		}
		fmt.Printf("✓ 已生成并写入 kb/%s/README.md，同步完成（%d 字符）\n", args[1], len(content))
	default:
		return fmt.Errorf("未知 domain 子命令 %q", args[0])
	}
	return nil
}

func domainList(svc *domain.Service) error {
	domains, err := svc.List()
	if err != nil {
		return err
	}
	if len(domains) == 0 {
		fmt.Println("暂无领域。创建：mian domain create <name>")
		return nil
	}
	fmt.Println("领域列表：")
	for _, d := range domains {
		mastery := "-"
		if d.Stats.Mastery > 0 {
			mastery = fmt.Sprintf("%.1f", d.Stats.Mastery)
		}
		fmt.Printf("  %-20s 文件 %d · 知识块 %d · 训练 %d 场 · 掌握度 %s\n",
			d.Name, len(d.Files), d.ChunkCount, d.Stats.SessionCount, mastery)
	}
	return nil
}

// kbImportFile 兼容：kb import 写入领域目录 + 同步（原实现迁入 domain 体系）。
func kbImportFile(svc *domain.Service, topic, path string) error {
	topic = filepath.Base(topic)
	files, err := svc.Files(topic)
	if err != nil {
		// 领域不存在则创建
		if err := svc.Create(topic); err != nil {
			return err
		}
		files, _ = svc.Files(topic)
	}
	_ = files
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取文件: %w", err)
	}
	name := filepath.Base(path)
	if err := svc.WriteFile(topic, name, string(raw)); err != nil {
		return err
	}
	n, err := svc.Sync(context.Background(), topic)
	if err != nil {
		return err
	}
	fmt.Printf("✓ 已导入 %s → kb/%s/%s，同步 %d 块\n", path, topic, name, n)
	return nil
}

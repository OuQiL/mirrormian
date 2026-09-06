// 简历管理命令（交互层，无业务逻辑）。
package main

import (
	"fmt"
	"os"
	"strings"

	"mirror-mian/internal/config"
	"mirror-mian/internal/resume"
	"mirror-mian/internal/store"
)

func cmdResume(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("resume 需要子命令：add <file> | list | show <id> | delete <id>")
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()
	svc := resume.NewService(st)

	switch args[0] {
	case "add":
		if len(args) < 2 {
			return fmt.Errorf("resume add 需要文件路径")
		}
		data, err := os.ReadFile(args[1])
		if err != nil {
			return fmt.Errorf("读取文件: %w", err)
		}
		name := args[1]
		if i := strings.LastIndex(name, "\\"); i >= 0 {
			name = name[i+1:]
		}
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		fmt.Printf("解析并上传 %s ...\n", name)
		r, err := svc.Add(name, data)
		if err != nil {
			return err
		}
		fmt.Printf("✓ 已上传 %q（%s，解析 %d 字符）id=%s\n", r.Filename, r.Ext, len(r.Text), r.ID)
		return nil

	case "list":
		list, err := svc.List()
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("暂无简历。上传：mian resume add <file.pdf|docx|txt|md>")
			return nil
		}
		fmt.Println("简历列表：")
		for _, r := range list {
			fmt.Printf("  %s  %-20s %s  %d KB  %d 字符  %s\n",
				r.ID[:8], r.Filename, r.Ext, r.SizeBytes/1024, len(r.Text), r.CreatedAt.Format("2006-01-02 15:04"))
		}
		fmt.Println("\n查看：mian resume show <id>；删除：mian resume delete <id>；面试用：mian train full --jd <file> --resume-id <id>")
		return nil

	case "show":
		if len(args) < 2 {
			return fmt.Errorf("resume show 需要 id")
		}
		r, err := svc.Get(args[1])
		if err != nil {
			return err
		}
		fmt.Printf("=== %s（%s，%d 字符）===\n%s\n", r.Filename, r.Ext, len(r.Text), r.Text)
		return nil

	case "delete":
		if len(args) < 2 {
			return fmt.Errorf("resume delete 需要 id")
		}
		if err := svc.Delete(args[1]); err != nil {
			return err
		}
		fmt.Printf("✓ 已删除简历 %s\n", args[1])
		return nil

	default:
		return fmt.Errorf("未知 resume 子命令 %q", args[0])
	}
}

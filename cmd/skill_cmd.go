// 技能命令：skill list / skill run "<请求>" / skill <name>（交互层，无业务逻辑）。
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"mirror-mian/internal/config"
	"mirror-mian/internal/llm"
	"mirror-mian/internal/skill"
	"mirror-mian/internal/store"
)

// buildRegistry 装配技能注册中心（3 个内置技能）。
func buildRegistry(cfg *config.Config) (*skill.Registry, error) {
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	client, err := llm.New(cfg)
	if err != nil {
		st.Close()
		return nil, err
	}

	reg := skill.NewRegistry(st)
	reg.Register(
		skill.NewQuickQuiz(client, st),
		skill.NewConceptTutor(client),
		skill.NewReviewSkill(client, st),
	)
	return reg, nil
}

// cmdSkill 技能命令分发。
func cmdSkill(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("skill 需要子命令：list | run \"<请求>\" | <技能名>")
	}
	reg, err := buildRegistry(cfg)
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		for _, s := range reg.List() {
			fmt.Printf("  %-16s %s\n", s.Name(), s.Description())
		}
		fmt.Println("\n用法：skill run \"考考我 Redis\"（自动匹配）或 skill 快问快答（指定技能）")
		return nil

	case "run":
		if len(args) < 2 {
			return fmt.Errorf("skill run 需要请求文本，如：skill run \"考考我 Redis\"")
		}
		input := strings.Join(args[1:], " ")
		sess, res, err := reg.StartMatched(context.Background(), input)
		if err != nil {
			return err
		}
		return skillLoop(reg, sess, res)

	default:
		// 指定技能：skill <name> [input]
		input := ""
		if len(args) > 1 {
			input = strings.Join(args[1:], " ")
		}
		sess, res, err := reg.StartSkill(context.Background(), args[0], input)
		if err != nil {
			return err
		}
		return skillLoop(reg, sess, res)
	}
}

// skillLoop 交互循环（stdin）。
func skillLoop(reg *skill.Registry, sess *skill.Session, res *skill.TurnResult) error {
	fmt.Printf("\n[%s] %s\n", sess.SkillName, res.Reply)
	if res.Finished {
		return nil
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("\n你的输入（/quit 结束）：")
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("无法读取输入（%v）——请确认在交互终端中运行", err)
		}
		input := strings.TrimSpace(line)
		if input == "/quit" {
			fmt.Println("已结束会话")
			return nil
		}
		res, err := reg.Turn(context.Background(), sess.ID, input)
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		fmt.Printf("\n%s\n", res.Reply)
		if res.Finished {
			return nil
		}
	}
}

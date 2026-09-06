// 交互层入口：只做参数解析、调用编排层、展示输出，不包含业务逻辑。
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"mirror-mian/internal/config"
	"mirror-mian/internal/embedding"
	"mirror-mian/internal/llm"
	"mirror-mian/internal/mcp"
	"mirror-mian/internal/model"
	"mirror-mian/internal/orchestration"
	"mirror-mian/internal/profile"
	"mirror-mian/internal/store"
	"mirror-mian/internal/vector"
	"mirror-mian/internal/web"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	switch args[0] {
	case "run":
		return cmdRun(cfg)
	case "train":
		return cmdTrain(cfg, args[1:])
	case "review":
		return cmdReview(cfg, args[1:])
	case "profile":
		return cmdProfile(cfg)
	case "web":
		return cmdWeb(cfg, args[1:])
	case "kb":
		return cmdKB(cfg, args[1:])
	case "domain":
		return cmdDomain(cfg, args[1:])
	case "resources":
		return cmdResources(cfg, args[1:])
	case "analyze":
		return cmdAnalyze(cfg, args[1:])
	case "plan":
		return cmdPlan(cfg)
	case "resume":
		return cmdResume(cfg, args[1:])
	case "skill":
		return cmdSkill(cfg, args[1:])
	case "help", "-h", "--help":
		return usage()
	default:
		return usage()
	}
}

func usage() error {
	return fmt.Errorf(`usage: mian <command> [args]

命令：
  run                     健康自检（配置 + LLM 连通性 + 数据库）
  train special <topic>   专项面试：按主题训练
  train full --jd <file> [--resume <file>]   综合面试：按 JD/简历训练
  review [--score <id> <0-10>]   查询到期复习项，或对某项提交复习评分
  profile                 查看画像（掌握度/薄弱点）
  web [-addr :8012]       启动 Web 交互界面（浏览器访问）
  kb import <topic> <file>   导入知识库材料（Markdown/文本，写入领域目录并同步）
  kb list / kb search <topic> <query>   查看/检索知识库
  domain create|delete|rename <name>   领域 CRUD
  domain list / sync <topic> / files <topic>   领域统计/同步向量/文件列表
  domain generate <topic>   用 AI 生成核心知识梳理（TechSpar 提示词，8-12 个知识点）
  resources <topic> [count]   按主题推荐 GitHub 学习资源（需 GITHUB_TOKEN）
  analyze --jd <file> --resume <file>   JD/简历匹配度分析（对位/评分/差距）
  plan                    生成今日复习计划（基于画像）
  skill list / run "<请求>" / <技能名>  技能系统（快问快答/复习计划/知识学习）
  help                    显示本帮助

环境变量（.env 或环境）：
  LLM_BASE_URL / LLM_API_KEY / LLM_MODEL / MIRROR_DB_PATH
  LLM_EMBEDDING_BASE_URL / LLM_EMBEDDING_API_KEY / LLM_EMBEDDING_MODEL（可选，缺省复用 LLM）
  GITHUB_TOKEN（可选，resources 命令与 Web 资源推荐用）`)
}

// --- 交互：提问并读取回答（编排层的 ask 回调，本层只做收发） ---

func askQuestion(reader *bufio.Reader, q model.Question) (string, error) {
	label := "问题"
	if q.Round > 0 {
		label = fmt.Sprintf("追问%d", q.Round)
	}
	fmt.Printf("\n【%s】（%s）\n%s\n", label, q.KnowledgePt, q.Text)
	fmt.Print("你的回答（输入 /quit 结束面试）：")
	line, err := reader.ReadString('\n')
	if err != nil {
		if err == bufio.ErrBufferFull {
			// 超长输入：取缓冲内的内容继续处理
			line = string(line)
		} else {
			// EOF（stdin 关闭）等：说明不是交互终端
			return "", fmt.Errorf("无法读取输入（%v）——请确认在交互终端中运行本命令（不要在管道/非交互环境执行）", err)
		}
	}
	answer := strings.TrimSpace(line)
	if answer == "/quit" || answer == "" {
		return "", fmt.Errorf("面试终止")
	}
	return answer, nil
}

// --- 命令实现 ---

func cmdRun(cfg *config.Config) error {
	fmt.Println("健康自检...")
	if err := cfg.Validate(); err != nil {
		return err
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("数据库: %w", err)
	}
	defer st.Close()
	fmt.Printf("✓ 配置 OK（model=%s, db=%s）\n", cfg.LLMModel, cfg.DBPath)

	client, err := llm.New(cfg)
	if err != nil {
		return fmt.Errorf("LLM: %w", err)
	}
	if cfg.LLMAPIKey == "" {
		fmt.Println("⚠ LLM API Key 未配置（LLM_API_KEY），编排功能将不可用")
	} else {
		fmt.Printf("✓ LLM 配置 OK（%s）\n", client.ModelName())
	}
	return nil
}

func cmdTrain(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("train 需要子命令：special <topic> 或 full --jd <file> [--resume <file>]")
	}
	// 先校验参数（clear errors），再校验 LLM 配置
	var topic, jdPath, jdURL, resumePath, resumeID string
	switch args[0] {
	case "special":
		if len(args) < 2 || args[1] == "" {
			return fmt.Errorf("train special 需要主题参数：train special <topic>")
		}
		topic = args[1]
	case "full":
		jd, jdUrl, resume, rid, err := parseFullFlags(args[1:])
		if err != nil {
			return err
		}
		jdPath, jdURL, resumePath, resumeID = jd, jdUrl, resume, rid
	default:
		return fmt.Errorf("未知 train 子命令 %q（支持 special / full）", args[0])
	}
	if cfg.LLMAPIKey == "" {
		return fmt.Errorf("LLM_API_KEY 未配置，无法开始面试（请设置 .env 或环境变量）")
	}
	client, err := llm.New(cfg)
	if err != nil {
		return err
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()

	milvus := vector.New(cfg.MilvusAddr)
	defer milvus.Close()
	svc := orchestration.NewService(client, st, embedding.New(cfg), cfg.KBPath)
	svc.SetMilvus(milvus)
	reader := bufio.NewReader(os.Stdin)
	ask := func(q model.Question) (string, error) { return askQuestion(reader, q) }

	var sess *model.TrainingSession
	switch args[0] {
	case "special":
		fmt.Printf("\n=== 专项面试：%s ===\n", topic)
		sess, err = svc.RunSpecial(context.Background(), topic, ask)
	case "full":
		var jdText []byte
		if jdURL != "" {
			// JD URL：抓取页面文本
			fmt.Printf("抓取 JD 页面：%s ...\n", jdURL)
			jdText, err = fetchURL(jdURL)
			if err != nil {
				return err
			}
		} else {
			jdText, err = os.ReadFile(jdPath)
			if err != nil {
				return fmt.Errorf("读取 JD 文件: %w", err)
			}
		}
		var resumeText []byte
		if resumeID != "" {
			// 从简历库取文本
			r, err := st.GetResume(resumeID)
			if err != nil {
				return fmt.Errorf("读取简历库: %w", err)
			}
			fmt.Printf("使用已上传简历：%s\n", r.Filename)
			resumeText = []byte(r.Text)
		} else if resumePath != "" {
			resumeText, err = os.ReadFile(resumePath)
			if err != nil {
				return fmt.Errorf("读取简历文件: %w", err)
			}
		}
		fmt.Println("\n=== 综合面试（JD 驱动）===")
		sess, err = svc.RunFull(context.Background(), string(resumeText), string(jdText), ask)
	}
	if err != nil {
		return err
	}
	return showSession(sess)
}

// parseFullFlags 解析 full 子命令的 --jd / --jd-url / --resume / --resume-id 参数。
func parseFullFlags(args []string) (jd, jdURL, resume, resumeID string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--jd":
			if i+1 >= len(args) {
				return "", "", "", "", fmt.Errorf("--jd 缺少文件路径")
			}
			jd = args[i+1]
			i++
		case "--jd-url":
			if i+1 >= len(args) {
				return "", "", "", "", fmt.Errorf("--jd-url 缺少 URL")
			}
			jdURL = args[i+1]
			i++
		case "--resume":
			if i+1 >= len(args) {
				return "", "", "", "", fmt.Errorf("--resume 缺少文件路径")
			}
			resume = args[i+1]
			i++
		case "--resume-id":
			if i+1 >= len(args) {
				return "", "", "", "", fmt.Errorf("--resume-id 缺少 ID")
			}
			resumeID = args[i+1]
			i++
		default:
			return "", "", "", "", fmt.Errorf("未知参数 %q", args[i])
		}
	}
	if jd == "" && jdURL == "" {
		return "", "", "", "", fmt.Errorf("综合面试必须提供 --jd <file> 或 --jd-url <url>")
	}
	return jd, jdURL, resume, resumeID, nil
}

// fetchURL 用 Playwright MCP 抓取页面文本（JD URL 备面用）。
func fetchURL(url string) ([]byte, error) {
	scraper, err := mcp.NewWebScraper()
	if err != nil {
		return nil, fmt.Errorf("JD URL 抓取不可用（%v）——可改用 --jd <file> 粘贴方式", err)
	}
	defer scraper.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	text, err := scraper.Fetch(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("抓取 JD 页面失败（%v）——可改用 --jd <file> 粘贴方式", err)
	}
	return []byte(text), nil
}

func showSession(sess *model.TrainingSession) error {
	fmt.Printf("\n=== 面试完成（会话 %s）===\n", sess.ID)
	fmt.Printf("题目 %d 道，平均分 %.1f\n",
		len(sess.Questions), avgScore(sess.Answers))
	if sess.Review != "" {
		fmt.Printf("\n--- 复盘 ---\n%s\n", sess.Review)
	}
	return nil
}

func avgScore(answers []model.Answer) float64 {
	if len(answers) == 0 {
		return 0
	}
	var sum float64
	for _, a := range answers {
		sum += a.Score
	}
	return sum / float64(len(answers))
}

func cmdReview(cfg *config.Config, args []string) error {
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()
	p, err := st.GetProfile()
	if err != nil {
		return err
	}

	// 提交复习评分：review --score <id> <0-10>
	if len(args) >= 3 && args[0] == "--score" {
		return reviewSubmit(st, p, args[1], args[2])
	}

	due := profile.DueReviews(p.WeakPoints, time.Now())
	if len(due) == 0 {
		fmt.Println("没有到期的复习项 🎉")
		return nil
	}
	fmt.Printf("到期复习项 %d 条（按难度优先排序）：\n", len(due))
	for i, w := range due {
		fmt.Printf("  %d. [%s] %s（出现 %d 次，难度 %.2f）id=%s\n",
			i+1, w.Topic, w.Point, w.TimesSeen, w.SR.EaseFactor, w.ID)
	}
	fmt.Println("\n提交评分：mian review --score <id> <0-10>")
	return nil
}

func reviewSubmit(st store.Store, p *model.Profile, id, scoreStr string) error {
	var score float64
	if _, err := fmt.Sscanf(scoreStr, "%f", &score); err != nil || score < 0 || score > 10 {
		return fmt.Errorf("评分必须是 0-10 的数字")
	}
	for i := range p.WeakPoints {
		if p.WeakPoints[i].ID != id {
			continue
		}
		profile.ReviewWeakPoint(&p.WeakPoints[i], score, time.Now())
		if err := st.SaveProfile(p); err != nil {
			return err
		}
		fmt.Printf("已提交复习评分 %.0f：%s\n", score, p.WeakPoints[i].Point)
		if p.WeakPoints[i].Improved {
			fmt.Println("该薄弱点已标记为改进，不再进入复习队列")
		} else {
			fmt.Printf("下次复习：%s\n", p.WeakPoints[i].SR.NextReview)
		}
		return nil
	}
	return fmt.Errorf("未找到薄弱点 %s", id)
}

func cmdWeb(cfg *config.Config, args []string) error {
	addr := ":8080"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-addr":
			if i+1 >= len(args) {
				return fmt.Errorf("-addr 缺少端口参数")
			}
			addr = args[i+1]
			i++
		default:
			return fmt.Errorf("未知参数 %q（支持 -addr :8080）", args[i])
		}
	}
	// 允许未配置 key 启动：设置页可可视化配置（保存 .env 后重启生效）
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()
	srv := web.NewServer(cfg, st)
	return srv.Start(addr)
}

func cmdProfile(cfg *config.Config) error {
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()
	p, err := st.GetProfile()
	if err != nil {
		return err
	}
	fmt.Println("=== 画像 ===")
	if len(p.Mastery) == 0 {
		fmt.Println("（暂无掌握度数据，先跑一场面试）")
	} else {
		fmt.Println("掌握度（0-100）：")
		for topic, m := range p.Mastery {
			fmt.Printf("  %s: %.1f（%d 场会话）\n", topic, m.Score, m.SessionCount)
		}
	}
	if len(p.WeakPoints) == 0 {
		fmt.Println("（暂无薄弱点记录）")
	} else {
		fmt.Println("薄弱点：")
		for _, w := range p.WeakPoints {
			state := ""
			switch {
			case w.Improved:
				state = "已改进"
			case w.Archived:
				state = "已归档"
			default:
				state = fmt.Sprintf("下次复习 %s", w.SR.NextReview)
			}
			fmt.Printf("  [%s] %s（出现 %d 次，%s）\n", w.Topic, w.Point, w.TimesSeen, state)
		}
	}
	return nil
}

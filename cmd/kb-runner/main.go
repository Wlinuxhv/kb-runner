package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"kb-runnerx/internal/adapter"
	"kb-runnerx/internal/api"
	"kb-runnerx/internal/cases"
	"kb-runnerx/internal/executor"
	"kb-runnerx/internal/kbscripts"
	"kb-runnerx/internal/preprocessor"
	"kb-runnerx/internal/processor"
	"kb-runnerx/internal/scenario"
	"kb-runnerx/pkg/config"
	"kb-runnerx/pkg/logger"
	"kb-runnerx/pkg/result"
)

var (
	cfgFile      string
	verbose      bool
	quiet        bool
	scriptPath   string
	language     string
	params       []string
	timeout      int
	parallel     int
	outputDir    string
	outputFormat string
	weight       float64
	caseName     string
	scenarioName string
	interactive  bool
	category     string
	tags         []string
	searchTerm   string
	serveHost    string
	servePort    int
	serveToken   string
	runMode      string
	offlineQNo   string
	offlineRoot  string

	// 新增参数
	qno             string
	preprocess      bool
	unpreprocesslog bool
	processedPath   string
	runAll          bool
	kbList          string
	resultDir       string
	logPassword     string
)

var rootCmd = &cobra.Command{
	Use:   "kb-runner",
	Short: "KB脚本执行框架",
	Long: `KB脚本执行框架 - 用于执行KB检查脚本并生成结果矩阵

支持多种脚本语言（Bash、Python），提供标准化的结果格式和权重计算。

使用方式:
  kb-runner list              列出所有可用的CASE
  kb-runner show <case-name>  查看CASE详情
  kb-runner scenario list     列出所有场景
  kb-runner run [options]     执行脚本或CASE`,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有可用的CASE",
	Long: `列出所有已注册的CASE，支持按分类、标签筛选和搜索。

示例:
  kb-runner list
  kb-runner list --category security
  kb-runner list --tags critical,daily
  kb-runner list --search "check"`,
	RunE: listCases,
}

var showCmd = &cobra.Command{
	Use:   "show <case-name>",
	Short: "显示CASE详细信息",
	Long: `显示指定CASE的详细配置信息，包括脚本路径、参数、权重等。

示例:
  kb-runner show security_check`,
	Args: cobra.ExactArgs(1),
	RunE: showCase,
}

var scenarioCmd = &cobra.Command{
	Use:   "scenario",
	Short: "场景管理命令",
}

var scenarioListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有场景",
	RunE:  listScenarios,
}

var scenarioShowCmd = &cobra.Command{
	Use:   "show <scenario-name>",
	Short: "显示场景详情",
	Args:  cobra.ExactArgs(1),
	RunE:  showScenario,
}

var runCmd = &cobra.Command{
	Use:   "run [script-paths...]",
	Short: "执行KB脚本",
	Long: `执行一个或多个KB脚本，支持多种执行方式：

1. 直接执行脚本文件:
   kb-runner run -s ./scripts/check.sh -l bash

2. 按CASE名称执行:
   kb-runner run --case security_check

3. 按场景执行:
   kb-runner run --scenario daily_check

4. 交互式选择执行:
   kb-runner run --interactive

5. 批量执行:
   kb-runner run -s ./scripts/*.sh -l bash -n 5`,
	RunE: runScripts,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("kb-runner version 1.0.0")
	},
}

var kbInitCmd = &cobra.Command{
	Use:   "kbinit <kb-name>",
	Short: "创建 KB 目录模板",
	Long: `创建一个标准化的 KB 目录结构，包含配置文件、脚本模板和 Skill.md。

示例:
  kb-runner kbinit my_kb-00001                    # 创建默认 Bash KB
  kb-runner kbinit my_kb-00001 --lang python      # 创建 Python KB
  kb-runner kbinit my_kb-00001 --output ./kbscript # 指定输出目录`,
	Args: cobra.ExactArgs(1),
	RunE: kbInitCase,
}

var kbCheckCmd = &cobra.Command{
	Use:   "kbcheck [kb-name]",
	Short: "检查 KB 脚本和配置文件",
	Long: `检查 KB 脚本和 Skill.md 是否符合要求，包括：
- 文件完整性检查
- Skill.md 内容检查
- run.sh 规范性检查
- case.yaml 配置检查
- Offline 模式处理检查

示例:
  kb-runner kbcheck                        # 检查所有 KB
  kb-runner kbcheck my_kb-00001            # 检查指定 KB
  kb-runner kbcheck --verbose              # 详细输出`,
	RunE: kbCheck,
}

var initCmd = &cobra.Command{
	Use:        "init <case-name>",
	Short:      "创建 CASE 目录模板 (已废弃，请使用 kbinit)",
	Long:       `创建一个标准化的 CASE 目录结构，包含配置文件和脚本模板。`,
	Args:       cobra.ExactArgs(1),
	RunE:       initCase,
	Deprecated: "请使用 kbinit 命令",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "启动Web服务",
	Long: `启动内嵌的Web服务，提供图形化界面和RESTful API。

示例:
  kb-runner serve                    # 默认端口8080
  kb-runner serve --port 9090        # 指定端口
  kb-runner serve --host 127.0.0.1   # 指定监听地址
  kb-runner serve --token mytoken     # 指定访问Token`,
	RunE: runServe,
}

var preprocessCmd = &cobra.Command{
	Use:   "preprocess",
	Short: "日志预处理",
	Long: `预处理日志包，解压加密的ZIP文件。

示例:
  kb-runner preprocess Q2026031201098     # 预处理指定Q单号的日志
  kb-runner preprocess Q2026031201098 --root ./workspace/icare_log/logall`,
	Args: cobra.ExactArgs(1),
	RunE: preprocessLog,
}

var (
	preprocessRoot string
)

var (
	initLanguage string
	initOutput   string
	initTemplate string
)

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "配置文件路径")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "详细输出")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "静默模式")

	listCmd.Flags().StringVarP(&category, "category", "C", "", "按分类筛选")
	listCmd.Flags().StringSliceVarP(&tags, "tags", "T", nil, "按标签筛选")
	listCmd.Flags().StringVarP(&searchTerm, "search", "S", "", "搜索CASE")
	listCmd.Flags().StringVarP(&outputFormat, "format", "f", "table", "输出格式 (table/json)")

	scenarioCmd.AddCommand(scenarioListCmd)
	scenarioCmd.AddCommand(scenarioShowCmd)

	runCmd.Flags().StringVarP(&scriptPath, "script", "s", "", "脚本文件路径")
	runCmd.Flags().StringVarP(&language, "language", "l", "bash", "脚本语言类型")
	runCmd.Flags().StringArrayVarP(&params, "param", "p", nil, "脚本参数 (key=value)")
	runCmd.Flags().IntVarP(&timeout, "timeout", "t", 300, "执行超时时间(秒)")
	runCmd.Flags().IntVarP(&parallel, "parallel", "n", 1, "并行执行数量")
	runCmd.Flags().StringVarP(&outputDir, "output", "o", "", "结果输出目录")
	runCmd.Flags().StringVarP(&outputFormat, "format", "f", "json", "输出格式 (json/yaml/table)")
	runCmd.Flags().Float64VarP(&weight, "weight", "w", 1.0, "脚本权重")
	runCmd.Flags().StringVar(&caseName, "case", "", "按CASE名称执行")
	runCmd.Flags().StringVar(&scenarioName, "scenario", "", "按场景执行")
	runCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "交互式选择执行")
	runCmd.Flags().StringVar(&category, "category", "", "执行指定分类的所有CASE")
	runCmd.Flags().StringSliceVar(&tags, "tags", nil, "执行指定标签的所有CASE")
	runCmd.Flags().StringVar(&runMode, "mode", "online", "运行模式 (online/offline)")
	runCmd.Flags().StringVar(&offlineQNo, "qno", "", "离线日志包 Q 单号（用于 offline 模式自动解压定位）")
	runCmd.Flags().StringVar(&offlineRoot, "log-root", "./workspace/icare_log/logall", "离线日志根目录（用于 offline 模式）")

	// 新增参数
	runCmd.Flags().StringVar(&qno, "Q", "", "Q 单号（offline 模式必需）")
	runCmd.Flags().BoolVar(&preprocess, "preprocess", true, "启用日志预处理（默认启用）")
	runCmd.Flags().BoolVar(&unpreprocesslog, "unpreprocesslog", false, "禁用日志预处理")
	runCmd.Flags().StringVar(&processedPath, "processed-path", "", "已处理的日志路径（禁用预处理时必需）")
	runCmd.Flags().BoolVar(&runAll, "all", false, "执行所有 KB 脚本")
	runCmd.Flags().StringVar(&kbList, "kb", "", "执行指定的 KB 列表（逗号分隔）")
	runCmd.Flags().StringVar(&resultDir, "result-dir", "", "结果输出目录（覆盖配置）")
	runCmd.Flags().StringVar(&logPassword, "log-password", "", "日志包密码（可选，覆盖默认密码）")

	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(scenarioCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(kbInitCmd)
	rootCmd.AddCommand(kbCheckCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(preprocessCmd)

	kbInitCmd.Flags().StringVarP(&initLanguage, "language", "l", "bash", "脚本语言 (bash/python)")
	kbInitCmd.Flags().StringVarP(&initOutput, "output", "o", "./kbscript", "输出目录")
	kbInitCmd.Flags().StringVar(&initTemplate, "template", "default", "模板类型")

	initCmd.Flags().StringVarP(&initLanguage, "language", "l", "bash", "脚本语言 (bash/python)")
	initCmd.Flags().StringVarP(&initOutput, "output", "o", "./cases", "输出目录")
	initCmd.Flags().StringVar(&initTemplate, "template", "default", "模板类型")

	serveCmd.Flags().StringVarP(&serveHost, "host", "H", "0.0.0.0", "监听地址")
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 8080, "监听端口")
	serveCmd.Flags().StringVar(&serveToken, "token", "", "访问 Token")

	preprocessCmd.Flags().StringVar(&preprocessRoot, "root", "./workspace/icare_log/logall", "日志根目录")
}

func preprocessLog(cmd *cobra.Command, args []string) error {
	qno := args[0]

	fmt.Printf("开始预处理日志包: %s\n", qno)

	p := preprocessor.NewLogPreprocessor()
	p.RootPath = preprocessRoot
	p.ConfigPath = "./config/icare_log.json"

	if err := p.Init(qno); err != nil {
		return fmt.Errorf("初始化预处理失败: %w", err)
	}

	if err := p.Process(); err != nil {
		return fmt.Errorf("预处理失败: %w", err)
	}

	fmt.Printf("\n预处理完成!\n")
	fmt.Printf("日志路径: %s\n", p.GetQNoPath())
	if p.GetExtractPath() != "" {
		fmt.Printf("解压路径: %s\n", p.GetExtractPath())
	}

	return nil
}

func initConfig() {
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func listCases(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	caseManager := cases.NewManager()
	if err := loadCases(cfg, caseManager); err != nil {
		return err
	}

	opts := cases.FilterOptions{
		Category: category,
		Tags:     tags,
		Search:   searchTerm,
	}

	cases := caseManager.List(opts)

	switch outputFormat {
	case "json":
		return outputCaseListJSON(cases)
	default:
		return outputCaseListTable(cases)
	}
}

func showCase(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	caseManager := cases.NewManager()
	if err := loadCases(cfg, caseManager); err != nil {
		return err
	}

	c, err := caseManager.Get(args[0])
	if err != nil {
		return err
	}

	return outputCaseDetail(c)
}

func listScenarios(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	scenarioManager := scenario.NewManager()
	if err := loadScenarios(cfg, scenarioManager); err != nil {
		return err
	}

	scenarios := scenarioManager.List()
	return outputScenarioList(scenarios)
}

func showScenario(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	scenarioManager := scenario.NewManager()
	if err := loadScenarios(cfg, scenarioManager); err != nil {
		return err
	}

	s, err := scenarioManager.Get(args[0])
	if err != nil {
		return err
	}

	return outputScenarioDetail(s)
}

func runScripts(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// 检查并解压 KB 脚本
	if cfg.Scripts.EmbeddedKBEnabled {
		kbScriptDir := cfg.Scripts.GetKBscriptDirectory()
		if kbscripts.ShouldExtract(kbScriptDir) {
			fmt.Printf("KB scripts not found, extracting to: %s\n", kbScriptDir)
			if err := kbscripts.Extract(kbScriptDir); err != nil {
				return fmt.Errorf("failed to extract KB scripts: %w", err)
			}
			fmt.Println("KB scripts extracted successfully")
		}
	}

	log, err := createLoggerWithModule(cfg, "runner")
	if err != nil {
		return err
	}

	engine := executor.NewEngine(cfg, log)
	registerAdapters(cfg, engine)

	caseManager := cases.NewManager()
	if err := loadCases(cfg, caseManager); err != nil {
		log.Warn("Failed to load cases", "error", err)
	}

	scenarioManager := scenario.NewManager()
	if err := loadScenarios(cfg, scenarioManager); err != nil {
		log.Warn("Failed to load scenarios", "error", err)
	}

	var tasks []*adapter.Task

	// 处理 -s 参数
	if scriptPath != "" {
		args = append(args, scriptPath)
	}

	// 支持新的批量执行参数
	if runAll || kbList != "" || (qno != "" && category == "" && len(tags) == 0) {
		tasks, err = buildTasksForBatch(caseManager, cfg)
	} else {
		switch {
		case interactive:
			tasks, err = interactiveSelect(caseManager, cfg)
		case scenarioName != "":
			tasks, err = buildTasksFromScenario(scenarioManager, caseManager, scenarioName, cfg)
		case caseName != "":
			tasks, err = buildTasksFromCase(caseManager, caseName, cfg)
		case category != "":
			tasks, err = buildTasksFromCategory(caseManager, category, cfg)
		case len(tags) > 0:
			tasks, err = buildTasksFromTags(caseManager, tags, cfg)
		default:
			tasks = buildTasks(args, cfg)
		}
	}

	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		return fmt.Errorf("no tasks to execute")
	}

	// offline 模式：注入 KB_RUN_MODE，并在给定 Q 单号时自动预处理解压，注入离线路径
	targetQNo := ""
	if strings.ToLower(runMode) == "offline" || qno != "" || offlineQNo != "" {
		// 确定 Q 单号：优先使用新参数 --Q，否则使用 --qno
		targetQNo = qno
		if targetQNo == "" {
			targetQNo = offlineQNo
		}

		var qnoDir string

		// 判断是否需要预处理
		needPreprocess := preprocess && !unpreprocesslog

		if needPreprocess && targetQNo != "" {
			p := preprocessor.NewLogPreprocessor()
			p.RootPath = offlineRoot
			p.ConfigPath = "./config/icare_log.json"

			// 使用自定义密码（如果提供）
			if logPassword != "" {
				p.ArchivePassword = logPassword
			} else {
				p.ArchivePassword = cfg.GetArchivePassword()
			}

			if err := p.Init(targetQNo); err != nil {
				return fmt.Errorf("offline preprocess init failed: %w", err)
			}
			if err := p.Process(); err != nil {
				// 预处理失败，提示用户选择
				fmt.Printf("日志预处理失败：%v\n", err)
				fmt.Println("请选择：[c] 继续（使用已解压目录） [q] 退出")
				var choice string
				fmt.Scanln(&choice)
				if choice == "q" || choice == "Q" {
					return fmt.Errorf("用户选择退出")
				}
			}
			// 解压根目录：优先 extract dir，否则用 qno path
			qnoDir = p.GetExtractPath()
			if qnoDir == "" {
				qnoDir = p.GetQNoPath()
			}
		} else if unpreprocesslog {
			// 禁用预处理，使用指定的已处理路径
			if processedPath == "" && targetQNo == "" {
				return fmt.Errorf("--unpreprocesslog 必须提供 --processed-path 或 --Q")
			}
			if processedPath != "" {
				qnoDir = processedPath
			} else if targetQNo != "" {
				// 使用默认路径
				qnoDir = filepath.Join(offlineRoot, targetQNo[len("Q"):len(targetQNo)-2], targetQNo)
				if _, err := os.Stat(qnoDir); os.IsNotExist(err) {
					return fmt.Errorf("指定的日志目录不存在：%s", qnoDir)
				}
			}
		}

		hosts, logDirs := discoverOfflineHostsAndLogDirs(qnoDir)
		host := ""
		hostDir := ""
		logDir := ""
		if len(hosts) > 0 {
			host = hosts[0]
			hostDir = filepath.Join(qnoDir, host)
		}
		if len(logDirs) > 0 {
			logDir = logDirs[0]
		}

		var hostsJSON []byte
		var logDirsJSON []byte
		if len(hosts) > 0 {
			hostsJSON, _ = json.Marshal(hosts)
		}
		if len(logDirs) > 0 {
			logDirsJSON, _ = json.Marshal(logDirs)
		}

		icareRoot := ""
		if qnoDir != "" {
			// qnoDir: .../icare_log/logall/<yearMonth>/<QNo>
			// ICARE_LOG_ROOT: .../icare_log/logall
			icareRoot = filepath.Dir(filepath.Dir(qnoDir))
		}

		for _, t := range tasks {
			if t.Env == nil {
				t.Env = make(map[string]string)
			}
			t.Env["KB_RUN_MODE"] = "offline"
			if qnoDir != "" {
				t.Env["KB_OFFLINE_QNO_DIR"] = qnoDir
			}
			if offlineQNo != "" {
				t.Env["KB_OFFLINE_QNO"] = offlineQNo
			}
			if icareRoot != "" {
				t.Env["KB_OFFLINE_ICARE_LOG_ROOT"] = icareRoot
			}
			if host != "" {
				t.Env["KB_OFFLINE_HOST"] = host
			}
			if hostDir != "" {
				t.Env["KB_OFFLINE_HOST_DIR"] = hostDir
			}
			if logDir != "" {
				t.Env["KB_OFFLINE_LOG_DIR"] = logDir
			}
			if len(hostsJSON) > 0 {
				t.Env["KB_OFFLINE_HOSTS_JSON"] = string(hostsJSON)
			}
			if len(logDirsJSON) > 0 {
				t.Env["KB_OFFLINE_LOG_DIRS_JSON"] = string(logDirsJSON)
			}
		}
	} else {
		for _, t := range tasks {
			if t.Env == nil {
				t.Env = make(map[string]string)
			}
			t.Env["KB_RUN_MODE"] = "online"
		}
	}

	execResults, err := engine.ExecuteBatch(ctx, tasks)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	proc := processor.NewProcessor(cfg, log)
	matrix, err := proc.Process(execResults)
	if err != nil {
		return fmt.Errorf("result processing failed: %w", err)
	}

	// 保存历史记录
	if cfg.History.Enabled {
		if err := saveHistory(cfg, matrix, caseName, scenarioName); err != nil {
			log.Warn("Failed to save history", "error", err)
		}
	}

	// 保存结果到配置的目录（offline 模式或指定 Q 单号时）
	if targetQNo != "" {
		resultRoot := cfg.GetResultRoot()
		if resultDir != "" {
			resultRoot = resultDir
		}

		// 目录格式：Q{单号}-{时间戳}
		// 例如：Q2026031700281-20260327094611
		timestamp := time.Now().Format("20060102150405")
		qnoDirName := fmt.Sprintf("%s-%s", targetQNo, timestamp)
		resultQNoDir := filepath.Join(resultRoot, qnoDirName)

		// 确保目录存在
		if err := os.MkdirAll(resultQNoDir, 0755); err != nil {
			log.Warn("Failed to create result directory", "error", err)
		} else {
			log.Info("Result directory created", "dir", resultQNoDir)
			// 保存单个 KB 结果
			for _, script := range matrix.Scripts {
				// 从 script.Name 或 script.Results 中提取真实的 KB ID
				kbID := extractKBID(script.Name, script.Results)

				// 文件名格式：kb{ID}-{exec_id}_result.json
				filename := fmt.Sprintf("kb%s-%s_result.json", kbID, matrix.ExecutionID)

				singleResult := map[string]interface{}{
					"execution_id": matrix.ExecutionID,
					"qno":          targetQNo,
					"timestamp":    matrix.Timestamp.Format(time.RFC3339),
					"kb_id":        kbID,
					"kb_name":      script.Name,
					"status":       script.Status,
					"score":        script.FinalScore,
					"max_score":    script.MaxScore,
					"steps":        script.Steps,
					"results":      script.Results,
					"extensions": map[string]interface{}{
						"log_path": offlineRoot,
					},
				}

				data, err := json.MarshalIndent(singleResult, "", "  ")
				if err != nil {
					log.Warn("Failed to marshal single result", "error", err)
					continue
				}

				filePath := filepath.Join(resultQNoDir, filename)
				if err := os.WriteFile(filePath, data, 0644); err != nil {
					log.Warn("Failed to save single result", "error", err, "file", filePath)
				} else {
					log.Info("Result saved", "file", filePath, "kb_id", kbID)
				}
			}

			// 保存排名文件（文件名只包含 exec_id，不包含时间戳）
			rankedFilename := fmt.Sprintf("ranked_results_%s.json", matrix.ExecutionID)
			rankedData, err := json.MarshalIndent(matrix, "", "  ")
			if err != nil {
				log.Warn("Failed to marshal ranked results", "error", err)
			} else {
				rankedFilepath := filepath.Join(resultQNoDir, rankedFilename)
				if err := os.WriteFile(rankedFilepath, rankedData, 0644); err != nil {
					log.Warn("Failed to save ranked results", "error", err)
				} else {
					log.Info("Ranked results saved", "file", rankedFilepath)
				}
			}
		}
	}

	return outputResults(matrix, outputFormat, outputDir)
}

// extractKBID 从 KB 名称中提取真实的 KB ID
func extractKBID(kbName string, results map[string]interface{}) string {
	// 尝试从 kbName 中提取（格式：名称-ID）
	// 例如：3PAR 服务 LUN 导致无法添加存储 -35838 -> 35838
	parts := strings.Split(kbName, "-")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		// 检查是否是纯数字
		if kbID, err := strconv.Atoi(lastPart); err == nil && kbID > 0 {
			return lastPart
		}
	}

	// 尝试从 results 中读取 kb_id
	if kbID, ok := results["kb_id"].(string); ok && kbID != "" {
		return kbID
	}

	// 如果都失败，返回 kbName 本身
	return kbName
}

func discoverOfflineHostsAndLogDirs(qnoDir string) ([]string, []string) {
	if qnoDir == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(qnoDir)
	if err != nil {
		return nil, nil
	}

	excluded := map[string]bool{
		"_members":           true,
		"cfgmaster_ini":      true,
		"collect_record.txt": true,
		"log_list.json":      true,
	}

	isDir := func(p string) bool {
		info, err := os.Stat(p)
		return err == nil && info.IsDir()
	}

	hosts := make([]string, 0)
	logDirs := make([]string, 0)

	for _, entry := range entries {
		if entry.IsDir() == false {
			continue
		}
		name := entry.Name()
		if excluded[name] {
			continue
		}

		hostDir := filepath.Join(qnoDir, name)
		candidates := []string{
			filepath.Join(hostDir, "sf", "log"), // 你实际解压出的重点结构
			filepath.Join(hostDir, "log"),
		}

		chosen := ""
		for _, c := range candidates {
			if !isDir(c) {
				continue
			}
			// 优先选择包含 blackbox 的 log 根（更贴合脚本关注点）
			if isDir(filepath.Join(c, "blackbox")) {
				chosen = c
				break
			}
			if chosen == "" {
				chosen = c
			}
		}

		if chosen != "" {
			hosts = append(hosts, name)
			logDirs = append(logDirs, chosen)
		}
	}

	return hosts, logDirs
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	return cfg, nil
}

func createLogger(cfg *config.Config) (*logger.Logger, error) {
	return createLoggerWithModule(cfg, "")
}

func createLoggerWithModule(cfg *config.Config, module string) (*logger.Logger, error) {
	level := cfg.Logging.Level
	if verbose {
		level = "debug"
	}
	if quiet {
		level = "error"
	}

	// 根据模块选择日志路径
	logPath := cfg.Logging.Output.Path
	if module != "" {
		// 为不同模块使用不同的日志文件
		logDir := filepath.Dir(logPath)
		baseName := filepath.Base(logPath)
		logPath = filepath.Join(logDir, fmt.Sprintf("%s-%s.log", strings.TrimSuffix(baseName, ".log"), module))
	}

	return logger.NewWithConfigAndModule(logger.Config{
		Level:      level,
		Format:     cfg.Logging.Format,
		OutputPath: logPath,
		MaxSize:    cfg.Logging.Output.MaxSize,
		MaxBackups: cfg.Logging.Output.MaxBackups,
		MaxAge:     cfg.Logging.Output.MaxAge,
	}, module)
}

func registerAdapters(cfg *config.Config, engine *executor.Engine) {
	execDir, _ := os.Getwd()
	scriptsDir := cfg.Scripts.Directory

	if !filepath.IsAbs(scriptsDir) {
		scriptsDir = filepath.Join(execDir, scriptsDir)
	}

	tempDir := cfg.Execution.TempDir
	if !filepath.IsAbs(tempDir) {
		tempDir = filepath.Join(execDir, tempDir)
	}

	bashAdapter := adapter.NewBashAdapter(
		filepath.Join(scriptsDir, "bash", "api.sh"),
		tempDir,
	)
	engine.RegisterAdapter(adapter.LanguageBash, bashAdapter)

	pythonAdapter := adapter.NewPythonAdapter(
		filepath.Join(scriptsDir, "python", "kb_api.py"),
		tempDir,
		"",
	)
	engine.RegisterAdapter(adapter.LanguagePython, pythonAdapter)
}

func loadCases(cfg *config.Config, caseManager *cases.Manager) error {
	// 加载工作目录下的cases
	casesDir := filepath.Join(cfg.Execution.WorkDir, "cases")
	if _, err := os.Stat(casesDir); !os.IsNotExist(err) {
		if err := caseManager.LoadFromDirectory(casesDir); err != nil {
			return err
		}
	}

	// 加载kbscript目录下的cases
	kbscriptDir := cfg.Scripts.GetKBscriptDirectory()
	if _, err := os.Stat(kbscriptDir); !os.IsNotExist(err) {
		if err := caseManager.LoadFromDirectory(kbscriptDir); err != nil {
			return err
		}
	}

	return nil
}

func loadScenarios(cfg *config.Config, scenarioManager *scenario.Manager) error {
	scenariosDir := filepath.Join(cfg.Execution.WorkDir, "scenarios")
	if _, err := os.Stat(scenariosDir); os.IsNotExist(err) {
		return nil
	}
	return scenarioManager.LoadFromDirectory(scenariosDir)
}

func buildTasks(args []string, cfg *config.Config) []*adapter.Task {
	paramMap := parseParams(params)
	tasks := make([]*adapter.Task, 0)

	for _, path := range args {
		tasks = append(tasks, &adapter.Task{
			ID:         generateID(),
			ScriptPath: path,
			Language:   adapter.Language(language),
			Params:     paramMap,
			Timeout:    time.Duration(timeout) * time.Second,
			Weight:     weight,
			ScriptName: filepath.Base(path),
		})
	}

	return tasks
}

func buildTasksFromCase(caseManager *cases.Manager, name string, cfg *config.Config) ([]*adapter.Task, error) {
	c, err := caseManager.Get(name)
	if err != nil {
		return nil, err
	}

	return []*adapter.Task{{
		ID:         generateID(),
		ScriptPath: c.Path,
		Language:   adapter.Language(c.Language),
		Params:     c.Params,
		Timeout:    c.Timeout,
		Weight:     c.Weight,
		ScriptName: c.Name,
	}}, nil
}

func buildTasksFromScenario(scenarioManager *scenario.Manager, caseManager *cases.Manager, name string, cfg *config.Config) ([]*adapter.Task, error) {
	s, err := scenarioManager.Get(name)
	if err != nil {
		return nil, err
	}

	tasks := make([]*adapter.Task, 0, len(s.Cases))
	for _, caseName := range s.Cases {
		c, err := caseManager.Get(caseName)
		if err != nil {
			continue
		}
		tasks = append(tasks, &adapter.Task{
			ID:         generateID(),
			ScriptPath: c.Path,
			Language:   adapter.Language(c.Language),
			Params:     c.Params,
			Timeout:    c.Timeout,
			Weight:     c.Weight,
			ScriptName: c.Name,
		})
	}

	return tasks, nil
}

func buildTasksFromCategory(caseManager *cases.Manager, cat string, cfg *config.Config) ([]*adapter.Task, error) {
	caseList := caseManager.GetByCategory(cat)
	tasks := make([]*adapter.Task, 0, len(caseList))

	for _, c := range caseList {
		tasks = append(tasks, &adapter.Task{
			ID:         generateID(),
			ScriptPath: c.Path,
			Language:   adapter.Language(c.Language),
			Params:     c.Params,
			Timeout:    c.Timeout,
			Weight:     c.Weight,
			ScriptName: c.Name,
		})
	}

	return tasks, nil
}

func buildTasksFromTags(caseManager *cases.Manager, tagList []string, cfg *config.Config) ([]*adapter.Task, error) {
	caseList := caseManager.GetByTags(tagList...)
	tasks := make([]*adapter.Task, 0, len(caseList))

	for _, c := range caseList {
		tasks = append(tasks, &adapter.Task{
			ID:         generateID(),
			ScriptPath: c.Path,
			Language:   adapter.Language(c.Language),
			Params:     c.Params,
			Timeout:    c.Timeout,
			Weight:     c.Weight,
			ScriptName: c.Name,
		})
	}

	return tasks, nil
}

func interactiveSelect(caseManager *cases.Manager, cfg *config.Config) ([]*adapter.Task, error) {
	caseList := caseManager.List(cases.FilterOptions{})
	if len(caseList) == 0 {
		return nil, fmt.Errorf("no cases available")
	}

	fmt.Println("可用的CASE列表:")
	for i, c := range caseList {
		fmt.Printf("  %d. %s (%s) - %s\n", i+1, c.Name, c.Category, c.Description)
	}

	fmt.Print("\n请输入要执行的CASE编号(多个用逗号分隔): ")
	var input string
	fmt.Scanln(&input)

	indices := strings.Split(input, ",")
	tasks := make([]*adapter.Task, 0)

	for _, idx := range indices {
		idx = strings.TrimSpace(idx)
		var i int
		if _, err := fmt.Sscanf(idx, "%d", &i); err != nil {
			continue
		}
		if i < 1 || i > len(caseList) {
			continue
		}
		c := caseList[i-1]
		tasks = append(tasks, &adapter.Task{
			ID:         generateID(),
			ScriptPath: c.Path,
			Language:   adapter.Language(c.Language),
			Params:     c.Params,
			Timeout:    c.Timeout,
			Weight:     c.Weight,
			ScriptName: c.Name,
		})
	}

	return tasks, nil
}

func parseParams(p []string) map[string]string {
	m := make(map[string]string)
	for _, param := range p {
		parts := strings.SplitN(param, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}

func outputCaseListTable(caseList []*cases.Case) error {
	if len(caseList) == 0 {
		fmt.Println("没有找到匹配的CASE")
		return nil
	}

	fmt.Println("\nCASE列表:")
	fmt.Println("┌─────────────────────┬────────────┬─────────────────────────────────┬──────────────┐")
	fmt.Println("│ Name                │ Category   │ Description                     │ Tags         │")
	fmt.Println("├─────────────────────┼────────────┼─────────────────────────────────┼──────────────┤")

	for _, c := range caseList {
		name := truncate(c.Name, 19)
		cat := truncate(c.Category, 10)
		desc := truncate(c.Description, 31)
		tags := truncate(strings.Join(c.Tags, ","), 12)
		fmt.Printf("│ %-19s │ %-10s │ %-31s │ %-12s │\n", name, cat, desc, tags)
	}

	fmt.Println("└─────────────────────┴────────────┴─────────────────────────────────┴──────────────┘")
	fmt.Printf("\n共 %d 个CASE\n", len(caseList))
	return nil
}

func outputCaseListJSON(caseList []*cases.Case) error {
	for _, c := range caseList {
		data, err := c.ToJSON()
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	}
	return nil
}

func outputCaseDetail(c *cases.Case) error {
	fmt.Printf("\nCASE: %s\n", c.Name)
	fmt.Println("────────────────────────────────────────")
	fmt.Printf("Path:        %s\n", c.Path)
	fmt.Printf("Language:    %s\n", c.Language)
	fmt.Printf("Category:    %s\n", c.Category)
	fmt.Printf("Tags:        %s\n", strings.Join(c.Tags, ", "))
	fmt.Printf("Description: %s\n", c.Description)
	fmt.Printf("Timeout:     %s\n", c.Timeout)
	fmt.Printf("Weight:      %.2f\n", c.Weight)
	if len(c.Params) > 0 {
		fmt.Println("Params:")
		for k, v := range c.Params {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}
	return nil
}

func outputScenarioList(scenarios []*scenario.Scenario) error {
	if len(scenarios) == 0 {
		fmt.Println("没有找到场景")
		return nil
	}

	fmt.Println("\n场景列表:")
	fmt.Println("┌─────────────────────┬──────────────────────────────────┬───────────┐")
	fmt.Println("│ Name                │ Description                      │ Cases     │")
	fmt.Println("├─────────────────────┼──────────────────────────────────┼───────────┤")

	for _, s := range scenarios {
		name := truncate(s.Name, 19)
		desc := truncate(s.Description, 32)
		fmt.Printf("│ %-19s │ %-32s │ %-9d │\n", name, desc, s.CaseCount())
	}

	fmt.Println("└─────────────────────┴──────────────────────────────────┴───────────┘")
	return nil
}

func outputScenarioDetail(s *scenario.Scenario) error {
	fmt.Printf("\n场景: %s\n", s.Name)
	fmt.Println("────────────────────────────────────────")
	fmt.Printf("Description: %s\n", s.Description)
	fmt.Printf("Parallel:    %v\n", s.Execution.Parallel)
	fmt.Printf("Timeout:     %s\n", s.Execution.Timeout)
	fmt.Println("Cases:")
	for _, c := range s.Cases {
		fmt.Printf("  - %s\n", c)
	}
	return nil
}

func outputResults(matrix *result.ResultMatrix, format, dir string) error {
	var output string
	var err error

	switch format {
	case "yaml":
		data, e := matrix.ToYAML()
		if e != nil {
			return e
		}
		output = string(data)
	case "table":
		output = formatMatrixTable(matrix)
	default:
		data, e := matrix.ToJSON()
		if e != nil {
			return e
		}
		output = string(data)
	}

	if dir != "" {
		filename := fmt.Sprintf("result_%s.json", matrix.ExecutionID)
		path := filepath.Join(dir, filename)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte(output), 0644)
	}

	fmt.Println(output)
	return err
}

func formatMatrixTable(matrix *result.ResultMatrix) string {
	var sb strings.Builder

	sb.WriteString("\n执行结果报告\n")
	sb.WriteString("═══════════════════════════════════════════════════════════════\n\n")
	sb.WriteString(fmt.Sprintf("执行ID: %s\n", matrix.ExecutionID))
	sb.WriteString(fmt.Sprintf("时间: %s\n", matrix.Timestamp.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("状态: 成功 %d, 失败 %d, 警告 %d\n\n",
		matrix.Summary.SuccessCount, matrix.Summary.FailureCount, matrix.Summary.WarningCount))

	sb.WriteString("脚本结果:\n")
	sb.WriteString("┌─────────────────────┬─────────┬───────┬──────────┐\n")
	sb.WriteString("│ Script              │ Status  │ Score │ Weighted │\n")
	sb.WriteString("├─────────────────────┼─────────┼───────┼──────────┤\n")

	for _, s := range matrix.Scripts {
		name := truncate(s.Name, 19)
		sb.WriteString(fmt.Sprintf("│ %-19s │ %-7s │ %5.2f │ %8.2f │\n",
			name, s.Status, s.RawScore, s.WeightedScore))
	}

	sb.WriteString("└─────────────────────┴─────────┴───────┴──────────┘\n\n")
	sb.WriteString(fmt.Sprintf("平均得分: %.2f\n", matrix.Summary.AverageScore))
	sb.WriteString(fmt.Sprintf("加权平均: %.2f\n", matrix.Summary.WeightedAverage))

	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func initCase(cmd *cobra.Command, args []string) error {
	caseName := args[0]

	if initLanguage != "bash" && initLanguage != "python" {
		return fmt.Errorf("不支持的脚本语言: %s, 请使用 bash 或 python", initLanguage)
	}

	caseDir := filepath.Join(initOutput, caseName)
	if _, err := os.Stat(caseDir); err == nil {
		return fmt.Errorf("目录已存在: %s", caseDir)
	}

	if err := os.MkdirAll(caseDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	caseYAML := fmt.Sprintf(`name: %s
language: %s
category: default
tags:
  - default
description: TODO: 在这里填写CASE的描述信息
timeout: 300s
weight: 1.0
params:
  key: value
`, caseName, initLanguage)

	yamlPath := filepath.Join(caseDir, "case.yaml")
	if err := os.WriteFile(yamlPath, []byte(caseYAML), 0644); err != nil {
		return fmt.Errorf("创建配置文件失败: %w", err)
	}

	var scriptContent string
	if initLanguage == "bash" {
		scriptContent = `#!/bin/bash
# CASE: CASE_NAME
# 描述: TODO: 在这里填写CASE的描述信息

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

source "$PROJECT_ROOT/scripts/bash/api.sh"

kb_init

# ============================================
# 在这里编写你的检查逻辑
# ============================================

# 示例步骤1: 检查系统环境
step_start "check_environment"
if [ -f "/etc/os-release" ]; then
    result "os" "linux"
    step_success "系统环境检查通过"
else
    step_warning "无法确定操作系统类型"
fi

# 示例步骤2: 执行检查
step_start "execute_check"
# TODO: 在这里添加你的检查逻辑
result "status" "ok"
step_success "检查执行完成"

# ============================================
# 保存结果
# ============================================

kb_save

echo "CASE执行完成"
`
		scriptContent = strings.ReplaceAll(scriptContent, "CASE_NAME", caseName)
		scriptPath := filepath.Join(caseDir, "run.sh")
		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
			return fmt.Errorf("创建脚本文件失败: %w", err)
		}
	} else {
		scriptContent = `#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# CASE: CASE_NAME
# 描述: TODO: 在这里填写CASE的描述信息

import sys
import os

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(SCRIPT_DIR)))
sys.path.insert(0, os.path.join(PROJECT_ROOT, 'scripts', 'python'))

from kb_api import kb

# ============================================
# 在这里编写你的检查逻辑
# ============================================

# 示例步骤1: 检查系统环境
kb.step_start("check_environment")
try:
    import platform
    result = {"os": platform.system()}
    kb.result("os", result["os"])
    kb.step_success("系统环境检查通过")
except Exception as e:
    kb.step_warning(f"无法确定操作系统类型: {e}")

# 示例步骤2: 执行检查
kb.step_start("execute_check")
# TODO: 在这里添加你的检查逻辑
try:
    kb.result("status", "ok")
    kb.step_success("检查执行完成")
except Exception as e:
    kb.step_failure(f"检查执行失败: {e}")

# ============================================
# 保存结果
# ============================================

kb.save()

print("CASE执行完成")
`
		scriptContent = strings.ReplaceAll(scriptContent, "CASE_NAME", caseName)
		scriptPath := filepath.Join(caseDir, "run.py")
		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
			return fmt.Errorf("创建脚本文件失败: %w", err)
		}
	}

	fmt.Printf("CASE创建成功: %s\n", caseDir)
	fmt.Printf("  - 配置文件: %s\n", yamlPath)
	var ext string
	if initLanguage == "bash" {
		ext = "sh"
	} else {
		ext = "py"
	}
	fmt.Printf("  - 脚本文件: %s\n", filepath.Join(caseDir, "run."+ext))
	fmt.Println("\n请编辑配置文件和脚本，然后就可以运行了:")
	fmt.Printf("  kb-runner run -s %s\n", filepath.Join(caseDir, "run."+ext))

	return nil
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// 检查并解压 KB 脚本
	if cfg.Scripts.EmbeddedKBEnabled {
		kbScriptDir := cfg.Scripts.GetKBscriptDirectory()
		if kbscripts.ShouldExtract(kbScriptDir) {
			fmt.Printf("KB scripts not found, extracting to: %s\n", kbScriptDir)
			if err := kbscripts.Extract(kbScriptDir); err != nil {
				return fmt.Errorf("failed to extract KB scripts: %w", err)
			}
			fmt.Println("KB scripts extracted successfully")
		}
	}

	log, err := createLoggerWithModule(cfg, "serve")
	if err != nil {
		return err
	}

	if err := cfg.EnsureDirectories(); err != nil {
		return err
	}

	if serveToken != "" {
		cfg.Server.Token = serveToken
	}

	srv := api.NewServer(cfg, log)

	registerAdapters(cfg, srv.Engine())

	if err := loadCases(cfg, srv.CaseManager()); err != nil {
		log.Warn("Failed to load cases", "error", err)
	}

	if err := loadScenarios(cfg, srv.ScenarioManager()); err != nil {
		log.Warn("Failed to load scenarios", "error", err)
	}

	addr := fmt.Sprintf("%s:%d", serveHost, servePort)
	fmt.Printf("Starting KB Runner Web Server...\n")
	fmt.Printf("  Address: http://%s\n", addr)
	fmt.Printf("  API:     http://%s/api/v1\n", addr)
	if cfg.Server.Token != "" {
		h := sha256.Sum256([]byte(cfg.Server.Token))
		hashed := base64.URLEncoding.EncodeToString(h[:])
		fmt.Printf("  Token:   已启用认证\n")
		fmt.Printf("  Raw Token:     %s\n", cfg.Server.Token)
		fmt.Printf("  Hashed Token:  %s\n", hashed)
	}
	fmt.Println("\nPress Ctrl+C to stop")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	return srv.Start(addr)
}

func generateID() string {
	now := time.Now()
	// 格式：YYYYMMDD-HHMMSS-ffffff
	// 例如：20260327-091720-779362
	return fmt.Sprintf("%s-%06d", now.Format("20060102-150405"), now.UnixNano()%1000000)
}

func saveHistory(cfg *config.Config, matrix *result.ResultMatrix, caseName, scenarioName string) error {
	historyDir := filepath.Join(cfg.Execution.WorkDir, "history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return err
	}

	status := "success"
	if matrix.Summary.FailureCount > 0 {
		status = "failure"
	} else if matrix.Summary.WarningCount > 0 {
		status = "warning"
	}

	cases := []string{}
	if caseName != "" {
		cases = []string{caseName}
	}

	record := map[string]interface{}{
		"execution_id": matrix.ExecutionID,
		"timestamp":    matrix.Timestamp.Format(time.RFC3339),
		"status":       status,
		"trigger":      "cli",
		"cases":        cases,
		"summary": map[string]interface{}{
			"total_scripts":    matrix.Summary.TotalScripts,
			"success_count":    matrix.Summary.SuccessCount,
			"failure_count":    matrix.Summary.FailureCount,
			"warning_count":    matrix.Summary.WarningCount,
			"average_score":    matrix.Summary.AverageScore,
			"weighted_average": matrix.Summary.WeightedAverage,
		},
		"result": matrix,
	}

	if scenarioName != "" {
		record["scenario"] = scenarioName
	}

	filename := fmt.Sprintf("history_%s.json", matrix.ExecutionID)
	path := filepath.Join(historyDir, filename)

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// saveResultsToDir 保存结果到指定目录
// buildTasksForBatch 构建批量执行任务
func buildTasksForBatch(caseManager *cases.Manager, cfg *config.Config) ([]*adapter.Task, error) {
	var tasks []*adapter.Task

	// 加载所有 KB 脚本（使用空 FilterOptions 获取全部）
	allCases := caseManager.List(cases.FilterOptions{})

	// 筛选 KB
	var filteredCases []*cases.Case
	if kbList != "" {
		// 按指定的 KB 列表筛选
		kbNames := strings.Split(kbList, ",")
		kbNameMap := make(map[string]bool)
		for _, name := range kbNames {
			kbNameMap[strings.TrimSpace(name)] = true
		}

		for _, c := range allCases {
			if kbNameMap[c.Name] {
				filteredCases = append(filteredCases, c)
			}
		}
	} else if category != "" {
		// 按分类筛选
		for _, c := range allCases {
			if c.Category == category {
				filteredCases = append(filteredCases, c)
			}
		}
	} else if len(tags) > 0 {
		// 按标签筛选
		tagMap := make(map[string]bool)
		for _, tag := range tags {
			tagMap[tag] = true
		}

		for _, c := range allCases {
			for _, tag := range c.Tags {
				if tagMap[tag] {
					filteredCases = append(filteredCases, c)
					break
				}
			}
		}
	} else {
		// 默认执行所有 KB
		filteredCases = allCases
	}

	// 构建任务
	for _, c := range filteredCases {
		// 从 Path 中提取目录名作为 KB ID（包含完整名称和数字 ID）
		kbDirName := filepath.Base(filepath.Dir(c.Path))

		task := &adapter.Task{
			ID:         kbDirName, // 使用完整目录名作为 ID
			ScriptName: c.Name,
			ScriptPath: c.Path,
			Language:   adapter.Language(c.Language),
			Timeout:    c.Timeout,
			Params:     c.Params,
			Env:        make(map[string]string),
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// kbInitCase 创建 KB 目录模板
func kbInitCase(cmd *cobra.Command, args []string) error {
	kbName := args[0]

	if initLanguage != "bash" && initLanguage != "python" {
		return fmt.Errorf("不支持的脚本语言：%s, 请使用 bash 或 python", initLanguage)
	}

	kbDir := filepath.Join(initOutput, kbName)
	if _, err := os.Stat(kbDir); err == nil {
		return fmt.Errorf("目录已存在：%s", kbDir)
	}

	if err := os.MkdirAll(kbDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败：%w", err)
	}

	// 创建 case.yaml
	caseYAML := fmt.Sprintf(`name: %s
language: %s
category: default
tags:
  - default
description: TODO: 在这里填写 KB 的描述信息
timeout: 300s
weight: 1.0
params:
  kb_id: "00000"

scoring:
  max_score: 100.0
  steps:
    - name: "检查相关告警"
      weight: 0.3
      expected_status: "success"
    - name: "检查配置"
      weight: 0.3
      expected_status: "success"
    - name: "结果分析"
      weight: 0.4
      expected_status: "success"
`, kbName, initLanguage)

	yamlPath := filepath.Join(kbDir, "case.yaml")
	if err := os.WriteFile(yamlPath, []byte(caseYAML), 0644); err != nil {
		return fmt.Errorf("创建配置文件失败：%w", err)
	}

	// 创建 Skill.md
	skillMD := fmt.Sprintf(`# %s

## KB ID

TODO: 填写 KB 单号

## 问题描述

TODO: 描述问题的现象和影响范围

## 告警匹配

- **告警来源**: TODO
- **触发条件**: TODO

## 排查步骤

### 步骤 1：检查相关告警

**脚本**: step01-check-alerts.bash

TODO: 说明检查什么告警

### 步骤 2：检查配置

**脚本**: step02-check-config.bash

TODO: 说明检查什么配置

### 步骤 3：结果分析

**脚本**: step03-analyze-results.bash

TODO: 说明如何分析结果

## 根因分析

TODO: 说明问题的根本原因

## 解决方案

TODO: 提供解决方案和处理步骤

## 建议

TODO: 给出建议和注意事项
`, kbName)

	skillPath := filepath.Join(kbDir, "Skill.md")
	if err := os.WriteFile(skillPath, []byte(skillMD), 0644); err != nil {
		return fmt.Errorf("创建 Skill.md 失败：%w", err)
	}

	// 创建 run.sh
	var scriptContent string
	if initLanguage == "bash" {
		scriptContent = `#!/bin/bash
# KB: KB_NAME
# 描述：TODO

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

source "$PROJECT_ROOT/scripts/bash/api.sh"
source "$PROJECT_ROOT/scripts/bash/icare_log_api.sh"

kb_init

# 步骤 1：检查相关告警
step_start "检查相关告警"
# TODO: 添加检查逻辑
step_success "未发现相关告警"

# 步骤 2：检查配置
step_start "检查配置"
# TODO: 添加检查逻辑
step_success "配置检查完成"

# 步骤 3：结果分析
step_start "结果分析"
# TODO: 添加分析逻辑
step_success "分析完成"

kb_save

echo "KB 执行完成"
`
		scriptContent = strings.ReplaceAll(scriptContent, "KB_NAME", kbName)
		scriptPath := filepath.Join(kbDir, "run.sh")
		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
			return fmt.Errorf("创建脚本文件失败：%w", err)
		}
	}

	fmt.Printf("✓ KB 目录创建成功：%s\n", kbDir)
	fmt.Printf("  - case.yaml: 配置文件\n")
	fmt.Printf("  - Skill.md: KB 文档\n")
	fmt.Printf("  - run.sh: 执行脚本\n")
	fmt.Printf("\n请编辑这些文件，补充 KB 的具体内容\n")

	return nil
}

// kbCheck 检查 KB 脚本和配置文件
func kbCheck(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	log, err := createLogger(cfg)
	if err != nil {
		return err
	}

	// 获取 KB 目录列表
	kbDirs := cfg.GetKBDirectories()
	if len(kbDirs) == 0 {
		kbDirs = []string{"./kbscript"}
	}

	var allKBs []string
	for _, kbDir := range kbDirs {
		entries, err := os.ReadDir(kbDir)
		if err != nil {
			log.Warn("读取 KB 目录失败", "dir", kbDir, "error", err)
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			kbPath := filepath.Join(kbDir, entry.Name())
			if _, err := os.Stat(filepath.Join(kbPath, "run.sh")); err == nil {
				allKBs = append(allKBs, kbPath)
			}
		}
	}

	// 如果指定了 KB 名称，只检查该 KB
	if len(args) > 0 {
		kbName := args[0]
		var found bool
		var filteredKBs []string
		for _, kbPath := range allKBs {
			if filepath.Base(kbPath) == kbName {
				filteredKBs = append(filteredKBs, kbPath)
				found = true
			}
		}
		if !found {
			return fmt.Errorf("找不到 KB: %s", kbName)
		}
		allKBs = filteredKBs
	}

	// 检查每个 KB
	totalKBs := len(allKBs)
	passedKBs := 0
	failedKBs := 0

	for _, kbPath := range allKBs {
		kbName := filepath.Base(kbPath)
		log.Info("检查 KB", "name", kbName)

		passed := true

		// 检查文件存在性
		requiredFiles := []string{"Skill.md", "run.sh", "case.yaml"}
		for _, file := range requiredFiles {
			filePath := filepath.Join(kbPath, file)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				log.Error("缺少必需文件", "file", file)
				passed = false
			}
		}

		// 检查 Skill.md 内容
		skillPath := filepath.Join(kbPath, "Skill.md")
		if data, err := os.ReadFile(skillPath); err == nil {
			content := string(data)
			required := []string{"KB ID", "问题", "步骤", "解决", "根因"}
			for _, req := range required {
				if !strings.Contains(content, req) {
					log.Warn("Skill.md 缺少内容", "missing", req)
					passed = false
				}
			}
		}

		// 检查 run.sh
		runPath := filepath.Join(kbPath, "run.sh")
		if data, err := os.ReadFile(runPath); err == nil {
			content := string(data)
			required := []string{"kb_init", "kb_save", "step_start"}
			for _, req := range required {
				if !strings.Contains(content, req) {
					log.Warn("run.sh 缺少调用", "missing", req)
					passed = false
				}
			}

			// 检查 offline 模式是否有提前退出
			if strings.Contains(content, "KB_RUN_MODE") && strings.Contains(content, "exit 0") {
				log.Warn("Offline 模式可能有提前退出")
				passed = false
			}
		}

		// 检查 case.yaml
		yamlPath := filepath.Join(kbPath, "case.yaml")
		if data, err := os.ReadFile(yamlPath); err == nil {
			content := string(data)
			if !strings.Contains(content, "scoring:") || !strings.Contains(content, "steps:") {
				log.Warn("case.yaml 缺少 scoring 配置")
				passed = false
			}
		}

		if passed {
			log.Info("KB 检查通过", "name", kbName)
			passedKBs++
		} else {
			log.Error("KB 检查失败", "name", kbName)
			failedKBs++
		}
	}

	// 输出总结
	fmt.Printf("\n==========================================\n")
	fmt.Printf("检查总结\n")
	fmt.Printf("==========================================\n")
	fmt.Printf("KB 总数：%d\n", totalKBs)
	fmt.Printf("通过：%d\n", passedKBs)
	fmt.Printf("失败：%d\n", failedKBs)

	if failedKBs > 0 {
		return fmt.Errorf("有 %d 个 KB 检查失败", failedKBs)
	}

	return nil
}

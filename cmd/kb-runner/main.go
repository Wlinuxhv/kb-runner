package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"kb-runnerx/internal/adapter"
	"kb-runnerx/internal/cases"
	"kb-runnerx/internal/executor"
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

var initCmd = &cobra.Command{
	Use:   "init <case-name>",
	Short: "创建CASE目录模板",
	Long: `创建一个标准化的CASE目录结构，包含配置文件和脚本模板。

示例:
  kb-runner init my_check                  # 创建默认Bash CASE
  kb-runner init my_check --lang python    # 创建Python CASE
  kb-runner init my_check --output ./cases # 指定输出目录`,
	Args: cobra.ExactArgs(1),
	RunE: initCase,
}

var (
	initLanguage  string
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

	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(scenarioCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(versionCmd)

	initCmd.Flags().StringVarP(&initLanguage, "language", "l", "bash", "脚本语言 (bash/python)")
	initCmd.Flags().StringVarP(&initOutput, "output", "o", "./cases", "输出目录")
	initCmd.Flags().StringVar(&initTemplate, "template", "default", "模板类型")
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

	log, err := createLogger(cfg)
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

	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		return fmt.Errorf("no tasks to execute")
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

	return outputResults(matrix, outputFormat, outputDir)
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	return cfg, nil
}

func createLogger(cfg *config.Config) (*logger.Logger, error) {
	level := cfg.Logging.Level
	if verbose {
		level = "debug"
	}
	if quiet {
		level = "error"
	}
	return logger.New(level, cfg.Logging.Format, cfg.Logging.Output.Path)
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
	casesDir := filepath.Join(cfg.Execution.WorkDir, "cases")
	if _, err := os.Stat(casesDir); os.IsNotExist(err) {
		return nil
	}
	return caseManager.LoadFromDirectory(casesDir)
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

func generateID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}

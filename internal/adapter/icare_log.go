package adapter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// IcareLogAdapter ICare日志适配器 - 简化版
// 技术支持人员只需提供Q单号，其他自动处理
type IcareLogAdapter struct {
	RootPath string // 日志根目录

	// 查询条件（只需Q单号，其他自动解析）
	QNo       string   // Q单号（如Q2026031700424）
	YearMonth string   // 自动从Q单号解析：2603
	Host      string   // 主机名（如sp_10.250.0.7）
	Hosts     []string // 自动查找的主机列表
	Extracted string   // 解压目录（如有）
}

// LogFile 日志文件信息
type LogFile struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"`
}

// SearchResult 搜索结果
type SearchResult struct {
	FilePath string `json:"file"`
	LineNum  int    `json:"line"`
	Content  string `json:"content"`
}

// NewIcareLogAdapter 创建ICare日志适配器
func NewIcareLogAdapter() *IcareLogAdapter {
	return &IcareLogAdapter{
		RootPath: "/sf/data/icare_log/logall",
	}
}

// Init 初始化适配器（只需Q单号）
func (a *IcareLogAdapter) Init(qno string) error {
	if qno == "" {
		return fmt.Errorf("Q单号不能为空")
	}

	// 验证Q单号格式
	if !regexp.MustCompile(`^Q\d{12}$`).MatchString(qno) {
		return fmt.Errorf("Q单号格式错误，应为Q+12位数字，如Q2026031700424")
	}

	a.QNo = qno

	// 从Q单号解析年月：Q2026031700424 -> 2603
	// Q单号格式：Q + 年(4位) + 月(2位) + 序号(6位)
	year := qno[1:5]   // 2026
	month := qno[5:7]  // 03
	a.YearMonth = year[2:] + month // 2603

	// 自动查找主机
	a.findHosts()

	// 自动解压（如需要）
	a.extractIfNeeded()

	return nil
}

// parseYearMonth 解析格式化的年月份
func (a *IcareLogAdapter) parseYearMonth() string {
	if a.QNo == "" {
		return ""
	}
	year := a.QNo[1:5]
	month := a.QNo[5:7]
	return fmt.Sprintf("%s-%s", year, month)
}

// findHosts 自动查找主机列表
func (a *IcareLogAdapter) findHosts() {
	qnoDir := filepath.Join(a.RootPath, a.YearMonth, a.QNo)

	// 如果目录不存在，尝试在其他年月份中查找
	if _, err := os.Stat(qnoDir); os.IsNotExist(err) {
		// 搜索所有可能的年月份目录
		entries, _ := os.ReadDir(a.RootPath)
		for _, entry := range entries {
			testDir := filepath.Join(a.RootPath, entry.Name(), a.QNo)
			if info, err := os.Stat(testDir); err == nil && info.IsDir() {
				qnoDir = testDir
				a.YearMonth = entry.Name()
				break
			}
		}
	}

	a.Hosts = nil

	entries, err := os.ReadDir(qnoDir)
	if err != nil {
		return
	}

	// 排除特殊文件
	excluded := map[string]bool{
		"_members":           true,
		"cfgmaster_ini":      true,
		"collect_record.txt": true,
		"log_list.json":      true,
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if excluded[name] {
			continue
		}
		// 排除ZIP文件
		if strings.HasSuffix(name, ".zip") {
			continue
		}
		a.Hosts = append(a.Hosts, name)
	}

	sort.Strings(a.Hosts)

	// 设置第一个主机为当前主机
	if len(a.Hosts) > 0 {
		a.Host = a.Hosts[0]
	}
}

// extractIfNeeded 自动解压ZIP文件（如存在）
func (a *IcareLogAdapter) extractIfNeeded() {
	qnoDir := filepath.Join(a.RootPath, a.YearMonth, a.QNo)

	entries, err := os.ReadDir(qnoDir)
	if err != nil {
		return
	}

	// 查找ZIP文件
	var zipFile string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".zip") {
			zipFile = filepath.Join(qnoDir, entry.Name())
			break
		}
	}

	if zipFile == "" {
		return
	}

	// 检查是否已解压
	extractDir := strings.TrimSuffix(zipFile, ".zip")
	if _, err := os.Stat(extractDir); err == nil {
		a.Extracted = extractDir
		return
	}

	// 执行解压（需要外部调用unzip命令）
	// 这里只记录状态，实际解压由调用者处理
}

// SetHost 设置当前主机
func (a *IcareLogAdapter) SetHost(host string) error {
	for _, h := range a.Hosts {
		if h == host {
			a.Host = host
			return nil
		}
	}
	return fmt.Errorf("主机不存在: %s", host)
}

// GetQNo 获取Q单号
func (a *IcareLogAdapter) GetQNo() string {
	return a.QNo
}

// GetYearMonth 获取年月份
func (a *IcareLogAdapter) GetYearMonth() string {
	return a.YearMonth
}

// GetYearMonthFormatted 获取格式化的年月份
func (a *IcareLogAdapter) GetYearMonthFormatted() string {
	return a.parseYearMonth()
}

// ListHosts 获取主机列表
func (a *IcareLogAdapter) ListHosts() []string {
	return a.Hosts
}

// GetLogPath 获取日志目录路径
func (a *IcareLogAdapter) GetLogPath() string {
	if a.QNo == "" || a.Host == "" {
		return ""
	}
	return filepath.Join(a.RootPath, a.YearMonth, a.QNo, a.Host)
}

// Exists 检查目录是否存在
func (a *IcareLogAdapter) Exists() bool {
	path := a.GetLogPath()
	return path != "" && func() bool {
		info, err := os.Stat(path)
		return err == nil && info.IsDir()
	}()
}

// ListFiles 获取日志文件列表
func (a *IcareLogAdapter) ListFiles() ([]LogFile, error) {
	path := a.GetLogPath()
	if path == "" {
		return nil, fmt.Errorf("日志路径未初始化")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("目录不存在: %s", path)
	}

	var files []LogFile
	err := filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if walkPath == path {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(path, walkPath)
		files = append(files, LogFile{
			Path:  relPath,
			Name:  info.Name(),
			Size:  info.Size(),
			Mtime: info.ModTime().Unix(),
		})
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// Read 读取日志文件内容
func (a *IcareLogAdapter) Read(filePath string) (string, error) {
	fullPath := filepath.Join(a.GetLogPath(), filePath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("文件不存在: %s", filePath)
		}
		return "", fmt.Errorf("读取文件失败: %w", err)
	}
	return string(data), nil
}

// Search 搜索日志关键字
func (a *IcareLogAdapter) Search(keyword string) ([]SearchResult, error) {
	path := a.GetLogPath()
	if path == "" || keyword == "" {
		return nil, nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	var results []SearchResult

	// 需要搜索的文件扩展名
	logExtensions := map[string]bool{
		".log":  true,
		".txt":  true,
		".info": true,
	}

	err := filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		ext := filepath.Ext(info.Name())
		if !logExtensions[ext] && ext != "" {
			return nil
		}

		data, err := os.ReadFile(walkPath)
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(path, walkPath)
		lines := strings.Split(string(data), "\n")

		for lineNum, line := range lines {
			if strings.Contains(line, keyword) {
				results = append(results, SearchResult{
					FilePath: relPath,
					LineNum:  lineNum + 1,
					Content:  strings.TrimSpace(line),
				})
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return results, nil
}

// CountKeyword 统计关键字出现次数
func (a *IcareLogAdapter) CountKeyword(keyword string) int {
	results, _ := a.Search(keyword)
	return len(results)
}

// Status 获取适配器状态
func (a *IcareLogAdapter) Status() map[string]interface{} {
	return map[string]interface{}{
		"qno":                  a.QNo,
		"yearmonth":            a.YearMonth,
		"yearmonth_formatted":  a.parseYearMonth(),
		"log_path":             a.GetLogPath(),
		"host_count":           len(a.Hosts),
		"hosts":                a.Hosts,
		"current_host":         a.Host,
		"extracted":            a.Extracted,
	}
}

package preprocessor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type LogPreprocessor struct {
	RootPath        string
	ConfigPath      string
	ArchivePassword string
	QNo             string
	YearMonth       string
	ExtractDir      string
}

type Config struct {
	Archive struct {
		PasswordHash string `json:"password_hash"`
		Salt         string `json:"salt"`
		Algorithm    string `json:"algorithm"`
	} `json:"archive"`
}

func NewLogPreprocessor() *LogPreprocessor {
	return &LogPreprocessor{
		RootPath:        "./workspace/icare_log/logall",
		ConfigPath:      "./config/icare_log.json",
		ArchivePassword: "sangfor.vt@aDeploy2019",
	}
}

func (p *LogPreprocessor) Init(qno string) error {
	if qno == "" {
		return fmt.Errorf("Q单号不能为空")
	}

	if !regexp.MustCompile(`^Q\d{12,13}$`).MatchString(qno) {
		return fmt.Errorf("Q单号格式错误，应为Q+12-13位数字")
	}

	p.QNo = qno
	year := qno[1:5]
	month := qno[5:7]
	p.YearMonth = year[2:] + month

	return nil
}

func (p *LogPreprocessor) Process() error {
	// 兼容两种布局：
	// 1) Root/YYMM/Qxxxx/   目录下存在 zip
	// 2) Root/YYMM/Qxxxx.zip  直接放在 YYMM 下（当前用户场景）
	qnoDir := filepath.Join(p.RootPath, p.YearMonth, p.QNo)
	zipAtYM := filepath.Join(p.RootPath, p.YearMonth, p.QNo+".zip")

	if _, err := os.Stat(qnoDir); os.IsNotExist(err) {
		if _, zipErr := os.Stat(zipAtYM); zipErr == nil {
			// 创建 Q 单目录并解压 zip 到该目录（等价于解压后的根目录是 Root/YYMM/Qxxxx）
			if err := os.MkdirAll(qnoDir, 0755); err != nil {
				return fmt.Errorf("创建Q单目录失败: %w", err)
			}
			fmt.Printf("检测到日志包: %s\n", zipAtYM)
			fmt.Printf("正在解压到: %s\n", qnoDir)
			if err := p.extractZip(zipAtYM, qnoDir); err != nil {
				return fmt.Errorf("解压失败: %w", err)
			}
			p.ExtractDir = qnoDir
			return nil
		}
		return fmt.Errorf("日志目录不存在: %s", qnoDir)
	}

	entries, err := os.ReadDir(qnoDir)
	if err != nil {
		return fmt.Errorf("读取目录失败: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".zip") {
			continue
		}

		zipFile := filepath.Join(qnoDir, entry.Name())
		extractDir := strings.TrimSuffix(zipFile, ".zip")

		if _, err := os.Stat(extractDir); err == nil {
			fmt.Printf("已解压: %s\n", entry.Name())
			p.ExtractDir = extractDir
			continue
		}

		fmt.Printf("正在解压: %s\n", entry.Name())
		if err := p.extractZip(zipFile, extractDir); err != nil {
			return fmt.Errorf("解压失败: %w", err)
		}

		p.ExtractDir = extractDir
		fmt.Printf("解压完成: %s\n", extractDir)
	}

	return nil
}

func (p *LogPreprocessor) extractZip(zipFile, extractDir string) error {
	// 创建解压目录
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败：%w", err)
	}

	// 解压 ZIP 文件到指定目录（兼容：无密码 / 有密码）
	// unzip 的行为：
	// - 对无加密 zip，-P 参数通常不会影响解压
	// - 对加密 zip，密码不对会失败
	// 所以这里采用：先尝试带密码，再尝试不带密码
	tryUnzip := func(args ...string) ([]byte, error) {
		cmd := exec.Command("unzip", args...)
		return cmd.CombinedOutput()
	}

	output, err := tryUnzip("-P", p.ArchivePassword, "-o", zipFile, "-d", extractDir)
	if err != nil {
		// 第二次尝试：无密码
		output2, err2 := tryUnzip("-o", zipFile, "-d", extractDir)
		if err2 != nil {
			return fmt.Errorf("unzip 命令执行失败：%w, output: %s", err, string(output))
		}
		output = output2
	}
	_ = output

	// 检查是否有 .tgz 文件需要二次解压
	entries, _ := os.ReadDir(extractDir)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tgz") {
			tgzFile := filepath.Join(extractDir, entry.Name())
			fmt.Printf("发现日志压缩包：%s\n", entry.Name())

			// 创建 tgz 解压目录（以主机 IP 命名）
			hostName := strings.TrimSuffix(entry.Name(), ".tgz")
			hostDir := filepath.Join(extractDir, hostName)
			if err := os.MkdirAll(hostDir, 0755); err != nil {
				return fmt.Errorf("创建主机目录失败：%w", err)
			}

			fmt.Printf("解压日志到：%s\n", hostDir)
			if err := p.extractTgz(tgzFile, hostDir); err != nil {
				return fmt.Errorf("二次解压失败：%w", err)
			}

			// 删除 tgz 文件以节省空间
			os.Remove(tgzFile)
			fmt.Printf("日志解压完成：%s\n", hostDir)
		}
	}

	// 递归解压 extractDir 下的嵌套压缩包（zip/tgz/tar.gz/tar.zst）
	if err := p.extractNestedArchives(extractDir); err != nil {
		return err
	}

	return nil
}

func (p *LogPreprocessor) extractTgz(tgzFile, extractDir string) error {
	cmd := exec.Command("tar", "-xzf", tgzFile, "-C", extractDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar 命令执行失败：%w, output: %s", err, string(output))
	}

	// 删除 tgz 文件
	os.Remove(tgzFile)

	// 递归解压内部的所有压缩包
	if err := p.extractNestedArchives(extractDir); err != nil {
		return err
	}

	return nil
}

// 递归解压目录下所有嵌套的压缩包
func (p *LogPreprocessor) extractNestedArchives(dir string) error {
	// 多次遍历，直到没有压缩包为止（同时处理子目录）
	for {
		changed := false

		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			fullPath := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				continue
			}

			name := entry.Name()

			// zip
			if strings.HasSuffix(name, ".zip") {
				fmt.Printf("发现嵌套压缩包：%s\n", name)
				// 关键：必须避免 unzip 交互等待（密码/继续写入确认等）
				// 因此始终传入 -P，并提供 stdin 默认回答。
				cmd := exec.Command("unzip", "-q", "-P", p.ArchivePassword, "-o", fullPath, "-d", dir)
				// 如果 unzip 发生 Continue? / password 等交互，这里用默认回答避免卡住
				cmd.Stdin = strings.NewReader("n\n")
				if output, err := cmd.CombinedOutput(); err != nil {
					return fmt.Errorf("解压嵌套zip失败：%w, output: %s", err, string(output))
				}

				_ = os.Remove(fullPath)
				fmt.Printf("已解压并清理：%s\n", name)
				changed = true
				continue
			}

			// tar.gz / tgz
			if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") {
				fmt.Printf("发现嵌套压缩包：%s\n", name)
				cmd := exec.Command("tar", "-xzf", fullPath, "-C", dir)
				output, err := cmd.CombinedOutput()
				if err != nil {
					return fmt.Errorf("解压嵌套tar.gz失败：%w, output: %s", err, string(output))
				}
				_ = output
				_ = os.Remove(fullPath)
				fmt.Printf("已解压并清理：%s\n", name)
				changed = true
				continue
			}

			// tar.zst（需要系统支持 tar -I zstd）
			if strings.HasSuffix(name, ".tar.zst") {
				fmt.Printf("发现嵌套压缩包：%s\n", name)
				cmd := exec.Command("tar", "-I", "zstd", "-xf", fullPath, "-C", dir)
				output, err := cmd.CombinedOutput()
				if err != nil {
					// 如果系统没有 zstd 或 tar 不支持 -I，忽略（不作为致命错误）
					if errors.Is(err, exec.ErrNotFound) {
						continue
					}
					// tar 返回码非 0
					_ = output
					continue
				}
				_ = output
				_ = os.Remove(fullPath)
				fmt.Printf("已解压并清理：%s\n", name)
				changed = true
				continue
			}
		}

		// 递归处理子目录
		subEntries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range subEntries {
			if entry.IsDir() {
				if err := p.extractNestedArchives(filepath.Join(dir, entry.Name())); err != nil {
					return err
				}
			}
		}

		if !changed {
			break
		}
	}

	return nil
}

func (p *LogPreprocessor) GetExtractPath() string {
	return p.ExtractDir
}

func (p *LogPreprocessor) GetQNoPath() string {
	return filepath.Join(p.RootPath, p.YearMonth, p.QNo)
}

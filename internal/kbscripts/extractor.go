package kbscripts

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Extract 解压嵌入式 KB 脚本到目标目录
func Extract(targetDir string) error {
	// 确保目标目录存在
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// 遍历嵌入的文件系统
	return fs.WalkDir(KBScripts, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// 跳过根目录
		if path == "." {
			return nil
		}

		// 跳过 Go 文件
		if strings.HasSuffix(path, ".go") {
			return nil
		}

		targetPath := filepath.Join(targetDir, path)

		if d.IsDir() {
			// 创建目录
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
			return nil
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		// 读取嵌入的文件
		srcFile, err := KBScripts.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		// 写入目标文件
		dstFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		// 复制内容
		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			return err
		}

		// 如果是脚本文件，添加执行权限
		if strings.HasSuffix(path, ".sh") || strings.HasSuffix(path, ".py") {
			if err := os.Chmod(targetPath, 0755); err != nil {
				return err
			}
		}

		return nil
	})
}

// ShouldExtract 检查是否需要解压
func ShouldExtract(targetDir string) bool {
	// 目录不存在，需要解压
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return true
	}

	// 目录为空，需要解压
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return true
	}

	// 检查是否有 case.yaml 文件（判断是否是有效的 KB 目录）
	for _, entry := range entries {
		if entry.IsDir() {
			caseYamlPath := filepath.Join(targetDir, entry.Name(), "case.yaml")
			if _, err := os.Stat(caseYamlPath); err == nil {
				return false // 已有 KB 脚本，不需要解压
			}
		}
	}

	return true // 目录存在但没有 KB 脚本，需要解压
}

// CleanAndExtract 清理后解压（强制重新解压）
func CleanAndExtract(targetDir string) error {
	// 删除目标目录
	if err := os.RemoveAll(targetDir); err != nil {
		return err
	}
	// 重新解压
	return Extract(targetDir)
}

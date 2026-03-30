package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

type Config struct {
	Level      string
	Format     string
	OutputPath string
	MaxSize    string // 格式如 "10MB"
	MaxBackups int
	MaxAge     int
}

type Logger struct {
	zap    *zap.SugaredLogger
	level  zapcore.Level
	mu     sync.Mutex
	writer io.WriteCloser // 可以是文件或其他写入器
	config Config
	pid    int
	module string
}

var (
	defaultLogger *Logger
	once          sync.Once
)

func New(level, format, outputPath string) (*Logger, error) {
	cfg := Config{
		Level:      level,
		Format:     format,
		OutputPath: outputPath,
	}

	return NewWithConfig(cfg)
}

func NewWithConfig(cfg Config) (*Logger, error) {
	return NewWithConfigAndModule(cfg, "")
}

// NewWithConfigAndModule 创建带模块标识的日志
func NewWithConfigAndModule(cfg Config, module string) (*Logger, error) {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(cfg.Level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	var ws zapcore.WriteSyncer
	var writer io.WriteCloser

	// 展开 PID 占位符
	pid := os.Getpid()
	outputPath := ExpandPID(cfg.OutputPath, pid)

	if cfg.OutputPath != "" && cfg.OutputPath != "stdout" {
		dir := filepath.Dir(outputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		// 创建基本的日志轮转写入器
		maxSize := parseMaxSize(cfg.MaxSize)
		basicWriter, err := NewBasicRotatingWriter(outputPath, maxSize, cfg.MaxBackups)
		if err != nil {
			return nil, fmt.Errorf("failed to create rotating writer: %w", err)
		}

		ws = zapcore.AddSync(basicWriter)
		writer = basicWriter
	} else {
		ws = zapcore.AddSync(os.Stdout)
		writer = nopWriteCloser{os.Stdout}
	}

	core := zapcore.NewCore(encoder, ws, zapLevel)

	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	// 添加模块标识
	if module != "" {
		zapLogger = zapLogger.With(zap.String("module", module), zap.Int("pid", pid))
	} else {
		zapLogger = zapLogger.With(zap.Int("pid", pid))
	}

	logger := &Logger{
		zap:    zapLogger.Sugar(),
		level:  zapLevel,
		writer: writer,
		config: cfg,
		pid:    pid,
		module: module,
	}

	return logger, nil
}

// BasicRotatingWriter 基本的日志轮转实现
type BasicRotatingWriter struct {
	path        string
	file        *os.File
	maxSize     int64
	maxBackups  int
	currentSize int64
	mu          sync.Mutex
}

func NewBasicRotatingWriter(path string, maxSize int64, maxBackups int) (*BasicRotatingWriter, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	// 获取初始文件大小
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	return &BasicRotatingWriter{
		path:        path,
		file:        file,
		maxSize:     maxSize,
		maxBackups:  maxBackups,
		currentSize: stat.Size(),
	}, nil
}

func (bw *BasicRotatingWriter) Write(p []byte) (n int, err error) {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	// 检查是否需要轮转
	if bw.currentSize+int64(len(p)) > bw.maxSize {
		// 关闭当前文件
		bw.file.Close()

		// 重命名当前文件
		oldPath := fmt.Sprintf("%s.%d", bw.path, time.Now().Unix())
		os.Rename(bw.path, oldPath)

		// 创建新文件
		newFile, err := os.OpenFile(bw.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// 如果创建失败，尝试重新打开旧文件
			bw.file, _ = os.OpenFile(bw.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			return 0, err
		}

		bw.file = newFile
		bw.currentSize = 0

		// 清理旧备份文件（保留最多 maxBackups 个）
		bw.cleanupOldFiles()
	}

	n, err = bw.file.Write(p)
	bw.currentSize += int64(n)
	return n, err
}

func (bw *BasicRotatingWriter) Sync() error {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	return bw.file.Sync()
}

func (bw *BasicRotatingWriter) Close() error {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	return bw.file.Close()
}

func (bw *BasicRotatingWriter) cleanupOldFiles() {
	// 简单清理：获取所有备份文件，保留最新的 maxBackups 个
	dir := filepath.Dir(bw.path)
	baseName := filepath.Base(bw.path)

	// 这找所有相关的备份文件
	// 这找所有以 baseName. 开头的文件
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var backupFiles []string
	for _, file := range files {
		name := file.Name()
		if strings.HasPrefix(name, baseName+".") && !file.IsDir() {
			backupFiles = append(backupFiles, filepath.Join(dir, name))
		}
	}

	// 如果备份文件超过了 maxBackups，删除最旧的
	if len(backupFiles) > bw.maxBackups {
		// 这单清理：只保留最新的几个
		for i := 0; i < len(backupFiles)-bw.maxBackups; i++ {
			os.Remove(backupFiles[i])
		}
	}
}

// nopWriteCloser 是一个空的 WriteCloser 实现，用于 stdout
type nopWriteCloser struct {
	io.Writer
}

func (nwc nopWriteCloser) Close() error { return nil }

// parseMaxSize 解析最大文件大小
func parseMaxSize(sizeStr string) int64 {
	if sizeStr == "" {
		return 10 * 1024 * 1024 // 默认 10MB
	}

	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))
	if strings.HasSuffix(sizeStr, "MB") {
		numStr := strings.TrimSuffix(sizeStr, "MB")
		if num, err := strconv.ParseInt(numStr, 10, 64); err == nil {
			return num * 1024 * 1024
		}
	} else if strings.HasSuffix(sizeStr, "GB") {
		numStr := strings.TrimSuffix(sizeStr, "GB")
		if num, err := strconv.ParseInt(numStr, 10, 64); err == nil {
			return num * 1024 * 1024 * 1024
		}
	}

	// 默认 10MB
	return 10 * 1024 * 1024
}

// ExpandPID 展开 {pid} 占位符
func ExpandPID(path string, pid int) string {
	return strings.ReplaceAll(path, "{pid}", strconv.Itoa(pid))
}

// Info 记录信息日志
func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	l.zap.Infow(msg, keysAndValues...)
}

// Debug 记录调试日志
func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	l.zap.Debugw(msg, keysAndValues...)
}

// Warn 记录警告日志
func (l *Logger) Warn(msg string, keysAndValues ...interface{}) {
	l.zap.Warnw(msg, keysAndValues...)
}

// Error 记录错误日志
func (l *Logger) Error(msg string, keysAndValues ...interface{}) {
	l.zap.Errorw(msg, keysAndValues...)
}

// Fatal 记录致命错误日志并退出
func (l *Logger) Fatal(msg string, keysAndValues ...interface{}) {
	l.zap.Fatalw(msg, keysAndValues...)
}

// Panic 记录恐慌日志并 panic
func (l *Logger) Panic(msg string, keysAndValues ...interface{}) {
	l.zap.Panicw(msg, keysAndValues...)
}

// WithFields 添加字段到日志
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	args := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	newLogger := *l
	newLogger.zap = l.zap.With(args...)
	return &newLogger
}

// WithField 添加单个字段到日志
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := *l
	newLogger.zap = l.zap.With(key, value)
	return &newLogger
}

// Close 关闭日志
func (l *Logger) Close() error {
	if l.zap != nil {
		l.zap.Sync()
	}
	if l.writer != nil {
		l.writer.Close()
	}
	return nil
}

// GetZapLogger 获取底层 zap 日志记录器
func (l *Logger) GetZapLogger() *zap.Logger {
	return l.zap.Desugar()
}

// GetLevel 获取日志级别
func (l *Logger) GetLevel() Level {
	return Level(l.level.String())
}

// SetLevel 设置日志级别
func (l *Logger) SetLevel(level string) error {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		return fmt.Errorf("invalid log level: %s", level)
	}

	l.level = zapLevel
	// 重建 logger
	return nil
}

// GetModule 获取模块标识
func (l *Logger) GetModule() string {
	return l.module
}

// GetPID 获取进程 ID
func (l *Logger) GetPID() int {
	return l.pid
}

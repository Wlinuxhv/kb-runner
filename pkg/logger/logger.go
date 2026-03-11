package logger

import (
	"fmt"
	"os"
	"path/filepath"
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
	MaxSize    int
	MaxBackups int
	MaxAge     int
}

type Logger struct {
	zap    *zap.SugaredLogger
	level  zapcore.Level
	mu     sync.Mutex
	file   *os.File
	config Config
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
	if cfg.OutputPath != "" && cfg.OutputPath != "stdout" {
		dir := filepath.Dir(cfg.OutputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		file, err := os.OpenFile(cfg.OutputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		ws = zapcore.AddSync(file)
	} else {
		ws = zapcore.AddSync(os.Stdout)
	}

	core := zapcore.NewCore(encoder, ws, zapLevel)
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &Logger{
		zap:    zapLogger.Sugar(),
		level:  zapLevel,
		config: cfg,
	}, nil
}

func Default() *Logger {
	once.Do(func() {
		var err error
		defaultLogger, err = New("info", "console", "")
		if err != nil {
			panic(fmt.Sprintf("failed to create default logger: %v", err))
		}
	})
	return defaultLogger
}

func (l *Logger) Debug(args ...interface{}) {
	l.zap.Debug(args...)
}

func (l *Logger) Debugf(template string, args ...interface{}) {
	l.zap.Debugf(template, args...)
}

func (l *Logger) Info(args ...interface{}) {
	l.zap.Info(args...)
}

func (l *Logger) Infof(template string, args ...interface{}) {
	l.zap.Infof(template, args...)
}

func (l *Logger) Warn(args ...interface{}) {
	l.zap.Warn(args...)
}

func (l *Logger) Warnf(template string, args ...interface{}) {
	l.zap.Warnf(template, args...)
}

func (l *Logger) Error(args ...interface{}) {
	l.zap.Error(args...)
}

func (l *Logger) Errorf(template string, args ...interface{}) {
	l.zap.Errorf(template, args...)
}

func (l *Logger) Fatal(args ...interface{}) {
	l.zap.Fatal(args...)
}

func (l *Logger) Fatalf(template string, args ...interface{}) {
	l.zap.Fatalf(template, args...)
}

func (l *Logger) With(fields ...interface{}) *Logger {
	return &Logger{
		zap:   l.zap.With(fields...),
		level: l.level,
	}
}

func (l *Logger) WithScript(scriptName string) *Logger {
	return l.With("script", scriptName)
}

func (l *Logger) WithStep(scriptName, stepName string) *Logger {
	return l.With("script", scriptName, "step", stepName)
}

func (l *Logger) LogScript(scriptName, level, message string) {
	switch level {
	case "DEBUG", "debug":
		l.WithScript(scriptName).Debug(message)
	case "INFO", "info":
		l.WithScript(scriptName).Info(message)
	case "WARN", "warn":
		l.WithScript(scriptName).Warn(message)
	case "ERROR", "error":
		l.WithScript(scriptName).Error(message)
	default:
		l.WithScript(scriptName).Info(message)
	}
}

func (l *Logger) LogStep(scriptName, stepName, level, message string) {
	switch level {
	case "DEBUG", "debug":
		l.WithStep(scriptName, stepName).Debug(message)
	case "INFO", "info":
		l.WithStep(scriptName, stepName).Info(message)
	case "WARN", "warn":
		l.WithStep(scriptName, stepName).Warn(message)
	case "ERROR", "error":
		l.WithStep(scriptName, stepName).Error(message)
	default:
		l.WithStep(scriptName, stepName).Info(message)
	}
}

func (l *Logger) Sync() error {
	return l.zap.Sync()
}

func (l *Logger) Close() error {
	if err := l.Sync(); err != nil {
		return err
	}
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func FormatLogLine(scriptName, stepName, level, message string) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	if stepName != "" {
		return fmt.Sprintf("[%s] [%s] [%s:%s] %s", timestamp, level, scriptName, stepName, message)
	}
	return fmt.Sprintf("[%s] [%s] [%s] %s", timestamp, level, scriptName, message)
}

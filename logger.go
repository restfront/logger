package logger

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	path        string
	level       string
	structured  bool
	baseLogger  *zap.Logger
	sugarLogger *zap.SugaredLogger
	rotator     *fileRotator
}

type Option func(*Logger)

func Path(path string) Option {
	return func(l *Logger) {
		l.path = path
	}
}

func Level(level string) Option {
	return func(l *Logger) {
		if _, exist := loggerLevelMap[level]; !exist {
			level = "info"
		}
		l.level = level
	}
}

func Structured(enable bool) Option {
	return func(l *Logger) {
		l.structured = enable
	}
}

func BaseLogger(baseLogger *zap.Logger) Option {
	return func(l *Logger) {
		l.baseLogger = baseLogger
		l.sugarLogger = l.baseLogger.Sugar()
	}
}

func NewLogger(options ...Option) *Logger {
	l := &Logger{
		path:       "",
		level:      "info",
		structured: false,
	}

	for _, option := range options {
		option(l)
	}

	return l
}

var loggerLevelMap = map[string]zapcore.Level{
	"debug":  zapcore.DebugLevel,
	"info":   zapcore.InfoLevel,
	"warn":   zapcore.WarnLevel,
	"error":  zapcore.ErrorLevel,
	"dpanic": zapcore.DPanicLevel,
	"panic":  zapcore.PanicLevel,
	"fatal":  zapcore.FatalLevel,
}

func (l *Logger) getLoggerLevel() zapcore.Level {
	level, exist := loggerLevelMap[l.level]
	if !exist {
		return zapcore.DebugLevel
	}

	return level
}

func (l *Logger) InitLogger(consoleOutputEnable bool) {
	encoderCfg := zap.NewProductionEncoderConfig()

	encoderCfg.EncodeTime = func(t time.Time, pae zapcore.PrimitiveArrayEncoder) {
		pae.AppendString(t.Format("2006-01-02 15:04:05"))
	}
	encoderCfg.LevelKey = "level"
	encoderCfg.CallerKey = "caller"
	encoderCfg.TimeKey = "time"
	encoderCfg.NameKey = "name"
	encoderCfg.MessageKey = "message"
	encoderCfg.StacktraceKey = "stacktrace"

	var encoder zapcore.Encoder

	cores := make([]zapcore.Core, 0)

	if consoleOutputEnable {
		lvl := zap.NewAtomicLevel()
		lvl.SetLevel(l.getLoggerLevel())
		writer := zapcore.Lock(os.Stdout)
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
		core := zapcore.NewCore(encoder, writer, lvl)
		cores = append(cores, core)
	}

	lvl := zap.NewAtomicLevel()
	lvl.SetLevel(l.getLoggerLevel())

	fileRotator := &fileRotator{
		path:     l.path,
		compress: true,
	}

	writer := zapcore.AddSync(fileRotator)

	l.rotator = fileRotator

	if l.structured {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	}

	core := zapcore.NewCore(encoder, writer, lvl)
	cores = append(cores, core)

	combinedCore := zapcore.NewTee(cores...)

	l.baseLogger = zap.New(combinedCore,
		//	zap.AddStacktrace(zap.ErrorLevel),
		zap.AddCaller(), zap.AddCallerSkip(1),
	)

	l.sugarLogger = l.baseLogger.Sugar()
}

func (l *Logger) Close() error {
	err := l.sugarLogger.Sync()
	if err != nil {
		return err
	}

	if l.rotator != nil {
		err = l.rotator.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func (l *Logger) Debug(args ...interface{}) {
	l.sugarLogger.Debug(args...)
}

func (l *Logger) Debugf(template string, args ...interface{}) {
	l.sugarLogger.Debugf(template, args...)
}

func (l *Logger) Info(args ...interface{}) {
	l.sugarLogger.Info(args...)
}

func (l *Logger) Infof(template string, args ...interface{}) {
	l.sugarLogger.Infof(template, args...)
}

func (l *Logger) Warn(args ...interface{}) {
	l.sugarLogger.Warn(args...)
}

func (l *Logger) Warnf(template string, args ...interface{}) {
	l.sugarLogger.Warnf(template, args...)
}

func (l *Logger) Error(args ...interface{}) {
	l.sugarLogger.Error(args...)
}

func (l *Logger) Errorf(template string, args ...interface{}) {
	l.sugarLogger.Errorf(template, args...)
}

func (l *Logger) DPanic(args ...interface{}) {
	l.sugarLogger.DPanic(args...)
}

func (l *Logger) DPanicf(template string, args ...interface{}) {
	l.sugarLogger.DPanicf(template, args...)
}

func (l *Logger) Panic(args ...interface{}) {
	l.sugarLogger.Panic(args...)
}

func (l *Logger) Panicf(template string, args ...interface{}) {
	l.sugarLogger.Panicf(template, args...)
}

func (l *Logger) Fatal(args ...interface{}) {
	l.sugarLogger.Fatal(args...)
}

func (l *Logger) Fatalf(template string, args ...interface{}) {
	l.sugarLogger.Fatalf(template, args...)
}

func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}

	newBaseLogger := l.baseLogger.With(zapFields...)

	return &Logger{
		path:        l.path,
		level:       l.level,
		structured:  l.structured,
		baseLogger:  newBaseLogger,
		sugarLogger: newBaseLogger.Sugar(),
		rotator:     l.rotator,
	}
}

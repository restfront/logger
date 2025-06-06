package logger

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TestLoggerInitialization проверяет инициализацию логгера с различными опциями.
func TestLoggerInitialization(t *testing.T) {
	tests := []struct {
		name     string
		options  []Option
		expected Logger
	}{
		{
			name:    "Default initialization",
			options: []Option{},
			expected: Logger{
				path:       "",
				level:      "info",
				structured: false,
			},
		},
		{
			name:    "With path",
			options: []Option{Path("/tmp/logs")},
			expected: Logger{
				path:       "/tmp/logs",
				level:      "info",
				structured: false,
			},
		},
		{
			name:    "With level",
			options: []Option{Level("debug")},
			expected: Logger{
				path:       "",
				level:      "debug",
				structured: false,
			},
		},
		{
			name:    "With structured logging",
			options: []Option{Structured(true)},
			expected: Logger{
				path:       "",
				level:      "info",
				structured: true,
			},
		},
		{
			name:    "With base logger",
			options: []Option{BaseLogger(zap.NewNop())},
			expected: Logger{
				path:       "",
				level:      "info",
				structured: false,
				baseLogger: zap.NewNop(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(tt.options...)
			assert.Equal(t, tt.expected.path, logger.path)
			assert.Equal(t, tt.expected.level, logger.level)
			assert.Equal(t, tt.expected.structured, logger.structured)
			if tt.expected.baseLogger != nil {
				assert.NotNil(t, logger.baseLogger)
			}
		})
	}
}

// TestLoggerLevel проверяет корректность преобразования строкового уровня логирования в zapcore.Level.
func TestLoggerLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected zapcore.Level
	}{
		{
			name:     "Debug level",
			level:    "debug",
			expected: zapcore.DebugLevel,
		},
		{
			name:     "Info level",
			level:    "info",
			expected: zapcore.InfoLevel,
		},
		{
			name:     "Warn level",
			level:    "warn",
			expected: zapcore.WarnLevel,
		},
		{
			name:     "Error level",
			level:    "error",
			expected: zapcore.ErrorLevel,
		},
		{
			name:     "Unknown level",
			level:    "unknown",
			expected: zapcore.DebugLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &Logger{level: tt.level}
			assert.Equal(t, tt.expected, logger.getLoggerLevel())
		})
	}
}

// TestLoggerInit проверяет инициализацию логгера с консольным выводом и без него.
func TestLoggerInit(t *testing.T) {
	tests := []struct {
		name          string
		consoleOutput bool
	}{
		{
			name:          "Console output enabled",
			consoleOutput: true,
		},
		{
			name:          "Console output disabled",
			consoleOutput: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "logger_test")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			// Перенаправляем stdout для перехвата вывода в консоль
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			require.NoError(t, err)
			os.Stdout = w

			// Инициализируем логгер
			logger := NewLogger(Path(tmpDir))
			logger.InitLogger(tt.consoleOutput)

			assert.NotNil(t, logger.baseLogger)
			assert.NotNil(t, logger.sugarLogger)

			// Записываем тестовое сообщение
			logger.Info("Test log message")

			// Восстанавливаем stdout
			w.Close()
			os.Stdout = oldStdout

			// Читаем перехваченный вывод
			var consoleOutput string
			if tt.consoleOutput {
				buf := new(bytes.Buffer)
				_, err := io.Copy(buf, r)
				require.NoError(t, err)
				consoleOutput = buf.String()
			}

			// Проверяем, что файл был создан и содержит лог
			files, err := os.ReadDir(tmpDir)
			require.NoError(t, err)
			assert.NotEmpty(t, files, "Log file should be created")

			filePath := filepath.Join(tmpDir, files[0].Name())
			content, err := os.ReadFile(filePath)
			require.NoError(t, err)
			assert.Contains(t, string(content), "Test log message", "Log file should contain the log message")

			// Проверяем, что сообщение было выведено в консоль (если консольный вывод включен)
			if tt.consoleOutput {
				assert.Contains(t, consoleOutput, "Test log message", "Log message should be printed to console")
			} else {
				assert.Empty(t, consoleOutput, "No output should be printed to console")
			}
		})
	}
}

// TestLoggerClose проверяет корректное закрытие логгера и освобождение ресурсов.
func TestLoggerClose(t *testing.T) {
	logger := NewLogger(Path("/tmp/logs"))
	logger.InitLogger(true)

	err := logger.Close()
	assert.NoError(t, err)
}

// TestFileRotatorWrite проверяет запись данных в файл.
func TestFileRotatorWrite(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected int
		err      error
	}{
		{
			name:     "Write success",
			data:     []byte("test log"),
			expected: 8,
			err:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rotator := &fileRotator{path: "/tmp/logs"}
			n, err := rotator.Write(tt.data)
			assert.Equal(t, tt.expected, n)
			assert.Equal(t, tt.err, err)
		})
	}
}

// TestFileRotatorRotate проверяет ротацию файла.
func TestFileRotatorRotate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "logger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	rotator := &fileRotator{path: tmpDir, compress: false}

	err = rotator.openNew(time.Now().AddDate(0, 0, -1))
	require.NoError(t, err)

	err = rotator.rotate()
	assert.NoError(t, err)

	// Проверяем, что было создано два файла
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, 2, len(files), "Expected two files after rotation")
}

// TestFileRotatorClose проверяет корректное закрытие файла.
func TestFileRotatorClose(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "logger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	rotator := &fileRotator{path: tmpDir}
	err = rotator.openNew(time.Now())
	require.NoError(t, err)

	err = rotator.Close()
	assert.NoError(t, err)
}

// TestFileRotatorCompress проверяет сжатие файла после ротации.
func TestFileRotatorCompress(t *testing.T) {
	// Создаем временный файл для тестирования
	tmpFile, err := os.CreateTemp("", "test_log_*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Записываем данные в файл
	_, err = tmpFile.Write([]byte("test log data"))
	require.NoError(t, err)
	tmpFile.Close()

	// Выполняем сжатие файла
	compressFile(tmpFile.Name())

	// Проверяем, что сжатый файл был создан
	zipFilePath := tmpFile.Name() + ".zip"
	_, err = os.Stat(zipFilePath)
	assert.NoError(t, err, "Compressed file should exist")

	// Проверяем, что исходный файл был удален
	_, err = os.Stat(tmpFile.Name())
	assert.True(t, os.IsNotExist(err), "Original file should be deleted")

	// Удаляем сжатый файл после теста
	os.Remove(zipFilePath)
}

// TestLoggerMethods проверяет методы логирования.
func TestLoggerMethods(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "logger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logger := NewLogger(Path(tmpDir), Level("debug"))
	logger.InitLogger(true)

	tests := []struct {
		name     string
		method   func()
		expected string
	}{
		{
			name:     "Debug",
			method:   func() { logger.Debug("debug message") },
			expected: "debug message",
		},
		{
			name:     "Info",
			method:   func() { logger.Info("info message") },
			expected: "info message",
		},
		{
			name:     "Warn",
			method:   func() { logger.Warn("warn message") },
			expected: "warn message",
		},
		{
			name:     "Error",
			method:   func() { logger.Error("error message") },
			expected: "error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.method()

			// Проверяем, что лог был записан в файл
			files, err := os.ReadDir(tmpDir)
			require.NoError(t, err)
			assert.NotEmpty(t, files, "Log file should be created")

			// Читаем содержимое файла и проверяем, что оно содержит лог
			logFilePath := filepath.Join(tmpDir, files[0].Name())
			content, err := os.ReadFile(logFilePath)
			require.NoError(t, err)
			assert.Contains(t, string(content), tt.expected, "Log file should contain the log message")
		})
	}
}

// TestLoggerWithFields проверяет логирование с дополнительными полями.
func TestLoggerWithFields(t *testing.T) {
	logger := NewLogger()
	logger.InitLogger(true)

	fields := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	newLogger := logger.WithFields(fields)
	assert.NotNil(t, newLogger)
}

// TestIntegrationLoggerAndFileRotator проверяет интеграцию логгера и fileRotator.
func TestIntegrationLoggerAndFileRotator(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "logger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logger := NewLogger(Path(tmpDir))
	logger.InitLogger(false)

	logger.Info("Integration test log")

	err = logger.Close()
	assert.NoError(t, err)
}

func TestStructuredLogging(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "logger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logger := NewLogger(Path(tmpDir), Structured(true))
	logger.InitLogger(false)

	logger.Info("Test log message")

	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.NotEmpty(t, files)

	filePath := filepath.Join(tmpDir, files[0].Name())
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)

	// Проверяем, что вывод является валидным JSON
	var logEntry map[string]interface{}
	err = json.Unmarshal(content, &logEntry)
	require.NoError(t, err, "Log output should be valid JSON")

	// Проверяем, что лог содержит необходимые поля
	message, exists := logEntry["message"]
	require.True(t, exists, "Log entry should contain 'message' key")
	assert.Equal(t, "Test log message", message, "Log message should match")
}

func TestConcurrentLogging(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "logger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logger := NewLogger(Path(tmpDir))
	logger.InitLogger(false)

	var wg sync.WaitGroup
	numGoroutines := 10
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			logger.Infof("Log message from goroutine %d", i)
		}(i)
	}
	wg.Wait()

	// Проверяем, что все сообщения записаны
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.NotEmpty(t, files)

	filePath := filepath.Join(tmpDir, files[0].Name())
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)

	// Подсчитываем количество строк в лог-файле
	lines := strings.Split(string(content), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	assert.Equal(t, numGoroutines, len(lines), "Expected %d log entries, got %d", numGoroutines, len(lines))
}

func TestInvalidLogLevel(t *testing.T) {
	logger := NewLogger(Level("invalid_level"))
	logger.InitLogger(true)

	assert.Equal(t, "info", logger.level)
}

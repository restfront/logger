package logger

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func BenchmarkLogger(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "logger_test")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	logger := NewLogger(Path(tmpDir))
	logger.InitLogger(false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark log message")
	}
}

package worker

import (
	"bufio"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogFile_Follow(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), logFilePattern)
	require.NoError(t, err)

	logFile, err := NewLogFile(file.Name())
	require.NoError(t, err)
	defer func() { require.NoError(t, logFile.Close()) }()

	writer := bufio.NewWriter(file)

	const wantInitialLog = "some initial log"
	const wantDelayedLog = "some delayed log"

	// Write initial logs before follow
	_, err = writer.WriteString(wantInitialLog + "\n")
	require.NoError(t, err)
	require.NoError(t, writer.Flush())

	// Follow and read initial logs
	stopCh := make(chan bool)
	defer func() { stopCh <- true }()
	logCh, err := logFile.Follow(stopCh)
	require.NoError(t, err)
	require.Equal(t, wantInitialLog, <-logCh)

	// Write more logs and read on fsnotify write events
	for i := 0; i < 10; i++ {
		_, err = writer.WriteString(wantDelayedLog + "\n")
		require.NoError(t, err)
		require.NoError(t, writer.Flush())
		require.Equal(t, wantDelayedLog, <-logCh)
	}
}

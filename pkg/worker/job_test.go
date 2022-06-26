package worker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJob(t *testing.T) {
	numLogs := 5
	wantLog := "some string"

	cmd := echoLoop(numLogs, 0.1, wantLog)
	job, err := NewJob(cmd, t.TempDir())
	require.NoError(t, err)
	require.Equal(t, JobStatePending, job.Status.State)

	err = job.Start()
	require.NoError(t, err)
	require.Equal(t, JobStateRunning, job.Status.State)

	logCh, _, err := job.FollowLogs()
	require.NoError(t, err)
	for i := 0; i < numLogs; i++ {
		require.Equal(t, wantLog, <-logCh)
	}

	job.Wait()
	require.Equal(t, JobStateCompleted, job.Status.State)
}

package worker

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWorker_jobCompletes(t *testing.T) {
	const wantLog = "test"
	const numLogs = 10
	const delay = 0.1

	worker := NewWorker(t.TempDir())

	var jobID string
	t.Run("start new job success", func(t *testing.T) {
		job, err := worker.StartJob(echoLoop(numLogs, delay, wantLog))
		require.NoError(t, err)
		require.NotEmpty(t, job.ID)
		jobID = job.ID
	})

	t.Run("query running job success", func(t *testing.T) {
		jobStatus, err := worker.QueryJob(jobID)
		require.NoError(t, err)
		require.Equal(t, JobStateRunning, jobStatus.State)
	})

	t.Run("follow logs until job done", func(t *testing.T) {
		logCh, _, err := worker.FollowLogs(jobID)
		require.NoError(t, err)
		assertLogs(t, logCh, wantLog, numLogs)
	})

	t.Run("query completed job success", func(t *testing.T) {
		jobStatus, err := worker.QueryJob(jobID)
		require.NoError(t, err)
		require.Equal(t, JobStateCompleted, jobStatus.State)
		require.Equal(t, 0, jobStatus.ExitCode)
	})
}

func TestWorker_stopRunningJob(t *testing.T) {
	const wantLog = "test"
	const numLogs = 200
	const delay = 0.1

	worker := NewWorker(t.TempDir())

	var jobID string
	t.Run("start new job success", func(t *testing.T) {
		job, err := worker.StartJob(echoLoop(numLogs, delay, wantLog))
		require.NoError(t, err)
		require.NotEmpty(t, job.ID)
		jobID = job.ID
	})

	t.Run("query running job success", func(t *testing.T) {
		jobStatus, err := worker.QueryJob(jobID)
		require.NoError(t, err)
		require.Equal(t, JobStateRunning, jobStatus.State)
	})

	t.Run("follow logs for 0.5 seconds", func(t *testing.T) {
		logCh, cancel, err := worker.FollowLogs(jobID)
		require.NoError(t, err)

		go func() {
			time.Sleep(500 * time.Millisecond)
			cancel()
		}()

		for log := range logCh {
			require.Equal(t, wantLog, log)
		}
	})

	t.Run("stop running job success", func(t *testing.T) {
		err := worker.StopJob(jobID)
		require.NoError(t, err)
	})

	t.Run("query completed job success", func(t *testing.T) {
		jobStatus, err := worker.QueryJob(jobID)
		require.NoError(t, err)
		require.Equal(t, JobStateCompleted, jobStatus.State)
		require.Equal(t, -1, jobStatus.ExitCode)
		require.Contains(t, jobStatus.ExitError.Error(), "signal: killed")
	})
}

func TestWorker_concurrentLogFollowers(t *testing.T) {
	const wantLog = "test"
	const numLogs = 10
	const delay = 0.1

	worker := NewWorker(t.TempDir())

	job, err := worker.StartJob(echoLoop(numLogs, delay, wantLog))
	require.NoError(t, err)

	logCh1, _, err := worker.FollowLogs(job.ID)
	require.NoError(t, err)
	logCh2, _, err := worker.FollowLogs(job.ID)
	require.NoError(t, err)

	go func() { assertLogs(t, logCh1, wantLog, numLogs) }()
	go func() { assertLogs(t, logCh2, wantLog, numLogs) }()
}

func assertLogs(t *testing.T, logCh <-chan string, wantLog string, numLogs int) {
	gotLogs := 0
	for log := range logCh {
		require.Equal(t, wantLog, log)
		gotLogs++
	}

	require.Equal(t, numLogs, gotLogs)
}

func echoLoop(iterations int, delay float64, echo string) Command {
	return Command{
		Cmd:  "bash",
		Args: []string{"-c", fmt.Sprintf("for i in {1..%d}; do echo %s; sleep %f; done", iterations, echo, delay)},
	}
}

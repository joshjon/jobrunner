package worker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const logFilePattern = "jobrunner_log_"

type JobState int

const (
	JobStateUnspecified JobState = iota
	JobStatePending
	JobStateRunning
	JobStateCompleted
)

type CancelFunc func()

type Command struct {
	Cmd  string
	Args []string
}

type JobStatus struct {
	State     JobState
	ExitCode  int
	ExitError error
}

type Job struct {
	sync.Mutex
	ID      string
	Status  JobStatus
	cmd     *exec.Cmd
	logFile *os.File
	done    bool
}

func NewJob(command Command, logDir string) (*Job, error) {
	jobID := uuid.New().String()
	path := filepath.Join(logDir, fmt.Sprintf("%s.log", jobID))

	logFile, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(command.Cmd, command.Args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	job := &Job{
		ID: jobID,
		Status: JobStatus{
			State: JobStatePending,
		},
		cmd:     cmd,
		logFile: logFile,
	}

	return job, nil
}

func (j *Job) Start() error {
	j.Status.State = JobStateRunning

	if err := j.cmd.Start(); err != nil {
		j.Status.State = JobStateCompleted
		return err
	}

	go func() {
		err := j.cmd.Wait()
		j.Lock()
		defer j.Unlock()

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				j.Status.ExitCode = exitErr.ExitCode()
			}
			j.Status.ExitError = err
		}

		j.Status.State = JobStateCompleted

		if err := j.logFile.Close(); err != nil {
			zap.L().Error("error closing log file", zap.Error(err))
		}

		j.done = true
	}()

	return nil
}

func (j *Job) FollowLogs() (<-chan string, CancelFunc, error) {
	logs, err := NewLogFile(j.logFile.Name())
	if err != nil {
		return nil, nil, err
	}

	stopCh := make(chan bool)

	go func() {
		j.Wait()
		stopCh <- true
	}()

	cancelFunc := func() {
		stopCh <- true
	}

	logCh, err := logs.Follow(stopCh)
	if err != nil {
		return nil, nil, err
	}

	return logCh, cancelFunc, nil
}

func (j *Job) Wait() {
	for {
		if j.done {
			return
		}
	}
}

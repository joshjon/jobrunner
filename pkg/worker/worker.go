package worker

import (
	"errors"
	"sync"
)

var ErrorJobNotFound = errors.New("job not found")

type Worker struct {
	jobs   sync.Map
	logDir string
}

func NewWorker(logDir string) *Worker {
	return &Worker{
		jobs:   sync.Map{},
		logDir: logDir,
	}
}

func (w *Worker) StartJob(command Command) (*Job, error) {
	job, err := NewJob(command, w.logDir)
	if err != nil {
		return nil, err
	}

	w.jobs.Store(job.ID, job)

	if err := job.Start(); err != nil {
		return nil, err
	}

	return job, nil
}

func (w *Worker) StopJob(jobID string) error {
	if val, ok := w.jobs.Load(jobID); ok {
		if job, ok := val.(*Job); ok && job != nil {
			if err := job.cmd.Process.Kill(); err != nil {
				return err
			}
			job.Wait()
			return nil
		}
	}
	return ErrorJobNotFound
}

func (w *Worker) QueryJob(jobID string) (JobStatus, error) {
	if val, ok := w.jobs.Load(jobID); ok {
		if job, ok := val.(*Job); ok && job != nil {
			return job.Status, nil
		}
	}
	return JobStatus{}, ErrorJobNotFound
}

func (w *Worker) FollowLogs(jobID string) (<-chan string, CancelFunc, error) {
	if val, ok := w.jobs.Load(jobID); ok {
		if job, ok := val.(*Job); ok && job != nil {
			return job.FollowLogs()
		}
	}
	return nil, nil, ErrorJobNotFound
}

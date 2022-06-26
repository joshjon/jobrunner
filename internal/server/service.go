//go:generate go run github.com/golang/mock/mockgen -destination=mock_worker_test.go -package=server . Worker

package server

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/joshjon/jobrunner/internal/auth"
	"github.com/joshjon/jobrunner/pkg/worker"
	servicepb "github.com/joshjon/jobrunner/proto/gen/service/v1"
)

const (
	objectWildcard = "*"
	createAction   = "create"
	readAction     = "read"
	deleteAction   = "delete"
)

type Worker interface {
	StartJob(worker.Command) (*worker.Job, error)
	StopJob(string) error
	QueryJob(string) (worker.JobStatus, error)
	FollowLogs(string) (<-chan string, worker.CancelFunc, error)
}

var (
	ErrorJobNotFound    = status.Error(codes.NotFound, "Job not found")
	ErrorInternalServer = status.Error(codes.Internal, "Internal server error")
)

type Service struct {
	servicepb.UnimplementedServiceServer
	worker     Worker
	authorizer Authorizer
}

func NewService(logDir string, aclModelFile string, aclPolicyFile string) *Service {
	return &Service{
		worker:     worker.NewWorker(logDir),
		authorizer: auth.New(aclModelFile, aclPolicyFile),
	}
}

func (s *Service) Start(ctx context.Context, req *servicepb.StartRequest) (*servicepb.StartResponse, error) {
	if err := s.authorizer.Authorize(subject(ctx), objectWildcard, createAction); err != nil {
		return nil, err
	}

	cmd := worker.Command{
		Cmd:  req.Command.Cmd,
		Args: req.Command.Args,
	}

	job, err := s.worker.StartJob(cmd)
	if err != nil {
		return nil, s.handleError(err)
	}

	return &servicepb.StartResponse{
		JobId: job.ID,
	}, nil
}

func (s *Service) Stop(ctx context.Context, req *servicepb.StopRequest) (*servicepb.StopResponse, error) {
	if err := s.authorizer.Authorize(subject(ctx), objectWildcard, deleteAction); err != nil {
		return nil, err
	}

	err := s.worker.StopJob(req.JobId)
	if err != nil {
		return nil, s.handleError(err)
	}
	return &servicepb.StopResponse{}, nil
}

func (s *Service) Query(ctx context.Context, req *servicepb.QueryRequest) (*servicepb.QueryResponse, error) {
	if err := s.authorizer.Authorize(subject(ctx), objectWildcard, readAction); err != nil {
		return nil, err
	}

	jobStatus, err := s.worker.QueryJob(req.JobId)
	if err != nil {
		return nil, s.handleError(err)
	}

	var state servicepb.State

	switch jobStatus.State {
	case worker.JobStateUnspecified:
		state = servicepb.State_STATE_UNSPECIFIED
	case worker.JobStateRunning:
		state = servicepb.State_STATE_RUNNING
	case worker.JobStateCompleted:
		state = servicepb.State_STATE_COMPLETED
	}

	resp := &servicepb.QueryResponse{
		JobStatus: &servicepb.JobStatus{
			Id:       req.JobId,
			State:    state,
			ExitCode: int64(jobStatus.ExitCode),
		},
	}

	return resp, nil
}

func (s *Service) FollowLogs(req *servicepb.FollowLogsRequest, stream servicepb.Service_FollowLogsServer) error {
	if err := s.authorizer.Authorize(subject(stream.Context()), objectWildcard, readAction); err != nil {
		return err
	}

	logCh, cancel, err := s.worker.FollowLogs(req.JobId)
	if err != nil {
		return s.handleError(err)
	}
	defer cancel()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case log, ok := <-logCh:
			if !ok {
				return nil
			}
			resp := &servicepb.FollowLogsResponse{
				Log: log,
			}
			if err = stream.Send(resp); err != nil {
				return s.handleError(err)
			}
		}
	}
}

func (s *Service) handleError(err error) error {
	if err.Error() == ErrorJobNotFound.Error() {
		return ErrorJobNotFound
	}

	zap.L().Error("unexpected error occurred", zap.Error(err))
	return ErrorInternalServer
}

type Authorizer interface {
	Authorize(subject, object, action string) error
}

func subject(ctx context.Context) string {
	return ctx.Value(subjectContextKey{}).(string)
}

type subjectContextKey struct{}

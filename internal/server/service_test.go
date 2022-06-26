package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/joshjon/jobrunner/internal/auth"
	"github.com/joshjon/jobrunner/pkg/worker"
	servicepb "github.com/joshjon/jobrunner/proto/gen/service/v1"
)

const (
	port    = 9595
	timeout = 5 * time.Second
)

var (
	CAFile               = certFile("ca.pem")
	ServerCertFile       = certFile("server.pem")
	ServerKeyFile        = certFile("server-key.pem")
	RootClientCertFile   = certFile("root-client.pem")
	RootClientKeyFile    = certFile("root-client-key.pem")
	NobodyClientCertFile = certFile("nobody-client.pem")
	NobodyClientKeyFile  = certFile("nobody-client-key.pem")
	ACLModelFile         = configFile("model.conf")
	ACLPolicyFile        = configFile("policy.csv")
)

var rpcAddr = &net.TCPAddr{
	IP:   []byte{127, 0, 0, 1},
	Port: port,
}

func TestService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockWorker := NewMockWorker(ctrl)

	srv := newServer(t, mockWorker)
	defer srv.Stop()
	client := newClient(t, RootClientCertFile, RootClientKeyFile)

	const wantLog = "test"
	const numLogs = 10
	logCh := mockLogs(wantLog, numLogs)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var err error
	var startResp *servicepb.StartResponse

	t.Run("start new job success", func(t *testing.T) {
		wantJob := &worker.Job{ID: "1", Status: worker.JobStatus{State: worker.JobStateRunning}}
		mockWorker.EXPECT().StartJob(gomock.Any()).Return(wantJob, nil).Times(1)
		startResp, err = client.Start(ctx, &servicepb.StartRequest{Command: &servicepb.Command{Cmd: "some-command"}})
		require.NoError(t, err)
		require.NotEmpty(t, startResp.JobId)
	})

	t.Run("query running success", func(t *testing.T) {
		wantJobStatus := worker.JobStatus{State: worker.JobStateRunning}
		mockWorker.EXPECT().QueryJob(gomock.Any()).Return(wantJobStatus, nil).Times(1)
		queryResp, err := client.Query(ctx, &servicepb.QueryRequest{JobId: startResp.JobId})
		require.NoError(t, err)
		require.Equal(t, servicepb.State_STATE_RUNNING, queryResp.JobStatus.State)
		require.Equal(t, startResp.JobId, queryResp.JobStatus.Id)
	})

	t.Run("follow logs", func(t *testing.T) {
		mockWorker.EXPECT().FollowLogs(gomock.Any()).Return(logCh, func() {}, nil).Times(1)
		streamClient, err := client.FollowLogs(ctx, &servicepb.FollowLogsRequest{JobId: startResp.JobId})
		require.NoError(t, err)

		for i := 0; i < numLogs; i++ {
			logResp, err := streamClient.Recv()
			require.NoError(t, err)
			require.Equal(t, wantLog, logResp.Log)
		}
	})

	t.Run("stop running job success", func(t *testing.T) {
		mockWorker.EXPECT().StopJob(gomock.Any()).Return(nil).Times(1)
		_, err := client.Stop(ctx, &servicepb.StopRequest{JobId: startResp.JobId})
		require.NoError(t, err)
	})

	t.Run("query completed job success", func(t *testing.T) {
		wantJobStatus := worker.JobStatus{State: worker.JobStateCompleted}
		mockWorker.EXPECT().QueryJob(gomock.Any()).Return(wantJobStatus, nil).Times(1)
		queryResp, err := client.Query(ctx, &servicepb.QueryRequest{JobId: startResp.JobId})
		require.NoError(t, err)
		require.Equal(t, servicepb.State_STATE_COMPLETED, queryResp.JobStatus.State)
		require.Equal(t, startResp.JobId, queryResp.JobStatus.Id)
	})
}

func TestService_jobNotFoundError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockWorker := NewMockWorker(ctrl)

	srv := newServer(t, mockWorker)
	defer srv.Stop()
	client := newClient(t, RootClientCertFile, RootClientKeyFile)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	mockWorker.EXPECT().QueryJob(gomock.Any()).Return(worker.JobStatus{}, ErrorJobNotFound)
	mockWorker.EXPECT().StopJob(gomock.Any()).Return(ErrorJobNotFound)
	mockWorker.EXPECT().FollowLogs(gomock.Any()).Return(nil, nil, ErrorJobNotFound)

	tests := []struct {
		name string
		rpc  func() (any, error)
	}{
		{
			name: "query job not found error",
			rpc:  func() (any, error) { return client.Query(ctx, &servicepb.QueryRequest{JobId: "unknown"}) },
		},
		{
			name: "stop job not found error",
			rpc:  func() (any, error) { return client.Stop(ctx, &servicepb.StopRequest{JobId: "unknown"}) },
		},
		{
			name: "follow job not found error",
			rpc: func() (any, error) {
				streamClient, err := client.FollowLogs(ctx, &servicepb.FollowLogsRequest{JobId: "unknown"})
				assert.NoError(t, err)
				return streamClient.Recv()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.rpc()
			require.EqualError(t, err, ErrorJobNotFound.Error())
		})
	}
}

func TestService_insecureCredentials(t *testing.T) {
	srv := newServer(t, &MockWorker{})
	defer srv.Stop()

	conn, err := grpc.Dial(rpcAddr.String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	client := servicepb.NewServiceClient(conn)

	_, err = client.Start(context.Background(), &servicepb.StartRequest{})
	s, _ := status.FromError(err)
	assert.Equal(t, codes.Unavailable, s.Code())
}

func TestService_unauthorized(t *testing.T) {
	srv := newServer(t, &MockWorker{})
	defer srv.Stop()
	ctx := context.Background()
	client := newClient(t, NobodyClientCertFile, NobodyClientKeyFile)

	tests := []struct {
		name   string
		rpc    func() (any, error)
		action string
	}{
		{
			name:   "start unauthorized",
			action: createAction,
			rpc:    func() (any, error) { return client.Start(ctx, &servicepb.StartRequest{}) },
		},
		{
			name:   "query unauthorized",
			action: readAction,
			rpc:    func() (any, error) { return client.Query(ctx, &servicepb.QueryRequest{}) },
		},
		{
			name:   "follow unauthorized",
			action: readAction,
			rpc: func() (any, error) {
				streamClient, err := client.FollowLogs(ctx, &servicepb.FollowLogsRequest{})
				assert.NoError(t, err)
				return streamClient.Recv()
			},
		},
		{
			name:   "stop unauthorized",
			action: deleteAction,
			rpc:    func() (any, error) { return client.Stop(ctx, &servicepb.StopRequest{}) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.rpc()
			s, _ := status.FromError(err)
			assert.Equal(t, codes.PermissionDenied, s.Code())
			assert.Equal(t, fmt.Sprintf("nobody not permitted to %s on *", tt.action), s.Message())
		})
	}
}

func newServer(t *testing.T, worker Worker) *Server {
	tlsCfg, err := newTLSConfig(auth.TypeServer, ServerCertFile, ServerKeyFile)
	require.NoError(t, err)

	service := &Service{
		worker:     worker,
		authorizer: auth.New(ACLModelFile, ACLPolicyFile),
	}

	srv, err := New(Config{
		Address: rpcAddr.String(),
		Service: service,
		TLS:     tlsCfg,
	})
	require.NoError(t, err)

	go func() {
		err = srv.Serve()
		require.NoError(t, err)
	}()

	return srv
}

func newClient(t *testing.T, certFile string, keyFile string) servicepb.ServiceClient {
	tlsCfg, err := newTLSConfig(auth.TypeClient, certFile, keyFile)
	require.NoError(t, err)
	conn, err := grpc.Dial(rpcAddr.String(), grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	require.NoError(t, err)
	return servicepb.NewServiceClient(conn)
}

func newTLSConfig(tlsType auth.Type, certFile string, keyFile string) (*tls.Config, error) {
	return auth.SetupTLSConfig(auth.TLSConfig{
		Type:          tlsType,
		CertFile:      certFile,
		KeyFile:       keyFile,
		CAFile:        CAFile,
		ServerAddress: rpcAddr.IP.String(),
	})
}

func mockLogs(log string, num int) chan string {
	logCh := make(chan string)
	go func() {
		for i := 0; i < num; i++ {
			logCh <- log
		}
	}()
	return logCh
}

func certFile(filename string) string {
	_, b, _, _ := runtime.Caller(0)
	dir := fmt.Sprintf("%s/../../testdata", filepath.Dir(b))
	return filepath.Join(dir, filename)
}

func configFile(filename string) string {
	_, b, _, _ := runtime.Caller(0)
	dir := fmt.Sprintf("%s/../../config/cert", filepath.Dir(b))
	return filepath.Join(dir, filename)
}

package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"testing"

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

const port = 9595

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

func TestService_StartJob(t *testing.T) {
	deps := setup(t, RootClientCertFile, RootClientKeyFile)
	defer deps.close()
	wantJob := &worker.Job{ID: "1", Status: worker.JobStatus{State: worker.JobStateRunning}}
	deps.mockWorker.EXPECT().StartJob(gomock.Any()).Return(wantJob, nil).Times(1)
	startResp, err := deps.client.Start(context.Background(), &servicepb.StartRequest{Command: &servicepb.Command{Cmd: "some-command"}})
	require.NoError(t, err)
	require.NotEmpty(t, startResp.JobId)
}

func TestService_QueryJob(t *testing.T) {
	deps := setup(t, RootClientCertFile, RootClientKeyFile)
	defer deps.close()
	jobID := "job-id"
	wantJobStatus := worker.JobStatus{State: worker.JobStateRunning}
	deps.mockWorker.EXPECT().QueryJob(gomock.Any()).Return(wantJobStatus, nil).Times(1)
	queryResp, err := deps.client.Query(context.Background(), &servicepb.QueryRequest{JobId: jobID})
	require.NoError(t, err)
	require.Equal(t, servicepb.State_STATE_RUNNING, queryResp.JobStatus.State)
	require.Equal(t, jobID, queryResp.JobStatus.Id)
}

func TestService_FollowLogs(t *testing.T) {
	deps := setup(t, RootClientCertFile, RootClientKeyFile)
	defer deps.close()
	jobID, wantLog, numLogs := "job-id", "test", 10
	logCh := mockLogs(wantLog, numLogs)
	deps.mockWorker.EXPECT().FollowLogs(gomock.Any()).Return(logCh, func() {}, nil).Times(1)
	streamClient, err := deps.client.FollowLogs(context.Background(), &servicepb.FollowLogsRequest{JobId: jobID})
	require.NoError(t, err)

	for i := 0; i < numLogs; i++ {
		logResp, err := streamClient.Recv()
		require.NoError(t, err)
		require.Equal(t, wantLog, logResp.Log)
	}
}

func TestService_StopJob(t *testing.T) {
	deps := setup(t, RootClientCertFile, RootClientKeyFile)
	defer deps.close()
	jobID := "job-id"
	deps.mockWorker.EXPECT().StopJob(gomock.Any()).Return(nil).Times(1)
	_, err := deps.client.Stop(context.Background(), &servicepb.StopRequest{JobId: jobID})
	require.NoError(t, err)
}

func TestService_jobNotFoundError(t *testing.T) {
	deps := setup(t, RootClientCertFile, RootClientKeyFile)
	defer deps.close()
	deps.mockWorker.EXPECT().QueryJob(gomock.Any()).Return(worker.JobStatus{}, ErrorJobNotFound)
	deps.mockWorker.EXPECT().StopJob(gomock.Any()).Return(ErrorJobNotFound)
	deps.mockWorker.EXPECT().FollowLogs(gomock.Any()).Return(nil, nil, ErrorJobNotFound)
	ctx := context.Background()

	tests := []struct {
		name string
		rpc  func() (any, error)
	}{
		{
			name: "query job not found error",
			rpc:  func() (any, error) { return deps.client.Query(ctx, &servicepb.QueryRequest{JobId: "unknown"}) },
		},
		{
			name: "stop job not found error",
			rpc:  func() (any, error) { return deps.client.Stop(ctx, &servicepb.StopRequest{JobId: "unknown"}) },
		},
		{
			name: "follow job not found error",
			rpc: func() (any, error) {
				streamClient, err := deps.client.FollowLogs(ctx, &servicepb.FollowLogsRequest{JobId: "unknown"})
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
	defer conn.Close()
	require.NoError(t, err)
	client := servicepb.NewServiceClient(conn)
	_, err = client.Start(context.Background(), &servicepb.StartRequest{})
	s, _ := status.FromError(err)
	assert.Equal(t, codes.Unavailable, s.Code())
}

func TestService_unauthorized(t *testing.T) {
	deps := setup(t, NobodyClientCertFile, NobodyClientKeyFile)
	defer deps.close()
	ctx := context.Background()

	tests := []struct {
		name   string
		rpc    func() (any, error)
		action string
	}{
		{
			name:   "start unauthorized",
			action: createAction,
			rpc:    func() (any, error) { return deps.client.Start(ctx, &servicepb.StartRequest{}) },
		},
		{
			name:   "query unauthorized",
			action: readAction,
			rpc:    func() (any, error) { return deps.client.Query(ctx, &servicepb.QueryRequest{}) },
		},
		{
			name:   "follow unauthorized",
			action: readAction,
			rpc: func() (any, error) {
				streamClient, err := deps.client.FollowLogs(ctx, &servicepb.FollowLogsRequest{})
				assert.NoError(t, err)
				return streamClient.Recv()
			},
		},
		{
			name:   "stop unauthorized",
			action: deleteAction,
			rpc:    func() (any, error) { return deps.client.Stop(ctx, &servicepb.StopRequest{}) },
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

type dependencies struct {
	server     *Server
	client     servicepb.ServiceClient
	clientConn *grpc.ClientConn
	mockWorker *MockWorker
	ctrl       *gomock.Controller
}

func (d *dependencies) close() {
	d.ctrl.Finish()
	d.clientConn.Close()
	d.server.Stop()
}

func setup(t *testing.T, clientCert string, clientKey string) *dependencies {
	ctrl := gomock.NewController(t)
	mockWorker := NewMockWorker(ctrl)
	srv := newServer(t, mockWorker)
	client, conn := newClient(t, clientCert, clientKey)
	return &dependencies{
		server:     srv,
		client:     client,
		clientConn: conn,
		mockWorker: mockWorker,
		ctrl:       ctrl,
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

func newClient(t *testing.T, certFile string, keyFile string) (servicepb.ServiceClient, *grpc.ClientConn) {
	tlsCfg, err := newTLSConfig(auth.TypeClient, certFile, keyFile)
	require.NoError(t, err)
	conn, err := grpc.Dial(rpcAddr.String(), grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	require.NoError(t, err)
	return servicepb.NewServiceClient(conn), conn
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

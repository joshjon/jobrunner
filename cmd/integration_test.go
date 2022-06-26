//go:build integration

package main

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/joshjon/jobrunner/internal/auth"
	servicepb "github.com/joshjon/jobrunner/proto/gen/service/v1"
)

const (
	port    = 9090
	timeout = 5 * time.Second
)

var rpcAddr = &net.TCPAddr{
	IP:   []byte{127, 0, 0, 1},
	Port: port,
}

var (
	CAFile             = certFile("ca.pem")
	RootClientCertFile = certFile("root-client.pem")
	RootClientKeyFile  = certFile("root-client-key.pem")
)

func TestServiceIntegration(t *testing.T) {
	client := newClient(t, RootClientCertFile, RootClientKeyFile)

	const wantLog = "test"
	const numLogs = 10
	const delay = 0.1

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var err error
	var startResp *servicepb.StartResponse

	t.Run("start new job success", func(t *testing.T) {
		startResp, err = client.Start(ctx, &servicepb.StartRequest{Command: echoLoop(numLogs, delay, wantLog)})
		require.NoError(t, err)
		require.NotEmpty(t, startResp.JobId)
	})

	t.Run("query running success", func(t *testing.T) {
		queryResp, err := client.Query(ctx, &servicepb.QueryRequest{JobId: startResp.JobId})
		require.NoError(t, err)
		require.Equal(t, servicepb.State_STATE_RUNNING, queryResp.JobStatus.State)
		require.Equal(t, startResp.JobId, queryResp.JobStatus.Id)
	})

	t.Run("follow logs", func(t *testing.T) {
		streamClient, err := client.FollowLogs(ctx, &servicepb.FollowLogsRequest{JobId: startResp.JobId})
		require.NoError(t, err)

		for i := 0; i < numLogs; i++ {
			logResp, err := streamClient.Recv()
			require.NoError(t, err)
			require.Equal(t, wantLog, logResp.Log)
		}
	})
}

func newClient(t *testing.T, certFile string, keyFile string) servicepb.ServiceClient {
	tlsCfg, err := auth.SetupTLSConfig(auth.TLSConfig{
		Type:          auth.TypeClient,
		CertFile:      certFile,
		KeyFile:       keyFile,
		CAFile:        CAFile,
		ServerAddress: rpcAddr.IP.String(),
	})
	require.NoError(t, err)
	conn, err := grpc.DialContext(context.Background(), rpcAddr.String(), grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	require.NoError(t, err)
	return servicepb.NewServiceClient(conn)
}

func certFile(filename string) string {
	_, b, _, _ := runtime.Caller(0)
	dir := fmt.Sprintf("%s/../certs", filepath.Dir(b))
	return filepath.Join(dir, filename)
}

func echoLoop(iterations int, delay float64, echo string) *servicepb.Command {
	return &servicepb.Command{
		Cmd:  "bash",
		Args: []string{"-c", fmt.Sprintf("for i in {1..%d}; do echo %s; sleep %f; done", iterations, echo, delay)},
	}
}

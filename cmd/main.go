package main

import (
	"net"
	"os"

	"go.uber.org/zap"

	"github.com/joshjon/jobrunner/internal/auth"
	"github.com/joshjon/jobrunner/internal/config"
	"github.com/joshjon/jobrunner/internal/server"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal("error loading config", zap.Error(err))
	}

	rpcAddr := &net.TCPAddr{
		IP:   []byte{0, 0, 0, 0},
		Port: cfg.Port,
	}

	tlsConfig, err := auth.SetupTLSConfig(auth.TLSConfig{
		CertFile:      cfg.Cert.ServerCertFile,
		KeyFile:       cfg.Cert.ServerKeyFile,
		CAFile:        cfg.Cert.CAFile,
		ServerAddress: rpcAddr.IP.String(),
		Type:          auth.TypeServer,
	})
	if err != nil {
		logger.Fatal("error setting up tls config", zap.Error(err))
	}

	serverCfg := server.Config{
		Address: rpcAddr.String(),
		TLS:     tlsConfig,
		Service: server.NewService(os.TempDir(), cfg.Cert.ACLModelFile, cfg.Cert.ACLPolicyFile),
	}
	srv, err := server.New(serverCfg)
	if err != nil {
		logger.Fatal("error creating server", zap.Error(err))
	}

	logger.Info("serving on " + srv.Address())
	if err = srv.Serve(); err != nil {
		logger.Panic("error serving jobrunner server", zap.Error(err))
	}
}

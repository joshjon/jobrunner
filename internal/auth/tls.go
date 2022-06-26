package auth

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
)

type Type int

const (
	TypeUnspecified Type = iota
	TypeServer
	TypeClient
)

type TLSConfig struct {
	CertFile      string
	KeyFile       string
	CAFile        string
	ServerAddress string
	Type          Type
}

func SetupTLSConfig(cfg TLSConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, err
	}

	bytes, err := ioutil.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, err
	}

	ca := x509.NewCertPool()
	if ok := ca.AppendCertsFromPEM(bytes); !ok {
		return nil, fmt.Errorf("failed to parse root certificate: %q", cfg.CAFile)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
		ServerName:   cfg.ServerAddress,
	}

	if cfg.Type == TypeServer {
		tlsConfig.ClientCAs = ca
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	} else if cfg.Type == TypeClient {
		tlsConfig.RootCAs = ca
	} else {
		return nil, fmt.Errorf("tls type required")
	}

	return tlsConfig, nil
}

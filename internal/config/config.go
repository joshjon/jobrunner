package config

import (
	"errors"
	"flag"
	"os"

	"gopkg.in/yaml.v2"
)

type Cert struct {
	ACLModelFile   string `yaml:"ACLModelFile"`
	ACLPolicyFile  string `yaml:"ACLPolicyFile"`
	ServerCertFile string `yaml:"ServerCertFile"`
	ServerKeyFile  string `yaml:"ServerKeyFile"`
	CAFile         string `yaml:"CAFile"`
}

type Config struct {
	Port int  `yaml:"Port"`
	Cert Cert `yaml:"Cert"`
}

func LoadConfig() (*Config, error) {
	var cfg Config

	var filepath string
	flag.StringVar(&filepath, "config", "", "yaml config file path")
	flag.Parse()
	if filepath == "" {
		return nil, errors.New("required flag 'config' was not provided")
	}

	if err := readFile(filepath, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func readFile(filePath string, out *Config) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}

	if err = yaml.NewDecoder(f).Decode(out); err != nil {
		return err
	}

	return f.Close()
}

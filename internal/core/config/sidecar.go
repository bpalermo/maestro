package config

import (
	"os"

	"buf.build/go/protoyaml"
	configV1 "github.com/bpalermo/maestro/config/v1"
	"github.com/bufbuild/protovalidate-go"
)

type SidecarConfig struct {
	cfg *configV1.SidecarConfiguration
}

func NewSidecarConfig() *SidecarConfig {
	return &SidecarConfig{}
}

func NewSidecarConfigFromFile(filename string) (*SidecarConfig, error) {
	sidecarCfg := &SidecarConfig{}

	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	err = sidecarCfg.Set(string(b))
	if err != nil {
		return nil, err
	}

	return sidecarCfg, nil
}

func (c *SidecarConfig) String() string {
	y, err := protoyaml.Marshal(c.cfg)
	if err != nil {
		return ""
	}
	return string(y)
}

func (c *SidecarConfig) Set(config string) error {
	cfg := &configV1.SidecarConfiguration{}

	validator, err := protovalidate.New()
	if err != nil {
		return err
	}

	if err := protoyaml.Unmarshal([]byte(config), cfg); err != nil {
		return err
	}

	err = validator.Validate(cfg)
	if err != nil {
		return err
	}

	c.cfg = cfg

	return nil
}

func (c *SidecarConfig) Type() string {
	return "sidecarConfig"
}

func (c *SidecarConfig) GetConfig() *configV1.SidecarConfiguration {
	return c.cfg
}

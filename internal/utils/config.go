package utils

import (
	"io"
	"os"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	BotToken  string `validate:"required" yaml:"botToken"`
	ChannelID string `validate:"required" yaml:"channelID"`
	DbURI     string `validate:"required" yaml:"dbURI"`
}

func ReadAndValidateConfig(configPath string) (*AppConfig, error) {
	configFile, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}

	configBytes, err := io.ReadAll(configFile)
	if err != nil {
		return nil, err
	}

	var config AppConfig
	if err = yaml.Unmarshal(configBytes, &config); err != nil {
		return nil, err
	}

	v := validator.New(validator.WithRequiredStructEnabled())
	if err = v.Struct(&config); err != nil {
		return nil, err
	}

	return &config, nil

}

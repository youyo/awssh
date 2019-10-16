package awssh

import (
	"os"
	"path/filepath"
	"strings"

	ini "gopkg.in/ini.v1"
)

type (
	Config struct {
		Data     *ini.File
		Profiles []string
	}
)

func NewConfig() *Config {
	return new(Config)
}

func (c *Config) Load() (err error) {
	configFilePath := filepath.Join(os.Getenv("HOME"), ".aws/config")
	data, err := ini.Load(configFilePath)
	if err != nil {
		return err
	}

	c.Data = data

	return nil
}

func (c *Config) ListProfiles() (profiles []string) {
	for _, section := range c.Data.Sections() {
		if section.HasKey("role_arn") {
			profile := strings.Replace(section.Name(), "profile ", "", 1)
			profiles = append(profiles, profile)
		}
	}
	return profiles
}

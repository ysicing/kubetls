package config

import (
	"fmt"
	"io/ioutil"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

type Config struct {
	CR  Secrets `json:"cr" yaml:"cr"`
	TLS Secrets `json:"tls" yaml:"tls"`
}

type Secrets struct {
	Version string    `json:"version" yaml:"version"`
	Secrets []*Secret `json:"secrets" yaml:"secrets"`
}

type Secret struct {
	Name string `json:"name" yaml:"name"`
	Type string `json:"type" yaml:"type"`
	Data string `json:"data,omitempty" yaml:"data,omitempty"`
	Key  string `json:"key,omitempty" yaml:"key,omitempty"`
	Crt  string `json:"crt,omitempty" yaml:"crt,omitempty"`
}

func LoadFile(path string) (*Config, error) {
	r := new(Config)
	b, err := ioutil.ReadFile(path)
	if err != nil {
		logrus.Debugf("couldn't load repositories file (%s), err: %v", path, err)
		return r, fmt.Errorf("couldn't load config file (%s)", path)
	}
	err = yaml.Unmarshal(b, r)
	if err != nil {
		logrus.Debugf("yaml unmarshal err: %v", err)
		return nil, err
	}
	return r, nil
}

// HasImagePullSecret returns true if the given name is already a repository name.
func (r *Config) HasImagePullSecret(name string) bool {
	entry := r.GetImagePullSecret(name)
	return entry != nil
}

// GetImagePullSecret returns an entry with the given name if it exists, otherwise returns nil
func (r *Config) GetImagePullSecret(name string) *Secret {
	for _, entry := range r.CR.Secrets {
		if entry.Name == name {
			return entry
		}
	}
	return nil
}

// HasImagePullSecret returns true if the given name is already a repository name.
func (r *Config) HasExtTLS(name string) bool {
	entry := r.GetExtTLS(name)
	return entry != nil
}

// GetImagePullSecret returns an entry with the given name if it exists, otherwise returns nil
func (r *Config) GetExtTLS(name string) *Secret {
	for _, entry := range r.TLS.Secrets {
		if entry.Name == name {
			return entry
		}
	}
	return nil
}

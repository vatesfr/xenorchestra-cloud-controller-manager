/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package xenorchestra

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// xoConfig is Xen Orchestra cloud config.
type xoConfig struct {
	URL      string `yaml:"url"`
	Insecure bool   `yaml:"insecure,omitempty"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
	Token    string `yaml:"token,omitempty"`
	// Do we need to set the pool IDs?
}

// readCloudConfig reads cloud config from a reader.
func readCloudConfig(config io.Reader) (xoConfig, error) {
	cfg := xoConfig{}

	if config != nil {
		if err := yaml.NewDecoder(config).Decode(&cfg); err != nil {
			return xoConfig{}, err
		}
	}

	if cfg.Username != "" && cfg.Password != "" {
		if cfg.Token != "" {
			return xoConfig{}, fmt.Errorf("token is not allowed when username and password are set")
		}
	} else if cfg.Token == "" {
		return xoConfig{}, fmt.Errorf("either token or username/password are required for authentication")
	}

	if cfg.URL == "" || !strings.HasPrefix(cfg.URL, "http") {
		return xoConfig{}, fmt.Errorf("url is required")
	}

	return cfg, nil
}

// readCloudConfigFromFile reads cloud config from a file.
func readCloudConfigFromFile(file string) (xoConfig, error) {
	f, err := os.Open(filepath.Clean(file))
	if err != nil {
		return xoConfig{}, fmt.Errorf("error reading %s: %v", file, err)
	}
	defer f.Close() // nolint: errcheck

	return readCloudConfig(f)
}

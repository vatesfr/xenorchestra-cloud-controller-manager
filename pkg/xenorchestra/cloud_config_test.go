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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadCloudConfig(t *testing.T) {
	cfg, err := readCloudConfig(nil)
	assert.NotNil(t, err)
	assert.NotNil(t, cfg)

	// Empty config
	cfg, err = readCloudConfig(strings.NewReader(`
`))
	assert.NotNil(t, err)
	assert.NotNil(t, cfg)

	// Wrong config
	cfg, err = readCloudConfig(strings.NewReader(`
test: false
`))

	assert.NotNil(t, err)
	assert.NotNil(t, cfg)

	// Non full config
	cfg, err = readCloudConfig(strings.NewReader(`
url: abcd
token: 123ABC
`))

	assert.NotNil(t, err)
	assert.NotNil(t, cfg)

	// Valid config with one cluster
	cfg, err = readCloudConfig(strings.NewReader(`
url: https://example.com
insecure: false
token: "123ABC"
`))
	assert.Nil(t, err)
	assert.NotNil(t, cfg)

	// Valid config with one cluster (username/password)
	cfg, err = readCloudConfig(strings.NewReader(`
url: https://example.com
insecure: false
username: "user@pam"
password: "secret"
`))
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
}

func TestReadCloudConfigFromFile(t *testing.T) {
	cfg, err := readCloudConfigFromFile("testdata/cloud-config.yaml")
	assert.NotNil(t, err)
	assert.EqualError(t, err, "error reading testdata/cloud-config.yaml: open testdata/cloud-config.yaml: no such file or directory")
	assert.NotNil(t, cfg)

	cfg, err = readCloudConfigFromFile("../../hack/xo-config.yaml")
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
}

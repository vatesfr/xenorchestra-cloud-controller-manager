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

	xok8s "github.com/vatesfr/xenorchestra-k8s-common"
)

func TestNewCloudError(t *testing.T) {
	cloud, err := newCloud(&xok8s.XoConfig{})
	assert.NotNil(t, err)
	assert.Nil(t, cloud)
	assert.EqualError(t, err, "url is required")
}

func TestCloud(t *testing.T) {
	cfg, err := xok8s.ReadCloudConfig(strings.NewReader(`
url: https://example.com
insecure: false
token: "12ABC"
`))
	assert.Nil(t, err)
	assert.NotNil(t, cfg)

	cloud, err := newCloud(&cfg)
	assert.Nil(t, err)
	assert.NotNil(t, cloud)

	lb, res := cloud.LoadBalancer()
	assert.Nil(t, lb)
	assert.Equal(t, res, false)

	ins, res := cloud.Instances()
	assert.Nil(t, ins)
	assert.Equal(t, res, false)

	ins2, res := cloud.InstancesV2()
	assert.NotNil(t, ins2)
	assert.Equal(t, res, true)

	zone, res := cloud.Zones()
	assert.Nil(t, zone)
	assert.Equal(t, res, false)

	cl, res := cloud.Clusters()
	assert.Nil(t, cl)
	assert.Equal(t, res, false)

	route, res := cloud.Routes()
	assert.Nil(t, route)
	assert.Equal(t, res, false)

	pName := cloud.ProviderName()
	assert.Equal(t, pName, xok8s.ProviderName)

	clID := cloud.HasClusterID()
	assert.Equal(t, clID, true)
}

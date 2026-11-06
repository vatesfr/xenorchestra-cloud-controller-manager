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
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	mock_library "github.com/vatesfr/xenorchestra-cloud-controller-manager/pkg/xenorchestra/mocks"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
	xok8s "github.com/vatesfr/xenorchestra-k8s-common"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	cloudprovider "k8s.io/cloud-provider"
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

func TestRecordCloudProviderEvent(t *testing.T) {
	client := fake.NewClientset()

	err := recordCloudProviderEvent(t.Context(), client, corev1.EventTypeWarning, eventReasonFailedToCheckClient, "Failed to check Xen Orchestra client: tls: failed to verify certificate")
	assert.NoError(t, err)

	events, err := client.EventsV1().Events(metav1.NamespaceSystem).List(t.Context(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, events.Items, 1)

	event := events.Items[0]
	assert.Equal(t, corev1.EventTypeWarning, event.Type)
	assert.Equal(t, eventActionCheckClient, event.Action)
	assert.Equal(t, eventReasonFailedToCheckClient, event.Reason)
	assert.Contains(t, event.Note, "tls: failed to verify certificate")
	assert.Equal(t, ProviderName, event.ReportingController)
	assert.Equal(t, cloudControllerManagerClientName, event.ReportingInstance)
	assert.Equal(t, metav1.NamespaceSystem, event.Regarding.Namespace)
	assert.Equal(t, ProviderName, event.Regarding.Name)
	assert.Equal(t, componentKind, event.Regarding.Kind)
	assert.Nil(t, event.Series)
}

func TestRecordCloudProviderEventUsesPodReference(t *testing.T) {
	t.Setenv(podNameEnv, "xoccm-123")
	t.Setenv(podNamespaceEnv, "custom-system")
	t.Setenv(podUIDEnv, "87db141c-8f50-4b8e-9d23-0c8de730216a")

	client := fake.NewClientset()

	err := recordCloudProviderEvent(t.Context(), client, corev1.EventTypeWarning, eventReasonFailedToCheckClient, "Failed to check Xen Orchestra client: tls: failed to verify certificate")
	assert.NoError(t, err)

	events, err := client.EventsV1().Events("custom-system").List(t.Context(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, events.Items, 1)

	event := events.Items[0]
	assert.Equal(t, "custom-system", event.Namespace)
	assert.Equal(t, "custom-system", event.Regarding.Namespace)
	assert.Equal(t, "xoccm-123", event.Regarding.Name)
	assert.Equal(t, podKind, event.Regarding.Kind)
	assert.Equal(t, "87db141c-8f50-4b8e-9d23-0c8de730216a", string(event.Regarding.UID))
	assert.Equal(t, "xoccm-123", event.ReportingInstance)
}

func TestRecordCloudProviderEventUpdatesExistingEventSeries(t *testing.T) {
	t.Setenv(podNameEnv, "xoccm-123")
	t.Setenv(podNamespaceEnv, "custom-system")
	t.Setenv(podUIDEnv, "87db141c-8f50-4b8e-9d23-0c8de730216a")

	client := fake.NewClientset()

	err := recordCloudProviderEvent(t.Context(), client, corev1.EventTypeWarning, eventReasonFailedToCheckClient, "Failed to check Xen Orchestra client: tls: failed to verify certificate")
	assert.NoError(t, err)
	err = recordCloudProviderEvent(t.Context(), client, corev1.EventTypeWarning, eventReasonFailedToCheckClient, "Failed to check Xen Orchestra client: tls: failed to verify certificate")
	assert.NoError(t, err)

	events, err := client.EventsV1().Events("custom-system").List(t.Context(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, events.Items, 1)
	assert.NotNil(t, events.Items[0].Series)
	assert.Equal(t, int32(2), events.Items[0].Series.Count)
}

// fakeClientBuilder only implements ClientOrDie, the one method Initialize uses.
type fakeClientBuilder struct {
	cloudprovider.ControllerClientBuilder
	client clientset.Interface
}

func (b fakeClientBuilder) ClientOrDie(string) clientset.Interface { return b.client }

func TestInitializeRecordsCheckClientEvent(t *testing.T) {
	onePool := []*payloads.Pool{{NameLabel: "pool"}}

	tests := []struct {
		name      string
		pools     []*payloads.Pool
		poolErr   error
		wantEvent string
		wantExit  []int
	}{
		{name: "reachable", pools: onePool, wantEvent: corev1.EventTypeNormal + "/" + eventReasonClientReady},
		{
			name: "unreachable", poolErr: errors.New("dial tcp: connection refused"),
			wantEvent: corev1.EventTypeWarning + "/" + eventReasonFailedToCheckClient, wantExit: []int{1},
		},
		{
			name: "bad gateway", poolErr: apiError("pools", http.StatusBadGateway, ""),
			wantEvent: corev1.EventTypeWarning + "/" + eventReasonFailedToCheckClient, wantExit: []int{1},
		},
		{
			name: "unauthorized", poolErr: apiError("pools", http.StatusUnauthorized, ""),
			wantEvent: corev1.EventTypeWarning + "/" + eventReasonFailedToAuthenticate, wantExit: []int{1},
		},
		{
			name: "forbidden", poolErr: apiError("pools", http.StatusForbidden, ""),
			wantEvent: corev1.EventTypeWarning + "/" + eventReasonFailedToAuthenticate, wantExit: []int{1},
		},
		{
			name: "no pool", pools: []*payloads.Pool{},
			wantEvent: corev1.EventTypeWarning + "/" + eventReasonNoPoolAvailable, wantExit: []int{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			lib := mock_library.NewMockLibrary(ctrl)
			pool := mock_library.NewMockPool(ctrl)
			lib.EXPECT().Pool().Return(pool)
			pool.EXPECT().GetAll(gomock.Any(), 1, "").Return(tt.pools, tt.poolErr)

			if len(tt.pools) > 0 {
				vm := mock_library.NewMockVM(ctrl)
				lib.EXPECT().VM().Return(vm)
				vm.EXPECT().GetAll(gomock.Any(), 1, "").Return([]*payloads.VM{}, nil)
			}

			var exitCodes []int
			oldExit := exit
			exit = func(_ time.Duration, code int) { exitCodes = append(exitCodes, code) }
			t.Cleanup(func() { exit = oldExit })

			stop := make(chan struct{})
			t.Cleanup(func() { close(stop) })

			kube := fake.NewClientset()
			c := &cloud{client: &xok8s.XoClient{Client: lib}}
			c.Initialize(fakeClientBuilder{client: kube}, stop)

			assert.Equal(t, tt.wantExit, exitCodes)

			events, err := kube.EventsV1().Events(metav1.NamespaceSystem).List(t.Context(), metav1.ListOptions{})
			assert.NoError(t, err)
			assert.Len(t, events.Items, 1)
			assert.Equal(t, tt.wantEvent, events.Items[0].Type+"/"+events.Items[0].Reason)
		})
	}
}

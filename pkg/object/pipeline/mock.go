/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package pipeline

import (
	"sync"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/supervisor"
)

type mockFilter struct{}

type mockSpec struct{}

var _ Filter = (*mockFilter)(nil)

func (f *mockFilter) Kind() string                                              { return "MockFilter" }
func (f *mockFilter) DefaultSpec() interface{}                                  { return &mockSpec{} }
func (f *mockFilter) Description() string                                       { return "mock filter" }
func (f *mockFilter) Results() []string                                         { return nil }
func (f *mockFilter) Init(filterSpec *FilterSpec)                               {}
func (f *mockFilter) Inherit(filterSpec *FilterSpec, previousGeneration Filter) {}
func (f *mockFilter) Status() interface{}                                       { return nil }
func (f *mockFilter) Close()                                                    {}

// MockMQTTFilter is used for test pipeline, which will count the client number of MQTTContext
type MockMQTTFilter struct {
	mockFilter
	mu sync.Mutex

	spec        *MockMQTTSpec
	clients     map[string]int
	disconnect  map[string]struct{}
	subscribe   map[string][]string
	unsubscribe map[string][]string
}

// MockMQTTSpec is spec of MockMQTTFilter
type MockMQTTSpec struct {
	UserName    string   `yaml:"userName" jsonschema:"required"`
	Password    string   `yaml:"password" jsonschema:"required"`
	Port        uint16   `yaml:"port" jsonschema:"required"`
	BackendType string   `yaml:"backendType" jsonschema:"required"`
	EarlyStop   bool     `yaml:"earlyStop" jsonschema:"omitempty"`
	KeysToStore []string `yaml:"keysToStore" jsonschema:"omitempty"`
	ConnectKey  string   `yaml:"connectKey" jsonschema:"omitempty"`
}

// MockMQTTStatus is status of MockMQTTFilter
type MockMQTTStatus struct {
	ClientCount      map[string]int
	ClientDisconnect map[string]struct{}
	Subscribe        map[string][]string
	Unsubscribe      map[string][]string
}

var _ MQTTFilter = (*MockMQTTFilter)(nil)

// Kind retrun kind of MockMQTTFilter
func (m *MockMQTTFilter) Kind() string {
	return "MockMQTTFilter"
}

// DefaultSpec retrun default spec of MockMQTTFilter
func (m *MockMQTTFilter) DefaultSpec() interface{} {
	return &MockMQTTSpec{}
}

// Init init MockMQTTFilter
func (m *MockMQTTFilter) Init(filterSpec *FilterSpec) {
	m.spec = filterSpec.FilterSpec().(*MockMQTTSpec)
	m.clients = make(map[string]int)
	m.disconnect = make(map[string]struct{})
	m.subscribe = make(map[string][]string)
	m.unsubscribe = make(map[string][]string)
}

// HandleMQTT handle MQTTContext
func (m *MockMQTTFilter) HandleMQTT(ctx context.MQTTContext) *context.MQTTResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[ctx.Client().ClientID()]++

	switch ctx.PacketType() {
	case context.MQTTConnect:
		ctx.Client().Store(m.spec.ConnectKey, struct{}{})
		if ctx.ConnectPacket().Username != m.spec.UserName || string(ctx.ConnectPacket().Password) != m.spec.Password {
			ctx.SetDisconnect()
		}
	case context.MQTTDisconnect:
		m.disconnect[ctx.Client().ClientID()] = struct{}{}
	case context.MQTTSubscribe:
		m.subscribe[ctx.Client().ClientID()] = ctx.SubscribePacket().Topics
	case context.MQTTUnsubscribe:
		m.unsubscribe[ctx.Client().ClientID()] = ctx.UnsubscribePacket().Topics
	}

	for _, k := range m.spec.KeysToStore {
		ctx.Client().Store(k, struct{}{})
	}
	return nil
}

// Status return status of MockMQTTFilter
func (m *MockMQTTFilter) Status() interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	clientCount := make(map[string]int)
	for k, v := range m.clients {
		clientCount[k] = v
	}
	disconnect := make(map[string]struct{})
	for k := range m.disconnect {
		disconnect[k] = struct{}{}
	}
	subscribe := make(map[string][]string)
	for k, v := range m.subscribe {
		vv := make([]string, len(v))
		copy(vv, v)
		subscribe[k] = v
	}
	unsubscribe := make(map[string][]string)
	for k, v := range m.unsubscribe {
		vv := make([]string, len(v))
		copy(vv, v)
		unsubscribe[k] = vv
	}
	return MockMQTTStatus{
		ClientCount:      clientCount,
		ClientDisconnect: disconnect,
		Subscribe:        subscribe,
		Unsubscribe:      unsubscribe,
	}
}

type mockMQTTClient struct {
	cid      string
	userName string
	kvMap    sync.Map
}

var _ context.MQTTClient = (*mockMQTTClient)(nil)

// ClientID return client id of mockMQTTClient
func (c *mockMQTTClient) ClientID() string {
	return c.cid
}

// UserName return username of mockMQTTClient
func (c *mockMQTTClient) UserName() string {
	return c.userName
}

func (c *mockMQTTClient) Load(key interface{}) (interface{}, bool) {
	return c.kvMap.Load(key)
}

func (c *mockMQTTClient) Store(key interface{}, value interface{}) {
	c.kvMap.Store(key, value)
}

func (c *mockMQTTClient) Delete(key interface{}) {
	c.kvMap.Delete(key)
}

// MockFilterSpec help to create FilterSpec for test
func MockFilterSpec(super *supervisor.Supervisor, rawSpec map[string]interface{}, yamlConfig string,
	meta *FilterMetaSpec, filterSpec interface{}) *FilterSpec {
	return &FilterSpec{
		super:      super,
		rawSpec:    rawSpec,
		yamlConfig: yamlConfig,
		meta:       meta,
		filterSpec: filterSpec,
	}
}

// MockGetFilter is used to get running filter from pipeline
func MockGetFilter(p *Pipeline, name string) Filter {
	for _, f := range p.runningFilters {
		if f.spec.Name() == name {
			return f.filter
		}
	}
	return nil
}

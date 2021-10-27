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
)

type mockFilter struct {
}

type mockSpec struct {
}

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

	mu      sync.Mutex
	spec    *MockMQTTSpec
	clients map[string]int
}

// MockMQTTSpec is spec of MockMQTTFilter
type MockMQTTSpec struct {
	UserName    string `yaml:"userName" jsonschema:"required"`
	Port        uint16 `yaml:"port" jsonschema:"required"`
	BackendType string `yaml:"backendType" jsonschema:"required"`
}

// MockMQTTStatus is status of MockMQTTFilter
type MockMQTTStatus map[string]int

var _ MQTTFilter = (*MockMQTTFilter)(nil)

func (m *MockMQTTFilter) Kind() string {
	return "MockMQTTFilter"
}

func (m *MockMQTTFilter) DefaultSpec() interface{} {
	return &MockMQTTSpec{}
}

func (m *MockMQTTFilter) Init(filterSpec *FilterSpec) {
	m.spec = filterSpec.FilterSpec().(*MockMQTTSpec)
	m.clients = make(map[string]int)
}

func (m *MockMQTTFilter) HandleMQTT(ctx context.MQTTContext) MQTTResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[ctx.Client().ClientID()]++
	return nil
}

func (m *MockMQTTFilter) clientCount() map[string]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	ans := make(map[string]int)
	for k, v := range m.clients {
		ans[k] = v
	}
	return ans
}

func (m *MockMQTTFilter) Status() interface{} {
	return MockMQTTStatus(m.clientCount())
}

type mockMQTTClient struct {
	cid      string
	userName string
}

var _ context.MQTTClient = (*mockMQTTClient)(nil)

func (c *mockMQTTClient) ClientID() string {
	return c.cid
}

func (c *mockMQTTClient) UserName() string {
	return c.userName
}

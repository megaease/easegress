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
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

type MockedFilter struct {
	kind  *filters.Kind
	spec  *MockedSpec
	count int
}

type MockedSpec struct {
	filters.BaseSpec `yaml:",inline"`
}

type MockedStatus struct {
	Count int
}

func (m *MockedFilter) Name() string                              { return m.spec.Name() }
func (m *MockedFilter) Kind() *filters.Kind                       { return m.kind }
func (m *MockedFilter) Spec() filters.Spec                        { return nil }
func (m *MockedFilter) Close()                                    {}
func (m *MockedFilter) Init()                                     {}
func (m *MockedFilter) Inherit(previousGeneration filters.Filter) {}

func (m *MockedFilter) HeaderKV() (string, string) {
	return fmt.Sprintf("X-Mock-%s", m.spec.Name()), m.spec.Name()
}

func (m *MockedFilter) Status() interface{} {
	return &MockedStatus{m.count}
}

func (m *MockedFilter) Handle(ctx *context.Context) (result string) {
	m.count++
	req := ctx.GetInputRequest()
	if r, ok := req.(*httpprot.Request); ok {
		k, v := m.HeaderKV()
		r.HTTPHeader().Set(k, v)
	}
	return ""
}

func MockFilterKind(kind string, results []string) *filters.Kind {
	k := &filters.Kind{
		Name:        kind,
		Description: kind,
		Results:     results,
		DefaultSpec: func() filters.Spec {
			return &MockedSpec{}
		},
	}
	k.CreateInstance = func(spec filters.Spec) filters.Filter {
		return &MockedFilter{
			kind: k,
			spec: spec.(*MockedSpec),
		}
	}
	return k
}

func cleanup() {
	filters.ResetRegistry()
}

func TestSpecValidate(t *testing.T) {
	t.Run("spec missing filter definition", func(t *testing.T) {
		cleanup()
		filters.Register(MockFilterKind("mock-filter", nil))

		spec := `name: pipeline
kind: Pipeline
flow:
- filter: filter-1
filters:
- filter: filter-2
  kind: mock-filter`
		_, err := supervisor.NewSpec(spec)
		assert.NotNil(t, err, "filter-1 not found")
	})

	t.Run("valid jumpIf", func(t *testing.T) {
		cleanup()
		filters.Register(MockFilterKind("mock-filter", []string{"invalid"}))

		spec := `name: pipeline
kind: Pipeline
flow:
- filter: filter-1
  jumpIf:
    invalid: filter-2
- filter: filter-2
- filter: END
filters:
- name: filter-2
  kind: mock-filter
- name: filter-1
  kind: mock-filter`

		_, err := supervisor.NewSpec(spec)
		assert.Nil(t, err, "valid spec")
	})

	t.Run("invalid jumpIf", func(t *testing.T) {
		cleanup()
		filters.Register(MockFilterKind("mock-filter", []string{"invalid"}))

		spec := `name: pipeline
kind: Pipeline
flow:
- filter: filter-1
  jumpIf:
    invalid: filter-3
- filter: filter-2
filters:
- name: filter-2
  kind: mock-filter
- name: filter-1
  kind: mock-filter`

		_, err := supervisor.NewSpec(spec)
		assert.NotNil(t, err, "invalid spec")
	})

	t.Run("ordered filters without flow", func(t *testing.T) {
		cleanup()
		filters.Register(MockFilterKind("mock-filter", nil))

		spec := `name: pipeline
kind: Pipeline
filters:
- name: filter-2
  kind: mock-filter
- name: filter-1
  kind: mock-filter`

		_, err := supervisor.NewSpec(spec)
		assert.Nil(t, err, "valid spec")
	})

	t.Run("duplicate filter", func(t *testing.T) {
		cleanup()
		filters.Register(MockFilterKind("mock-filter", nil))

		spec := `name: pipeline
kind: Pipeline
filters:
- name: filter-1
  kind: mock-filter
- name: filter-1
  kind: mock-filter`

		_, err := supervisor.NewSpec(spec)
		assert.NotNil(t, err, "invalid spec")
	})
}

func TestRegistry(t *testing.T) {
	cleanup()
	t.Run("duplicate filter name", func(t *testing.T) {
		defer func() {
			if err := recover(); err == nil {
				t.Errorf("register did not panic")
			}
		}()
		filters.Register(MockFilterKind("mock-filter", nil))
		filters.Register(MockFilterKind("mock-filter", nil))
	})
	cleanup()
	t.Run("empty kind", func(t *testing.T) {
		defer func() {
			if err := recover(); err == nil {
				t.Errorf("register did not panic")
			}
		}()
		filters.Register(MockFilterKind("", nil))
	})
	cleanup()
	t.Run("repeated results", func(t *testing.T) {
		defer func() {
			if err := recover(); err == nil {
				t.Errorf("register did not panic")
			}
		}()
		results := []string{"res1", "res2", "res3", "res1"}
		filters.Register(MockFilterKind("filter", results))
	})
	cleanup()
	t.Run("export registry", func(t *testing.T) {
		filters.Register(MockFilterKind("mock-pipeline-2", nil))
		count := 0
		filters.WalkKind(func(k *filters.Kind) bool {
			count++
			return true
		})
		if count != 1 {
			t.Errorf("couldn't get the filter")
		}
	})
	cleanup()
}

func TestHTTPPipeline(t *testing.T) {
	superSpecYaml := `
name: http-pipeline-test
kind: Pipeline
flow:
  - filter: validator
    jumpIf: { invalid: END }
  - filter: requestAdaptor
    jumpIf: { specialCase: proxy }
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
    mainPool:
      servers:
        - url: http://127.0.0.1:9095
      loadBalance:
        policy: roundRobin
  - name: requestAdaptor
    kind: RequestAdaptor
    header:
      set:
        X-Adapt-Key: goodplan
  - name: validator
    kind: Validator
    headers:
      Content-Type:
        values:
        - application/json
`
	filters.Register(MockFilterKind("Proxy", nil))
	filters.Register(MockFilterKind("Pipeline", nil))
	t.Run("missing filter results", func(t *testing.T) {
		filters.Register(MockFilterKind("Validator", nil))
		filters.Register(MockFilterKind("RequestAdaptor", nil))
		_, err := supervisor.NewSpec(superSpecYaml)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
		filters.Unregister("Validator")
		filters.Unregister("RequestAdaptor")
	})
	filters.Register(MockFilterKind("Validator", []string{"invalid", "END"}))
	filters.Register(MockFilterKind("RequestAdaptor", []string{"specialCase"}))
	superSpec, err := supervisor.NewSpec(superSpecYaml)
	if err != nil {
		t.Errorf("failed to create spec %s", err)
	}
	pipeline := Pipeline{nil, nil, map[string]filters.Filter{}, nil, nil}
	pipeline.Init(superSpec, nil)
	pipeline.Inherit(superSpec, &pipeline, nil)

	status := pipeline.Status()
	if reflect.TypeOf(status).Kind() == reflect.Struct {
		t.Errorf("should be type of Status")
	}
	if pipeline.getFilter("unknown") != nil {
		t.Errorf("should not have filters")
	}
	pipeline.Close()
	cleanup()
}

func TestHTTPPipelineNoFlow(t *testing.T) {
	superSpecYaml := `
name: http-pipeline-test
kind: Pipeline
filters:
  - name: validator
    kind: Validator
    headers:
      Content-Type:
        values:
        - application/json
  - name: requestAdaptor
    kind: RequestAdaptor
    header:
      set:
        X-Adapt-Key: goodplan
  - name: proxy
    kind: Proxy
    mainPool:
      servers:
      - url: http://127.0.0.1:9095
      loadBalance:
        policy: roundRobin
`
	filters.Register(MockFilterKind("Proxy", nil))
	filters.Register(MockFilterKind("Pipeline", nil))
	filters.Register(MockFilterKind("Validator", nil))
	filters.Register(MockFilterKind("RequestAdaptor", nil))

	superSpec, err := supervisor.NewSpec(superSpecYaml)
	if err != nil {
		t.Errorf("failed to create spec %s", err)
	}
	pipeline := Pipeline{nil, nil, map[string]filters.Filter{}, nil, nil}
	pipeline.Init(superSpec, nil)
	pipeline.Inherit(superSpec, &pipeline, nil)

	status := pipeline.Status()
	if reflect.TypeOf(status).Kind() == reflect.Struct {
		t.Errorf("should be type of Status")
	}
	if pipeline.getFilter("unknown") != nil {
		t.Errorf("should not have filters")
	}
	if pipeline.getFilter("proxy") == nil {
		t.Errorf("should have filter")
	}
	pipeline.Close()
	cleanup()
}

func TestHandle(t *testing.T) {
	assert := assert.New(t)
	superSpecYaml := `
name: http-pipeline-test
kind: Pipeline
flow:
  - filter: filter1
  - filter: filter2 
    jumpIf:
      "": END
  - filter: filter3
filters:
  - name: filter1 
    kind: Filter1 
  - name: filter2
    kind: Filter2 
  - name: filter3
    kind: Filter2 
data:
  foo: bar
`
	filters.Register(MockFilterKind("Filter1", nil))
	filters.Register(MockFilterKind("Filter2", nil))
	superSpec, err := supervisor.NewSpec(superSpecYaml)
	assert.Nil(err)

	pipeline := &Pipeline{}
	pipeline.Init(superSpec, nil)
	defer pipeline.Close()
	defer cleanup()

	stdReq, err := http.NewRequest(http.MethodGet, "http://localhost:9095", nil)
	assert.Nil(err)
	req, err := httpprot.NewRequest(stdReq)
	assert.Nil(err)

	ctx := context.New(tracing.NoopSpan)
	ctx.SetRequest(context.DefaultNamespace, req)

	pipeline.Handle(ctx)

	filter1 := MockGetFilter(pipeline, "filter1").(*MockedFilter)
	k1, v1 := filter1.HeaderKV()
	filter2 := MockGetFilter(pipeline, "filter2").(*MockedFilter)
	k2, v2 := filter2.HeaderKV()
	assert.Equal(v1, stdReq.Header.Get(k1))
	assert.Equal(v2, stdReq.Header.Get(k2))

	tags := ctx.Tags()
	assert.Contains(tags, "filter1")
	assert.Contains(tags, "filter2")
	assert.NotContains(tags, "filter3")

	status := pipeline.Status().ObjectStatus.(*Status)
	assert.Equal(3, len(status.Filters))
	assert.Empty(status.ToMetrics("123"), "no metrics")

	var value string
	assert.NotPanics(func() {
		value = ctx.GetData("PIPELINE").(map[string]interface{})["foo"].(string)
	})
	assert.Equal("bar", value)
}

func TestHandleWithBeforeAfter(t *testing.T) {
	assert := assert.New(t)

	stdReq, err := http.NewRequest(http.MethodGet, "http://localhost:9095", nil)
	assert.Nil(err)
	req, err := httpprot.NewRequest(stdReq)
	assert.Nil(err)

	filters.Register(MockFilterKind("Filter1", nil))
	defer cleanup()

	yamlSpec := `
name: http-pipeline-test
kind: Pipeline
flow:
  - filter: filter2
filters:
  - name: filter2 
    kind: Filter1 
`
	spec, err := supervisor.NewSpec(yamlSpec)
	assert.Nil(err)

	pipeline := &Pipeline{}
	pipeline.Init(spec, nil)
	defer pipeline.Close()

	ctx := context.New(tracing.NoopSpan)
	ctx.SetRequest(context.DefaultNamespace, req)

	pipeline.HandleWithBeforeAfter(ctx, nil, nil)
	tags := ctx.Tags()
	assert.NotContains(tags, "filter1")
	assert.Contains(tags, "filter2")
	assert.NotContains(tags, "filter3")

	yamlSpec = `
name: http-pipeline-after
kind: Pipeline
flow:
  - filter: filter3
filters:
  - name: filter3 
    kind: Filter1 
`

	spec, err = supervisor.NewSpec(yamlSpec)
	assert.Nil(err)

	after := &Pipeline{}
	after.Init(spec, nil)
	defer after.Close()

	ctx = context.New(tracing.NoopSpan)
	ctx.SetRequest(context.DefaultNamespace, req)
	pipeline.HandleWithBeforeAfter(ctx, nil, after)
	tags = ctx.Tags()
	assert.NotContains(tags, "filter1")
	assert.Contains(tags, "filter2")
	assert.Contains(tags, "filter3")

	yamlSpec = `
name: http-pipeline-before
kind: Pipeline
flow:
  - filter: filter1
  - filter: END
filters:
  - name: filter1 
    kind: Filter1 
`
	spec, err = supervisor.NewSpec(yamlSpec)
	assert.Nil(err)

	before := &Pipeline{}
	before.Init(spec, nil)
	defer before.Close()

	ctx = context.New(tracing.NoopSpan)
	ctx.SetRequest(context.DefaultNamespace, req)
	pipeline.HandleWithBeforeAfter(ctx, before, after)
	tags = ctx.Tags()
	assert.Contains(tags, "filter1")
	assert.NotContains(tags, "filter2")
	assert.NotContains(tags, "filter3")
}

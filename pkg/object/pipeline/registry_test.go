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
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/stretchr/testify/assert"
)

// allProtocolFilter can handle http, mqtt, tcp
type allProtocolFilter struct {
	mockFilter
}

var (
	_ HTTPFilter = (*allProtocolFilter)(nil)
	_ MQTTFilter = (*allProtocolFilter)(nil)
	_ TCPFilter  = (*allProtocolFilter)(nil)
)

func (f *allProtocolFilter) Kind() string {
	return "allProtocolFilter"
}

func (f *allProtocolFilter) HandleHTTP(ctx context.HTTPContext) *context.HTTPResult {
	return nil
}

func (f *allProtocolFilter) HandleMQTT(ctx context.MQTTContext) *context.MQTTResult {
	return nil
}

func (f *allProtocolFilter) HandleTCP(ctx context.TCPContext) *context.TCPResult {
	return nil
}

func (f *allProtocolFilter) Results() []string {
	return []string{"protocolError", "contextError"}
}

// errResultFilter return repeat result
type errResultFilter struct {
	mockFilter
}

func (f *errResultFilter) Kind() string      { return "errResultFilter" }
func (f *errResultFilter) Results() []string { return []string{"error", "error"} }

// emptyKindFilter kind is empty
type emptyKindFilter struct {
	mockFilter
}

func (f *emptyKindFilter) Kind() string { return "" }

// noPtrFilter is filter but not a pointer
type nonPtrFilter struct{}

type noPtrSpec struct{}

var _ Filter = nonPtrFilter{}

func (f nonPtrFilter) Kind() string                                              { return "nonPtrFilter" }
func (f nonPtrFilter) DefaultSpec() interface{}                                  { return &noPtrSpec{} }
func (f nonPtrFilter) Description() string                                       { return "nonPtrFilter" }
func (f nonPtrFilter) Results() []string                                         { return nil }
func (f nonPtrFilter) Init(filterSpec *FilterSpec)                               {}
func (f nonPtrFilter) Inherit(filterSpec *FilterSpec, previousGeneration Filter) {}
func (f nonPtrFilter) Status() interface{}                                       { return nil }
func (f nonPtrFilter) Close()                                                    {}
func (f nonPtrFilter) APIs() []*APIEntry                                         { return nil }

// intSpecFilter return int default spec
type intSpecFilter struct {
	mockFilter
}

func (f *intSpecFilter) Kind() string             { return "nilSpecFilter" }
func (f *intSpecFilter) DefaultSpec() interface{} { return 2 }

// nonStructSpecFilter
type nonStructSpecFilter struct {
	mockFilter
}

func (f *nonStructSpecFilter) Kind() string             { return "nonStructSpecFilter" }
func (f *nonStructSpecFilter) DefaultSpec() interface{} { return map[string]string{} }

func TestRegistry(t *testing.T) {
	assert := assert.New(t)

	// test getProtocols
	allFilter := &allProtocolFilter{}
	protocols, err := getProtocols(allFilter)
	assert.Nil(err)
	assert.Equal(protocols, map[context.Protocol]struct{}{context.HTTP: {}, context.MQTT: {}, context.TCP: {}}, "allProtocolFilter support HTTP, MQTT and TCP")

	nonFilter := &mockFilter{}
	_, err = getProtocols(nonFilter)
	assert.NotNil(err, "nonFilter not support any protocol")

	// test GetPipeline
	_, err = GetPipeline("pipeline-no-exist", context.HTTP)
	assert.NotNil(err, "GetPipeline not exist pipeline should fail")

	// test store and delete pipeline
	assert.Panics(func() { deletePipeline("not-exist", context.HTTP) }, "delete not-exist pipeline should panic")
	assert.NotPanics(func() { storePipeline("pipeline-store", context.HTTP, &Pipeline{}) }, "store pipeline should success")
	assert.Panics(func() { storePipeline("pipeline-store", context.HTTP, &Pipeline{}) }, "store exist pipeline should panic")
	assert.NotPanics(func() { storePipeline("pipeline-store", context.MQTT, &Pipeline{}) }, "store same pipeline name but different protocol name should success")
	assert.NotPanics(func() { deletePipeline("pipeline-store", context.HTTP) }, "delete exist pipeline should success")

	Register(&mockFilter{})
	Register(&allProtocolFilter{})
	filters := GetFilterRegistry()
	assert.Contains(filters, nonFilter.Kind())
	assert.Contains(filters, allFilter.Kind())

	// check register
	assert.Panics(func() { Register(&errResultFilter{}) }, "errResultFilter return repeat results, should panic")
	assert.Panics(func() { Register(&emptyKindFilter{}) }, "emptyKindFilter return empty kind, should panic")
	assert.Panics(func() { Register(&mockFilter{}) }, "mockFilter already registered, should panic")
	assert.Panics(func() { Register(nonPtrFilter{}) }, "nonPtrFilter is not a ptr should panic")
	assert.Panics(func() { Register(&intSpecFilter{}) }, "intSpecFilter return int default spec, should panic")
	assert.Panics(func() { Register(&nonStructSpecFilter{}) }, "nonStructSpecFilter return non struct default spec, should panic")
}

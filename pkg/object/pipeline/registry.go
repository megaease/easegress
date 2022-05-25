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
	"sync"

	"github.com/megaease/easegress/pkg/context"
)

type (
	// Filter is the common interface for filters.
	Filter interface {
		// Kind returns the unique kind name to represent itself.
		Kind() string

		// DefaultSpec returns the default spec.
		DefaultSpec() interface{}

		// Description returns the description of the filter.
		Description() string

		// Results returns all possible results, excluding the default result value (empty string).
		Results() []string

		// Init initializes the Filter.
		Init(filterSpec *FilterSpec)

		// Inherit also initializes the Filter.
		// But it needs to handle the lifecycle of the previous generation.
		// So it is Filter's responsibility to inherit and clean the previous generation.
		// The http pipeline won't call Close for the previous generation.
		Inherit(filterSpec *FilterSpec, previousGeneration Filter)

		// Status returns its runtime status.
		// It could return nil.
		Status() interface{}

		// Close closes itself.
		Close()
	}

	// HTTPFilter is the common interface for filters to handle http traffic.
	HTTPFilter interface {
		Filter

		// Handle handles one HTTP request, all possible results
		// need be registered in Results.
		HandleHTTP(context.HTTPContext) *context.HTTPResult
	}

	// MQTTFilter is the common interface for filters to handle mqtt traffic.
	MQTTFilter interface {
		Filter

		// Handle handles one MQTT request, all possible results
		// need be registered in Results.
		HandleMQTT(context.MQTTContext) *context.MQTTResult
	}

	// TCPFilter is the common interface for filters to handle tcp traffic.
	TCPFilter interface {
		Filter

		HandleTCP(context.TCPContext) *context.TCPResult
	}

	// UDPFilter is the common interface for filters to handle udp traffic.
	UDPFilter interface {
		Filter
		HandleUDP(context.UDPContext) *context.UDPResult
	}

	// APIEntry contains filter api information
	APIEntry struct {
		Path    string
		Method  string
		Handler http.HandlerFunc
	}
)

func getProtocols(f Filter) (map[context.Protocol]struct{}, error) {
	ans := map[context.Protocol]struct{}{}
	if _, ok := f.(HTTPFilter); ok {
		ans[context.HTTP] = struct{}{}
	}
	if _, ok := f.(MQTTFilter); ok {
		ans[context.MQTT] = struct{}{}
	}
	if _, ok := f.(TCPFilter); ok {
		ans[context.TCP] = struct{}{}
	}
	if _, ok := f.(UDPFilter); ok {
		ans[context.UDP] = struct{}{}
	}
	if len(ans) == 0 {
		return nil, fmt.Errorf("filter %v protocol not found, currently only support HTTP, MQTT and TCP", f.Kind())
	}
	return ans, nil
}

var (
	runningPipelines = sync.Map{}
	filterRegistry   = map[string]Filter{}
)

func pipelineName(name string, protocol context.Protocol) string {
	return string(protocol) + "/" + name
}

// GetPipeline is used to get pipeline with given name and protocol
func GetPipeline(name string, protocol context.Protocol) (*Pipeline, error) {
	key := pipelineName(name, protocol)
	if value, ok := runningPipelines.Load(key); ok {
		return value.(*Pipeline), nil
	}
	return nil, fmt.Errorf("no running pipeline for %v %v", protocol, name)
}

func storePipeline(name string, protocol context.Protocol, pipeline *Pipeline) {
	_, loaded := runningPipelines.LoadOrStore(pipelineName(name, protocol), pipeline)
	if loaded {
		panic(fmt.Errorf("pipeline %v %v already exists", name, protocol))
	}
}

func deletePipeline(name string, protocol context.Protocol) {
	_, loaded := runningPipelines.LoadAndDelete(pipelineName(name, protocol))
	if !loaded {
		panic(fmt.Errorf("pipeline %v %v not exists", name, protocol))
	}
}

// Register registers filter.
func Register(f Filter) {
	if f.Kind() == "" {
		panic(fmt.Errorf("%T: empty kind", f))
	}

	existedFilter, existed := filterRegistry[f.Kind()]
	if existed {
		panic(fmt.Errorf("%T and %T got same kind: %s", f, existedFilter, f.Kind()))
	}

	// Checking filter type.
	filterType := reflect.TypeOf(f)
	if filterType.Kind() != reflect.Ptr {
		panic(fmt.Errorf("%s: want a pointer, got %s", f.Kind(), filterType.Kind()))
	}
	if filterType.Elem().Kind() != reflect.Struct {
		panic(fmt.Errorf("%s elem: want a struct, got %s", f.Kind(), filterType.Kind()))
	}

	// Checking spec type.
	specType := reflect.TypeOf(f.DefaultSpec())
	if specType.Kind() != reflect.Ptr {
		panic(fmt.Errorf("%s spec: want a pointer, got %s", f.Kind(), specType.Kind()))
	}
	if specType.Elem().Kind() != reflect.Struct {
		panic(fmt.Errorf("%s spec elem: want a struct, got %s", f.Kind(), specType.Elem().Kind()))
	}

	// Checking results.
	results := make(map[string]struct{})
	for _, result := range f.Results() {
		_, exists := results[result]
		if exists {
			panic(fmt.Errorf("repeated result: %s", result))
		}
		results[result] = struct{}{}
	}

	filterRegistry[f.Kind()] = f
}

// GetFilterRegistry get the filter registry.
func GetFilterRegistry() map[string]Filter {
	result := map[string]Filter{}

	for kind, f := range filterRegistry {
		result[kind] = f
	}

	return result
}

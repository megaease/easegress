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
	"strings"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/resilience"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/easemonitor"
	"github.com/megaease/easegress/pkg/util/fasttime"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

const (
	// Category is the category of Pipeline.
	Category = supervisor.CategoryPipeline

	// Kind is the kind of Pipeline.
	Kind = "Pipeline"

	// BuiltInFilterEnd is the name of the build-in end filter.
	BuiltInFilterEnd = "END"
)

func init() {
	supervisor.Register(&Pipeline{})
}

func isBuiltInFilter(name string) bool {
	return name == BuiltInFilterEnd
}

type (
	// Pipeline is Object Pipeline.
	Pipeline struct {
		superSpec *supervisor.Spec
		spec      *Spec

		filters    map[string]filters.Filter
		flow       []FlowNode
		resilience map[string]resilience.Policy
	}

	// Spec describes the Pipeline.
	Spec struct {
		Flow       []FlowNode               `yaml:"flow" jsonschema:"omitempty"`
		Filters    []map[string]interface{} `yaml:"filters" jsonschema:"required"`
		Resilience []map[string]interface{} `yaml:"resilience" jsonschema:"omitempty"`
		Data       map[string]interface{}   `yaml:"data" jsonschema:"omitempty"`
	}

	// FlowNode describes one node of the pipeline flow.
	FlowNode struct {
		FilterName  string            `yaml:"filter" jsonschema:"required,format=urlname"`
		FilterAlias string            `yaml:"alias" jsonschema:"omitempty"`
		Namespace   string            `yaml:"namespace" jsonshema:"omitempty"`
		JumpIf      map[string]string `yaml:"jumpIf" jsonschema:"omitempty"`
		filter      filters.Filter
	}

	// FilterStat records the statistics of a filter.
	FilterStat struct {
		Name     string
		Kind     string
		Result   string
		Duration time.Duration
	}

	// Status is the status of Pipeline.
	Status struct {
		Health  string                 `yaml:"health"`
		Filters map[string]interface{} `yaml:"filters"`
	}
)

func (fn *FlowNode) filterAlias() string {
	if fn.FilterAlias != "" {
		return fn.FilterAlias
	}
	return fn.FilterName
}

// ValidateJumpIf validates whether the target of JumpIfs are valid or not.
func (s *Spec) ValidateJumpIf(specs map[string]filters.Spec) {
	validTargets := map[string]int{BuiltInFilterEnd: 1}
	for i := len(s.Flow) - 1; i >= 0; i-- {
		node := &s.Flow[i]
		if node.FilterName == BuiltInFilterEnd {
			continue
		}
		spec := specs[node.FilterName]
		if spec == nil {
			panic(fmt.Errorf("filter %s not found", node.FilterName))
		}
		results := filters.GetKind(spec.Kind()).Results
		for result, target := range node.JumpIf {
			if !stringtool.StrInSlice(result, results) {
				msgFmt := "filter %s: result %s is not in %v"
				panic(fmt.Errorf(msgFmt, node.FilterName, result, results))
			}
			if count := validTargets[target]; count == 0 {
				msgFmt := "filter %s: target filter %s not found"
				panic(fmt.Errorf(msgFmt, node.FilterName, target))
			} else if count > 1 {
				panic(fmt.Errorf("duplicated filter name/alias: %s", target))
			}
		}
		validTargets[node.filterAlias()]++
	}
}

// Validate validates Spec.
func (s *Spec) Validate() (err error) {
	errPrefix := "filters"
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s: %s", errPrefix, r)
		}
	}()

	specs := map[string]filters.Spec{}

	// 1: validate filter spec
	for _, f := range s.Filters {
		// NOTE: Nil supervisor and pipeline are fine in spec validating phrase.
		spec, err := filters.NewSpec(nil, "", f)
		if err != nil {
			panic(err)
		}

		name := spec.Name()
		if isBuiltInFilter(name) {
			panic(fmt.Errorf("can't use %s(built-in) for filter name", name))
		}
		if _, ok := specs[name]; ok {
			panic(fmt.Errorf("duplicated filter name %s", name))
		}

		specs[name] = spec
	}

	// 2: validate flow
	errPrefix = "flow"
	s.ValidateJumpIf(specs)

	// 3: validate resilience
	for _, r := range s.Resilience {
		_, err := resilience.NewPolicy(r)
		if err != nil {
			panic(err)
		}
	}

	return nil
}

func serializeStats(stats []FilterStat) string {
	if len(stats) == 0 {
		return "pipeline: <empty>"
	}

	var sb strings.Builder
	sb.WriteString("pipeline: ")

	for i := range stats {
		if i > 0 {
			sb.WriteString("->")
		}

		stat := &stats[i]
		sb.WriteString(stat.Name)
		sb.WriteByte('(')
		if stat.Result != "" {
			sb.WriteString(stat.Result)
			sb.WriteByte(',')
		}
		sb.WriteString(stat.Duration.String())
		sb.WriteByte(')')
	}

	return sb.String()
}

// Category returns the category of Pipeline.
func (p *Pipeline) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of Pipeline.
func (p *Pipeline) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of Pipeline.
func (p *Pipeline) DefaultSpec() interface{} {
	return &Spec{}
}

// Init initializes Pipeline.
func (p *Pipeline) Init(superSpec *supervisor.Spec, muxMapper context.MuxMapper) {
	p.superSpec, p.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	p.reload(nil /*no previous generation*/)
}

// Inherit inherits previous generation of Pipeline.
func (p *Pipeline) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object, muxMapper context.MuxMapper) {
	p.superSpec, p.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	p.reload(previousGeneration.(*Pipeline))
	previousGeneration.Close()
}

func (p *Pipeline) reload(previousGeneration *Pipeline) {
	p.filters = make(map[string]filters.Filter)
	p.resilience = make(map[string]resilience.Policy)

	super := p.superSpec.Super()
	pipelineName := p.superSpec.Name()

	// create resilience
	for _, r := range p.spec.Resilience {
		policy, err := resilience.NewPolicy(r)
		if err != nil {
			panic(err)
		}
		p.resilience[policy.Name()] = policy
	}

	// create a flow in case the pipeline spec does not define one.
	flow := p.spec.Flow
	if len(flow) == 0 {
		flow = make([]FlowNode, 0, len(p.spec.Filters))
	}

	for _, rawSpec := range p.spec.Filters {
		// build the filter spec.
		spec, err := filters.NewSpec(super, pipelineName, rawSpec)
		if err != nil {
			panic(err)
		}

		// create filter instance.
		filter := filters.Create(spec)
		if filter == nil {
			panic(fmt.Errorf("kind %s not found", spec.Kind()))
		}

		// init or inherit from previous instance.
		var prev filters.Filter
		if previousGeneration != nil {
			prev = previousGeneration.getFilter(spec.Name())
		}
		if prev == nil {
			filter.Init()
		} else {
			filter.Inherit(prev)
		}
		if r, ok := filter.(filters.Resiliencer); ok {
			r.InjectResiliencePolicy(p.resilience)
		}

		// add the filter to pipeline, and if the pipeline does not define a
		// flow, append it to the flow we just created.
		p.filters[filter.Name()] = filter
		if len(p.spec.Flow) == 0 {
			flow = append(flow, FlowNode{FilterName: spec.Name()})
		}
	}

	p.flow = flow

	// bind filter instance to flow node.
	for i := range flow {
		node := &flow[i]
		if node.FilterName != BuiltInFilterEnd {
			node.filter = p.filters[node.FilterName]
		}
	}
}

func (p *Pipeline) getFilter(name string) filters.Filter {
	return p.filters[name]
}

// HandleWithBeforeAfter handles the request, with additional flow defined by
// the before/after pipeline.
func (p *Pipeline) HandleWithBeforeAfter(ctx *context.Context, before, after *Pipeline) string {
	if len(p.spec.Data) > 0 {
		ctx.SetData("PIPELINE", p.spec.Data)
	}

	result, sawEnd := "", false
	flowLen := len(p.flow)
	if before != nil {
		flowLen += len(before.flow)
	}
	if after != nil {
		flowLen += len(after.flow)
	}
	stats := make([]FilterStat, 0, flowLen)

	if before != nil {
		result, stats, sawEnd = p.doHandle(ctx, before.flow, stats)
	}

	if !sawEnd {
		result, stats, sawEnd = p.doHandle(ctx, p.flow, stats)
	}

	if !sawEnd && after != nil {
		result, stats, sawEnd = p.doHandle(ctx, after.flow, stats)
	}

	ctx.LazyAddTag(func() string {
		return serializeStats(stats)
	})
	return result
}

// Handle is the handler to deal with the request.
func (p *Pipeline) Handle(ctx *context.Context) string {
	if len(p.spec.Data) > 0 {
		ctx.SetData("PIPELINE", p.spec.Data)
	}

	stats := make([]FilterStat, 0, len(p.flow))
	result, stats, _ := p.doHandle(ctx, p.flow, stats)

	ctx.LazyAddTag(func() string {
		return serializeStats(stats)
	})
	return result
}

func (p *Pipeline) doHandle(ctx *context.Context, flow []FlowNode, stats []FilterStat) (string, []FilterStat, bool) {
	result, next, sawEnd := "", "", false

	for i := range flow {
		node := &flow[i]
		alias := node.filterAlias()

		if next != "" && next != alias {
			continue
		}

		if node.FilterName == BuiltInFilterEnd {
			sawEnd = true
			break
		}

		start := fasttime.Now()
		ctx.UseNamespace(node.Namespace)

		result = node.filter.Handle(ctx)
		stats = append(stats, FilterStat{
			Name:     alias,
			Kind:     node.filter.Kind().Name,
			Duration: fasttime.Since(start),
			Result:   result,
		})

		if result == "" {
			next = ""
			continue
		}

		next = node.JumpIf[result]
		if next == "" || next == BuiltInFilterEnd {
			sawEnd = true
			break
		}
	}

	return result, stats, sawEnd
}

// Status returns Status generated by Runtime.
func (p *Pipeline) Status() *supervisor.Status {
	s := &Status{
		Filters: make(map[string]interface{}),
	}

	for name, filter := range p.filters {
		s.Filters[name] = filter.Status()
	}

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes Pipeline.
func (p *Pipeline) Close() {
	for _, filter := range p.filters {
		filter.Close()
	}
}

// ToMetrics implements easemonitor.Metricer.
func (s *Status) ToMetrics(service string) []*easemonitor.Metrics {
	var results []*easemonitor.Metrics

	for name, status := range s.Filters {
		metricer, ok := status.(easemonitor.Metricer)
		if !ok {
			continue
		}

		svc := service + "/" + name
		results = append(results, metricer.ToMetrics(svc)...)
	}

	return results
}

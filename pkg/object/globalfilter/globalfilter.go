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

package globalfilter

import (
	"fmt"
	"sync/atomic"

	"github.com/megaease/easegress/pkg/protocol"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/yamltool"
)

const (
	// Category is the category of GlobalFilter.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of GlobalFilter.
	Kind = "GlobalFilter"
)

type (
	// GlobalFilter is a business controller.
	// It provides handler before and after pipeline in HTTPServer.
	GlobalFilter struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		beforePipeline atomic.Value
		afterPipeline  atomic.Value
	}

	// Spec describes the GlobalFilter.
	Spec struct {
		BeforePipeline httppipeline.Spec `yaml:"beforePipeline" jsonschema:"omitempty"`
		AfterPipeline  httppipeline.Spec `yaml:"afterPipeline" jsonschema:"omitempty"`
	}

	// pipelineSpec defines httppipeline spec to create an httppipeline entity.
	pipelineSpec struct {
		Kind              string `yaml:"kind" jsonschema:"omitempty"`
		Name              string `yaml:"name" jsonschema:"omitempty"`
		httppipeline.Spec `yaml:",inline"`
	}
)

func init() {
	supervisor.Register(&GlobalFilter{})
}

// CreateAndUpdateBeforePipelineForSpec creates beforPipeline if the spec is nil, otherwise it updates by the spec.
func (gf *GlobalFilter) CreateAndUpdateBeforePipelineForSpec(spec *Spec, previousGeneration *httppipeline.HTTPPipeline) error {
	beforePipeline := &pipelineSpec{
		Kind: httppipeline.Kind,
		Name: "before",
		Spec: spec.BeforePipeline,
	}
	pipeline, err := gf.CreateAndUpdatePipeline(beforePipeline, previousGeneration)
	if err != nil {
		return err
	}
	if pipeline == nil {
		return fmt.Errorf("before pipeline is nil, spec: %v", beforePipeline)
	}
	gf.beforePipeline.Store(pipeline)
	return nil
}

// CreateAndUpdateAfterPipelineForSpec creates afterPipeline if the spec is nil, otherwise it updates with the spec.
func (gf *GlobalFilter) CreateAndUpdateAfterPipelineForSpec(spec *Spec, previousGeneration *httppipeline.HTTPPipeline) error {
	afterPipeline := &pipelineSpec{
		Kind: httppipeline.Kind,
		Name: "after",
		Spec: spec.AfterPipeline,
	}
	pipeline, err := gf.CreateAndUpdatePipeline(afterPipeline, previousGeneration)
	if err != nil {
		return err
	}
	if pipeline == nil {
		return fmt.Errorf("after pipeline is nil, spec: %v", afterPipeline)
	}
	gf.afterPipeline.Store(pipeline)
	return nil
}

// CreateAndUpdatePipeline creates and updates GlobalFilter's pipelines.
func (gf *GlobalFilter) CreateAndUpdatePipeline(spec *pipelineSpec, previousGeneration *httppipeline.HTTPPipeline) (*httppipeline.HTTPPipeline, error) {
	// init config
	config := yamltool.Marshal(spec)
	specs, err := supervisor.NewSpec(string(config))
	if err != nil {
		return nil, err
	}

	// init or update pipeline
	pipeline := new(httppipeline.HTTPPipeline)
	if previousGeneration != nil {
		pipeline.Inherit(specs, previousGeneration, nil)
	} else {
		pipeline.Init(specs, nil)
	}
	return pipeline, nil
}

// Category returns the object category of itself.
func (gf *GlobalFilter) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the unique kind name to represent itself.
func (gf *GlobalFilter) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec.
// It must return a pointer to point a struct.
func (gf *GlobalFilter) DefaultSpec() interface{} {
	return &Spec{}
}

// Status returns its runtime status.
func (gf *GlobalFilter) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: struct{}{},
	}
}

// Init initializes GlobalFilter.
func (gf *GlobalFilter) Init(superSpec *supervisor.Spec) {
	gf.superSpec, gf.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	gf.reload(nil)
}

// Inherit inherits previous generation of GlobalFilter.
func (gf *GlobalFilter) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	gf.superSpec, gf.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	gf.reload(previousGeneration.(*GlobalFilter))
}

// Handle `beforePipeline` and `afterPipeline` before and after the httpHandler is executed.
func (gf *GlobalFilter) Handle(ctx context.HTTPContext, httpHandle protocol.HTTPHandler) {
	result := gf.beforeHandle(ctx)
	if result == httppipeline.LabelEND {
		return
	}
	result = httpHandle.Handle(ctx)
	if result == httppipeline.LabelEND {
		return
	}
	gf.afterHandle(ctx)
	return
}

// BeforeHandle before handler logic for beforePipeline spec.
func (gf *GlobalFilter) beforeHandle(ctx context.HTTPContext) string {
	value := gf.beforePipeline.Load()
	if value == nil {
		return ""
	}
	handler, ok := value.(*httppipeline.HTTPPipeline)
	if !ok {
		return ""
	}
	return handler.Handle(ctx)
}

// AfterHandle after handler logic for afterPipeline spec.
func (gf *GlobalFilter) afterHandle(ctx context.HTTPContext) string {
	value := gf.afterPipeline.Load()
	if value == nil {
		return ""
	}
	handler, ok := value.(*httppipeline.HTTPPipeline)
	if !ok {
		return ""
	}
	return handler.Handle(ctx)
}

// Close closes GlobalFilter itself.
func (gf *GlobalFilter) Close() {
}

// Validate validates Spec.
func (s *Spec) Validate() (err error) {
	err = s.BeforePipeline.Validate()
	if err != nil {
		return fmt.Errorf("before pipeline is invalid: %v", err)
	}
	err = s.AfterPipeline.Validate()
	if err != nil {
		return fmt.Errorf("after pipeline is invalid: %v", err)
	}

	return nil
}

func (gf *GlobalFilter) reload(previousGeneration *GlobalFilter) {
	var beforePreviousPipeline, afterPreviousPipeline *httppipeline.HTTPPipeline
	// create and update beforePipeline entity
	if len(gf.spec.BeforePipeline.Flow) != 0 {
		if previousGeneration != nil {
			previous := previousGeneration.beforePipeline.Load()
			if previous != nil {
				beforePreviousPipeline = previous.(*httppipeline.HTTPPipeline)
			}
		}
		err := gf.CreateAndUpdateBeforePipelineForSpec(gf.spec, beforePreviousPipeline)
		if err != nil {
			panic(fmt.Errorf("create before pipeline failed: %v", err))
		}
	}
	// create and update afterPipeline entity
	if len(gf.spec.AfterPipeline.Flow) != 0 {
		if previousGeneration != nil {
			previous := previousGeneration.afterPipeline.Load()
			if previous != nil {
				afterPreviousPipeline = previous.(*httppipeline.HTTPPipeline)
			}
		}
		err := gf.CreateAndUpdateAfterPipelineForSpec(gf.spec, afterPreviousPipeline)
		if err != nil {
			panic(fmt.Errorf("create after pipeline failed: %v", err))
		}
	}
}

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

package trafficcontroller

import (
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// Category is the category of TrafficController.
	Category = supervisor.CategorySystemController

	// Kind is the kind of TrafficController.
	Kind = "TrafficController"
)

type (
	// TrafficController is a system controller to manage
	// TrafficGate, Pipeline and their relationship.
	TrafficController struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		// Pointer aims to safely transform it to next generation.
		mutex      *sync.Mutex
		namespaces map[string]*Namespace
	}

	// Namespace is the namespace
	Namespace struct {
		namespace string
		// The scenario here satisfies the first common case:
		// When the entry for a given key is only ever written once but read many times.
		// Reference: https://golang.org/pkg/sync/#Map
		// types of both: map[string]*supervisor.ObjectEntity
		trafficGates sync.Map
		pipelines    sync.Map
	}

	// WalkFunc is the type of the function called for
	// walking traffic gate and pipeline.
	WalkFunc = supervisor.WalkFunc

	// Spec describes TrafficController.
	Spec struct{}

	// Status is the status of namespaces
	Status struct {
		Namespaces []*NamespacesStatus `yaml:"namespaces"`
	}

	// NamespacesStatus is the universal status in one namespace.
	NamespacesStatus struct {
		Namespace      string           `yaml:"namespace"`
		TrafficObjects []*TrafficObject `yaml:"trafficObjects"`
	}

	// TrafficObject is the traffic object.
	TrafficObject struct {
		Name                string `yaml:"name"`
		TrafficObjectStatus `yaml:",inline"`
	}

	// TrafficObjectStatus is the status of traffic object.
	TrafficObjectStatus struct {
		Spec   interface{} `yaml:"spec"`
		Status interface{} `yaml:"status"`
	}
)

func init() {
	supervisor.Register(&TrafficController{})
}

// TrafficNamespace returns the exported system namespace of the internal namespace of TrafficController.
func (ns *NamespacesStatus) TrafficNamespace() string {
	return cluster.TrafficNamespace(ns.Namespace)
}

func newNamespace(namespace string) *Namespace {
	return &Namespace{
		namespace: namespace,
	}
}

// GetHandler gets handler within the namespace
func (ns *Namespace) GetHandler(name string) (context.Handler, bool) {
	entity, exists := ns.pipelines.Load(name)
	if !exists {
		return nil, false
	}

	handler := entity.(*supervisor.ObjectEntity).Instance().(context.Handler)
	return handler, true
}

// Category returns the category of TrafficController.
func (tc *TrafficController) Category() supervisor.ObjectCategory {
	return Category
}

// Kind return the kind of TrafficController.
func (tc *TrafficController) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of TrafficController.
func (tc *TrafficController) DefaultSpec() interface{} {
	return &Spec{}
}

// Init initializes TrafficController.
func (tc *TrafficController) Init(superSpec *supervisor.Spec) {
	tc.superSpec, tc.spec, tc.super = superSpec, superSpec.ObjectSpec().(*Spec), superSpec.Super()

	tc.mutex = &sync.Mutex{}
	tc.namespaces = make(map[string]*Namespace)

	tc.reload(nil)
}

// Inherit inherits previous generation of TrafficController.
func (tc *TrafficController) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	tc.superSpec, tc.super = superSpec, superSpec.Super()
	tc.reload(previousGeneration.(*TrafficController))
}

func (tc *TrafficController) reload(previousGeneration *TrafficController) {
	if previousGeneration != nil {
		tc.mutex, tc.namespaces = previousGeneration.mutex, previousGeneration.namespaces
	}
}

// CreateTrafficGateForSpec creates traffic gate with a spec
func (tc *TrafficController) CreateTrafficGateForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error,
) {
	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.CreateTrafficGate(namespace, entity)
}

// CreateTrafficGate creates traffic gate
func (tc *TrafficController) CreateTrafficGate(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error,
) {
	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		space = newNamespace(namespace)
		tc.namespaces[namespace] = space
		logger.Infof("create namespace %s", namespace)
	}

	name := entity.Spec().Name()

	entity.InitWithRecovery(space)
	space.trafficGates.Store(name, entity)

	logger.Infof("create traffic gate %s/%s", namespace, name)

	return entity, nil
}

// UpdateTrafficGateForSpec updates traffic gate with a Spec
func (tc *TrafficController) UpdateTrafficGateForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error,
) {
	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.UpdateTrafficGate(namespace, entity)
}

// UpdateTrafficGate updates traffic gate
func (tc *TrafficController) UpdateTrafficGate(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error,
) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		return nil, fmt.Errorf("namespace %s not found", namespace)
	}

	name := entity.Spec().Name()

	previousEntity, exists := space.trafficGates.Load(name)
	if !exists {
		return nil, fmt.Errorf("traffic gate %s/%s not found", namespace, name)
	}

	entity.InheritWithRecovery(previousEntity.(*supervisor.ObjectEntity), space)
	space.trafficGates.Store(name, entity)

	logger.Infof("update traffic gate %s/%s", namespace, name)

	return entity, nil
}

// ApplyTrafficGateForSpec applies traffic gate with a Spec
func (tc *TrafficController) ApplyTrafficGateForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error,
) {
	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.ApplyTrafficGate(namespace, entity)
}

// ApplyTrafficGate applies traffic gate
func (tc *TrafficController) ApplyTrafficGate(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error,
) {
	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		space = newNamespace(namespace)
		tc.namespaces[namespace] = space
		logger.Infof("create namespace %s", namespace)
	}

	name := entity.Spec().Name()

	previousEntity, exists := space.trafficGates.Load(name)
	if !exists {
		entity.InitWithRecovery(space)
		space.trafficGates.Store(name, entity)

		logger.Infof("create traffic gate %s/%s", namespace, name)
	} else {
		prev := previousEntity.(*supervisor.ObjectEntity)
		if prev.Spec().Equals(entity.Spec()) {
			logger.Infof("traffic gate %s/%s nothing change", namespace, name)
			return prev, nil
		}

		entity.InheritWithRecovery(previousEntity.(*supervisor.ObjectEntity), space)
		space.trafficGates.Store(name, entity)

		logger.Infof("update traffic gate %s/%s", namespace, name)
	}

	return entity, nil
}

// DeleteTrafficGate deletes a traffic gate
func (tc *TrafficController) DeleteTrafficGate(namespace, name string) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		return fmt.Errorf("namespace %s not found", namespace)
	}

	entity, exists := space.trafficGates.LoadAndDelete(name)
	if !exists {
		return fmt.Errorf("traffic gate %s/%s not found", namespace, name)
	}

	entity.(*supervisor.ObjectEntity).CloseWithRecovery()
	logger.Infof("delete traffic gate %s/%s", namespace, name)

	tc._cleanSpace(namespace)

	return nil
}

// GetTrafficGate gets a traffic gate by its namespace and name
func (tc *TrafficController) GetTrafficGate(namespace, name string) (*supervisor.ObjectEntity, bool) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		return nil, false
	}

	entity, exists := space.trafficGates.Load(name)
	if !exists {
		return nil, false
	}

	return entity.(*supervisor.ObjectEntity), exists
}

// ListTrafficGates lists the traffic gates
func (tc *TrafficController) ListTrafficGates(namespace string) []*supervisor.ObjectEntity {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		return nil
	}

	entities := []*supervisor.ObjectEntity{}
	space.trafficGates.Range(func(k, v interface{}) bool {
		entities = append(entities, v.(*supervisor.ObjectEntity))
		return true
	})

	return entities
}

// WalkTrafficGates walks traffic gates
func (tc *TrafficController) WalkTrafficGates(namespace string, walkFn WalkFunc) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("WalkTrafficGates recover from err: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}
	}()

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		return
	}

	space.trafficGates.Range(func(k, v interface{}) bool {
		return walkFn(v.(*supervisor.ObjectEntity))
	})
}

// WalkPipelines walks the pipelines
func (tc *TrafficController) WalkPipelines(namespace string, walkFn WalkFunc) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("WalkPipelines recover from err: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}
	}()

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		return
	}

	space.pipelines.Range(func(k, v interface{}) bool {
		return walkFn(v.(*supervisor.ObjectEntity))
	})
}

// CreatePipelineForSpec creates a pipeline by a spec
func (tc *TrafficController) CreatePipelineForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error,
) {
	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.CreatePipeline(namespace, entity)
}

// CreatePipeline creates a pipeline
func (tc *TrafficController) CreatePipeline(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error,
) {
	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		space = newNamespace(namespace)
		tc.namespaces[namespace] = space
		logger.Infof("create namespace %s", namespace)
	}

	name := entity.Spec().Name()

	entity.InitWithRecovery(space)
	space.pipelines.Store(name, entity)

	logger.Infof("create pipeline %s/%s", namespace, name)

	return entity, nil
}

// UpdatePipelineForSpec updates the pipeline with a Spec
func (tc *TrafficController) UpdatePipelineForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error,
) {
	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.UpdatePipeline(namespace, entity)
}

// UpdatePipeline updates the pipeline
func (tc *TrafficController) UpdatePipeline(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error,
) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		return nil, fmt.Errorf("namespace %s not found", namespace)
	}

	name := entity.Spec().Name()

	previousEntity, exists := space.pipelines.Load(name)
	if !exists {
		return nil, fmt.Errorf("pipeline %s/%s not found", namespace, name)
	}

	entity.InheritWithRecovery(previousEntity.(*supervisor.ObjectEntity), space)
	space.pipelines.Store(name, entity)

	logger.Infof("update pipeline %s/%s", namespace, name)

	return entity, nil
}

// ApplyPipelineForSpec applies the pipeline with a Spec
func (tc *TrafficController) ApplyPipelineForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error,
) {
	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.ApplyPipeline(namespace, entity)
}

// ApplyPipeline applies the pipeline
func (tc *TrafficController) ApplyPipeline(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error,
) {
	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		space = newNamespace(namespace)
		tc.namespaces[namespace] = space
		logger.Infof("create namespace %s", namespace)
	}

	name := entity.Spec().Name()

	previousEntity, exists := space.pipelines.Load(name)
	if !exists {
		entity.InitWithRecovery(space)
		space.pipelines.Store(name, entity)

		logger.Infof("create pipeline %s/%s", namespace, name)
	} else {
		prev := previousEntity.(*supervisor.ObjectEntity)
		if prev.Spec().Equals(entity.Spec()) {
			logger.Infof("pipeline %s/%s nothing change", namespace, name)
			return prev, nil
		}

		entity.InheritWithRecovery(prev, space)
		space.pipelines.Store(name, entity)

		logger.Infof("update pipeline %s/%s", namespace, name)
	}

	return entity, nil
}

// DeletePipeline deletes the pipeline by its namespace and name
func (tc *TrafficController) DeletePipeline(namespace, name string) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		return fmt.Errorf("namespace %s not found", namespace)
	}

	entity, exists := space.pipelines.LoadAndDelete(name)
	if !exists {
		return fmt.Errorf("pipeline %s/%s not found", namespace, name)
	}

	entity.(*supervisor.ObjectEntity).CloseWithRecovery()
	logger.Infof("delete pipeline %s/%s", namespace, name)

	tc._cleanSpace(namespace)

	return nil
}

// GetPipeline returns the pipeline by its namespace and name.
func (tc *TrafficController) GetPipeline(namespace, name string) (*supervisor.ObjectEntity, bool) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		return nil, false
	}

	entity, exists := space.pipelines.Load(name)
	if !exists {
		return nil, false
	}

	return entity.(*supervisor.ObjectEntity), exists
}

// ListPipelines lists the pipelines
func (tc *TrafficController) ListPipelines(namespace string) []*supervisor.ObjectEntity {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		return nil
	}

	entities := []*supervisor.ObjectEntity{}
	space.pipelines.Range(func(k, v interface{}) bool {
		entities = append(entities, v.(*supervisor.ObjectEntity))
		return true
	})

	return entities
}

// Clean all traffic gates and pipelines of one namespace.
func (tc *TrafficController) Clean(namespace string) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exist := tc.namespaces[namespace]
	if !exist {
		return fmt.Errorf("namespace %s not found", namespace)
	}

	space.trafficGates.Range(func(k, v interface{}) bool {
		v.(*supervisor.ObjectEntity).CloseWithRecovery()
		logger.Infof("delete traffic gate %s/%s", namespace, k)
		space.trafficGates.Delete(k)
		return true
	})

	space.pipelines.Range(func(k, v interface{}) bool {
		v.(*supervisor.ObjectEntity).CloseWithRecovery()
		logger.Infof("delete pipeline %s/%s", namespace, k)
		space.pipelines.Delete(k)
		return true
	})

	tc._cleanSpace(namespace)

	return nil
}

// _cleanSpace must be called after deleting traffic gate or Pipeline.
// It's caller's duty to keep concurrent safety.
func (tc *TrafficController) _cleanSpace(namespace string) {
	space, exists := tc.namespaces[namespace]
	if !exists {
		logger.Errorf("BUG: namespace %s not found", namespace)
	}

	serverLen, pipelineLen := 0, 0
	space.trafficGates.Range(func(k, v interface{}) bool {
		serverLen++
		return false
	})
	space.pipelines.Range(func(k, v interface{}) bool {
		pipelineLen++
		return false
	})
	if serverLen+pipelineLen == 0 {
		delete(tc.namespaces, namespace)
		logger.Infof("delete namespace %s", namespace)
	}
}

// Status returns the status of TrafficController.
// It reports all traffic object spec and status.
func (tc *TrafficController) Status() *supervisor.Status {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	statuses := []*NamespacesStatus{}

	for namespace, namespaceSpec := range tc.namespaces {
		status := &NamespacesStatus{
			Namespace: namespace,
		}

		recordStatus := func(key, value interface{}) bool {
			k := key.(string)
			v := value.(*supervisor.ObjectEntity)

			trafficObject := &TrafficObject{
				Name: k,
				TrafficObjectStatus: TrafficObjectStatus{
					Spec:   v.Spec().ObjectSpec(),
					Status: v.Instance().Status().ObjectStatus,
				},
			}

			status.TrafficObjects = append(status.TrafficObjects, trafficObject)

			return true
		}

		namespaceSpec.trafficGates.Range(recordStatus)
		namespaceSpec.pipelines.Range(recordStatus)

		statuses = append(statuses, status)
	}

	return &supervisor.Status{ObjectStatus: &Status{Namespaces: statuses}}
}

// Close closes TrafficController.
func (tc *TrafficController) Close() {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	for name, space := range tc.namespaces {
		space.trafficGates.Range(func(k, v interface{}) bool {
			entity := v.(*supervisor.ObjectEntity)
			entity.CloseWithRecovery()
			logger.Infof("delete traffic gate %s/%s", space.namespace, k)
			return true
		})

		space.pipelines.Range(func(k, v interface{}) bool {
			entity := v.(*supervisor.ObjectEntity)
			entity.CloseWithRecovery()
			logger.Infof("delete pipeline %s/%s", space.namespace, k)
			return true
		})

		delete(tc.namespaces, name)
		logger.Infof("delete namespace %s", name)
	}
}

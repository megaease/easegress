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

package supervisor

import (
	"fmt"
	"os"
	"runtime/debug"
	"sync"

	"github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/option"
)

const watcherName = "__SUPERVISOR__"

type (
	// Supervisor manages all objects.
	Supervisor struct {
		options *option.Options
		cls     cluster.Cluster

		// The scenario here satisfies the first common case:
		// When the entry for a given key is only ever written once but read many times.
		// Reference: https://golang.org/pkg/sync/#Map
		businessControllers sync.Map
		systemControllers   sync.Map

		objectRegistry  *ObjectRegistry
		watcher         *ObjectEntityWatcher
		firstHandle     bool
		firstHandleDone chan struct{}
		done            chan struct{}
	}

	// WalkFunc is the type of the function called for
	// walking object entity.
	WalkFunc func(entity *ObjectEntity) bool
)

var globalSuper *Supervisor

func loadInitialObjects(s *Supervisor, paths []string) map[string]string {
	objs := map[string]string{}
	for _, path := range paths {
		data, e := os.ReadFile(path)
		if e != nil {
			logger.Errorf("failed to load initial object, path: %s, error: %v", path, e)
			continue
		}
		spec, e := s.NewSpec(string(data))
		if e != nil {
			logger.Errorf("failed to create spec for initial object, path: %s, error: %v", path, e)
			continue
		}
		objs[spec.Name()] = spec.YAMLConfig()
	}
	return objs
}

// MustNew creates a Supervisor.
func MustNew(opt *option.Options, cls cluster.Cluster) *Supervisor {
	s := &Supervisor{
		options: opt,
		cls:     cls,

		firstHandle:     true,
		firstHandleDone: make(chan struct{}),
		done:            make(chan struct{}),
	}

	initObjs := loadInitialObjects(s, opt.InitialObjectConfigFiles)

	s.objectRegistry = newObjectRegistry(s, initObjs)
	s.watcher = s.objectRegistry.NewWatcher(watcherName, FilterCategory(
		// NOTE: SystemController is only initialized internally.
		// CategorySystemController,
		CategoryBusinessController))

	globalSuper = s

	s.initSystemControllers()

	go s.run()

	return s
}

// Options returns the options applied to supervisor.
func (s *Supervisor) Options() *option.Options {
	return s.options
}

// Cluster return the cluster applied to supervisor.
func (s *Supervisor) Cluster() cluster.Cluster {
	return s.cls
}

func (s *Supervisor) initSystemControllers() {
	for _, rootObject := range objectRegistryOrderByDependency {
		kind := rootObject.Kind()

		if rootObject.Category() != CategorySystemController {
			continue
		}

		meta := &MetaSpec{
			// NOTE: Use kind to be the name since the system controller is unique.
			Name: kind,
			Kind: kind,
		}

		spec := s.newSpecInternal(meta, rootObject.DefaultSpec())
		entity, err := s.NewObjectEntityFromSpec(spec)
		if err != nil {
			panic(err)
		}

		logger.Infof("create %s", spec.Name())

		entity.InitWithRecovery(nil /* muxMapper */)
		s.systemControllers.Store(kind, entity)
	}
}

func (s *Supervisor) run() {
	for {
		select {
		case <-s.done:
			s.close()
			return
		case event := <-s.watcher.Watch():
			s.handleEvent(event)
		}
	}
}

func (s *Supervisor) handleEvent(event *ObjectEntityWatcherEvent) {
	if s.firstHandle {
		defer func() {
			s.firstHandle = false
			close(s.firstHandleDone)
		}()
	}

	for name := range event.Delete {
		entity, exists := s.businessControllers.LoadAndDelete(name)
		if !exists {
			logger.Errorf("BUG: delete %s not found", name)
			continue
		}

		logger.Infof("delete %s", name)
		entity.(*ObjectEntity).CloseWithRecovery()
	}

	for name, entity := range event.Create {
		_, exists := s.businessControllers.Load(name)
		if exists {
			logger.Errorf("BUG: create %s already existed", name)
			continue
		}

		logger.Infof("create %s", name)
		entity.InitWithRecovery(nil /* muxMapper */)
		s.businessControllers.Store(name, entity)
	}

	for name, entity := range event.Update {
		previousEntity, exists := s.businessControllers.Load(name)
		if !exists {
			logger.Errorf("BUG: update %s not found", name)
			continue
		}

		logger.Infof("update %s", name)
		entity.InheritWithRecovery(previousEntity.(*ObjectEntity), nil /* muxMapper */)
		s.businessControllers.Store(name, entity)
	}
}

// ObjectRegistry returns the registry of object
func (s *Supervisor) ObjectRegistry() *ObjectRegistry {
	return s.objectRegistry
}

// WalkControllers walks every controllers until walkFn returns false.
func (s *Supervisor) WalkControllers(walkFn WalkFunc) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("walkControllers recover from err: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}
	}()

	s.systemControllers.Range(func(k, v interface{}) bool {
		return walkFn(v.(*ObjectEntity))
	})

	s.businessControllers.Range(func(k, v interface{}) bool {
		return walkFn(v.(*ObjectEntity))
	})
}

// MustGetSystemController wraps GetSystemController with panic.
func (s *Supervisor) MustGetSystemController(name string) *ObjectEntity {
	entity, exists := s.GetSystemController(name)
	if !exists {
		panic(fmt.Errorf("system controller %s not found", name))
	}

	return entity
}

// GetSystemController returns the system controller with the existing flag.
// The name of system controller is its own kind.
func (s *Supervisor) GetSystemController(name string) (*ObjectEntity, bool) {
	entity, exists := s.systemControllers.Load(name)
	if !exists {
		return nil, false
	}
	return entity.(*ObjectEntity), true
}

// GetBusinessController returns the business controller with the existing flag.
func (s *Supervisor) GetBusinessController(name string) (*ObjectEntity, bool) {
	entity, exists := s.businessControllers.Load(name)
	if !exists {
		return nil, false
	}
	return entity.(*ObjectEntity), true
}

// FirstHandleDone returns the firstHandleDone channel,
// which will be closed after creating all objects at first time.
func (s *Supervisor) FirstHandleDone() chan struct{} {
	return s.firstHandleDone
}

// Close closes Supervisor.
func (s *Supervisor) Close(wg *sync.WaitGroup) {
	defer wg.Done()
	s.done <- struct{}{}
	<-s.done
}

func (s *Supervisor) close() {
	s.objectRegistry.CloseWatcher(watcherName)
	s.objectRegistry.close()

	s.businessControllers.Range(func(k, v interface{}) bool {
		entity := v.(*ObjectEntity)
		logger.Infof("delete %s", k)
		entity.CloseWithRecovery()
		return true
	})

	for i := len(objectRegistryOrderByDependency) - 1; i >= 0; i-- {
		rootObject := objectRegistryOrderByDependency[i]
		if rootObject.Category() != CategorySystemController {
			continue
		}

		kind := rootObject.Kind()
		value, exists := s.systemControllers.LoadAndDelete(kind)
		if !exists {
			logger.Errorf("BUG: system controller %s not found", kind)
			continue
		}

		logger.Infof("delete %s", kind)
		value.(*ObjectEntity).CloseWithRecovery()
	}

	close(s.done)
}

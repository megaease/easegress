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

package httpserver

import (
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// Category is the category of HTTPServer.
	Category = supervisor.CategoryTrafficGate

	// Kind is the kind of HTTPServer.
	Kind = "HTTPServer"
)

func init() {
	supervisor.Register(&HTTPServer{})
}

type (
	// HTTPServer is Object HTTPServer.
	HTTPServer struct {
		runtime *runtime
	}
)

// Category returns the category of HTTPServer.
func (hs *HTTPServer) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of HTTPServer.
func (hs *HTTPServer) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of HTTPServer.
func (hs *HTTPServer) DefaultSpec() interface{} {
	return &Spec{
		KeepAlive:        true,
		KeepAliveTimeout: "60s",
		MaxConnections:   10240,
	}
}

// Init initializes HTTPServer.
func (hs *HTTPServer) Init(superSpec *supervisor.Spec, muxMapper protocol.MuxMapper) {
	hs.runtime = newRuntime(superSpec, muxMapper)

	hs.runtime.eventChan <- &eventReload{
		nextSuperSpec: superSpec,
		muxMapper:     muxMapper,
	}
}

// Inherit inherits previous generation of HTTPServer.
func (hs *HTTPServer) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object, muxMapper protocol.MuxMapper) {
	hs.runtime = previousGeneration.(*HTTPServer).runtime

	hs.runtime.eventChan <- &eventReload{
		nextSuperSpec: superSpec,
		muxMapper:     muxMapper,
	}
}

// Status is the wrapper of runtime's Status.
func (hs *HTTPServer) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: hs.runtime.Status(),
	}
}

// Close closes HTTPServer.
func (hs *HTTPServer) Close() {
	hs.runtime.Close()
}

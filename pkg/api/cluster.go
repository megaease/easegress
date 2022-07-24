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

package api

import (
	"fmt"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/supervisor"
)

func (s *Server) _purgeMember(memberName string) {
	err := s.cluster.PurgeMember(memberName)
	if err != nil {
		ClusterPanic(fmt.Errorf("purge member %s failed: %s", memberName, err))
	}
}

func (s *Server) _getVersion() int64 {
	value, err := s.cluster.Get(s.cluster.Layout().ConfigVersion())
	if err != nil {
		ClusterPanic(err)
	}

	if value == nil {
		return 0
	}

	version, err := strconv.ParseInt(*value, 10, 64)
	if err != nil {
		panic(fmt.Errorf("parse version %s to int failed: %v", *value, err))
	}

	return version
}

func (s *Server) _plusOneVersion() int64 {
	version := s._getVersion()
	version++
	value := fmt.Sprintf("%d", version)

	err := s.cluster.Put(s.cluster.Layout().ConfigVersion(), value)
	if err != nil {
		ClusterPanic(err)
	}

	return version
}

func (s *Server) _getObject(name string) *supervisor.Spec {
	value, err := s.cluster.Get(s.cluster.Layout().ConfigObjectKey(name))
	if err != nil {
		ClusterPanic(err)
	}

	if value == nil {
		return nil
	}

	spec, err := s.super.NewSpec(*value)
	if err != nil {
		panic(fmt.Errorf("bad spec(err: %v) from yaml: %s", err, *value))
	}

	return spec
}

func (s *Server) _listObjects() []*supervisor.Spec {
	kvs, err := s.cluster.GetPrefix(s.cluster.Layout().ConfigObjectPrefix())
	if err != nil {
		ClusterPanic(err)
	}

	specs := make([]*supervisor.Spec, 0, len(kvs))
	for _, v := range kvs {
		spec, err := s.super.NewSpec(v)
		if err != nil {
			panic(fmt.Errorf("bad spec(err: %v) from yaml: %s", err, v))
		}
		specs = append(specs, spec)
	}

	return specs
}

func (s *Server) _putObject(spec *supervisor.Spec) {
	err := s.cluster.Put(s.cluster.Layout().ConfigObjectKey(spec.Name()),
		spec.YAMLConfig())
	if err != nil {
		ClusterPanic(err)
	}
}

func (s *Server) _deleteObject(name string) {
	err := s.cluster.Delete(s.cluster.Layout().ConfigObjectKey(name))
	if err != nil {
		ClusterPanic(err)
	}
}

func (s *Server) _getStatusObject(name string) map[string]string {
	prefix := s.cluster.Layout().StatusObjectPrefix(name)
	kvs, err := s.cluster.GetPrefix(prefix)
	if err != nil {
		ClusterPanic(err)
	}

	status := make(map[string]string)
	for k, v := range kvs {
		// NOTE: Here omitting the step yaml.Unmarshal in _listStatusObjects.
		status[strings.TrimPrefix(k, prefix)] = v
	}

	return status
}

func (s *Server) _listStatusObjects() map[string]interface{} {
	prefix := s.cluster.Layout().StatusObjectsPrefix()
	kvs, err := s.cluster.GetPrefix(prefix)
	if err != nil {
		ClusterPanic(err)
	}

	status := make(map[string]interface{})
	for k, v := range kvs {
		k = strings.TrimPrefix(k, prefix)

		// NOTE: This needs top-level of the status to be a map.
		m := map[string]interface{}{}
		err = yaml.Unmarshal([]byte(v), &m)
		if err != nil {
			ClusterPanic(fmt.Errorf("unmarshal %s to yaml failed: %v", v, err))
		}
		status[k] = m
	}

	return status
}

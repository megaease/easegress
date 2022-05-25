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
	"net/http"
	"reflect"
	"sort"

	"github.com/go-chi/chi/v5"
	yaml "gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/v"
)

const (
	// MetadataPrefix is the metadata prefix.
	MetadataPrefix = "/metadata"

	// ObjectMetadataPrefix is the object metadata prefix.
	ObjectMetadataPrefix = "/metadata/objects"

	// FilterMetaPrefix is the filter of HTTPPipeline metadata prefix.
	FilterMetaPrefix = "/metadata/objects/httppipeline/filters"
)

type (
	// FilterMeta is the metadata of filter.
	FilterMeta struct {
		Kind        string
		Results     []string
		SpecType    reflect.Type
		Description string
	}
)

var (
	filterMetaBook = map[string]*FilterMeta{}
	filterKinds    []string
)

func (s *Server) initMetadata() {
	filterRegistry := httppipeline.GetFilterRegistry()
	for kind, f := range filterRegistry {
		filterMetaBook[kind] = &FilterMeta{
			Kind:        kind,
			Results:     f.Results(),
			SpecType:    reflect.TypeOf(f.DefaultSpec()),
			Description: f.Description(),
		}
		filterKinds = append(filterKinds, kind)
		sort.Strings(filterMetaBook[kind].Results)
	}
	sort.Strings(filterKinds)
}

func (s *Server) metadataAPIEntries() []*Entry {
	return []*Entry{
		{
			Path:    FilterMetaPrefix,
			Method:  "GET",
			Handler: s.listFilters,
		},
		{
			Path:    FilterMetaPrefix + "/{kind}" + "/description",
			Method:  "GET",
			Handler: s.getFilterDescription,
		},
		{
			Path:    FilterMetaPrefix + "/{kind}" + "/schema",
			Method:  "GET",
			Handler: s.getFilterSchema,
		},
		{
			Path:    FilterMetaPrefix + "/{kind}" + "/results",
			Method:  "GET",
			Handler: s.getFilterResults,
		},
	}
}

func (s *Server) listFilters(w http.ResponseWriter, r *http.Request) {
	buff, err := yaml.Marshal(filterKinds)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", filterKinds, err))
	}

	w.Header().Set("Content-Type", "text/vnd.yaml")
	w.Write(buff)
}

func (s *Server) getFilterDescription(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")

	fm, exits := filterMetaBook[kind]
	if !exits {
		HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("not found"))
		return
	}
	w.Write([]byte(fm.Description))
}

func (s *Server) getFilterSchema(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")

	fm, exits := filterMetaBook[kind]
	if !exits {
		HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	buff, err := v.GetSchemaInYAML(fm.SpecType)
	if err != nil {
		panic(fmt.Errorf("get schema for %v failed: %v", fm.Kind, err))
	}

	w.Header().Set("Content-Type", "text/vnd.yaml")
	w.Write(buff)
}

func (s *Server) getFilterResults(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")

	fm, exits := filterMetaBook[kind]
	if !exits {
		HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("not found"))
		return
	}

	buff, err := yaml.Marshal(fm.Results)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to yaml failed: %v", fm.Results, err))
	}

	w.Header().Set("Content-Type", "text/vnd.yaml")
	w.Write(buff)
}

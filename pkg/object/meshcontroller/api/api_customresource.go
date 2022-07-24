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
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easemesh-api/v2alpha1"
	"github.com/xeipuuv/gojsonschema"
)

func (a *API) readURLParam(r *http.Request, name string) (string, error) {
	value := chi.URLParam(r, name)
	if value == "" {
		return "", fmt.Errorf("required URL parameter %s is empty", name)
	}
	return value, nil
}

func (a *API) listCustomResourceKinds(w http.ResponseWriter, r *http.Request) {
	kinds := a.service.ListCustomResourceKinds()
	sort.Slice(kinds, func(i, j int) bool {
		return kinds[i].Name < kinds[j].Name
	})

	var pbKinds []*v2alpha1.CustomResourceKind
	for _, v := range kinds {
		kind := &v2alpha1.CustomResourceKind{}
		err := a.convertSpecToPB(v, kind)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		pbKinds = append(pbKinds, kind)
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(pbKinds)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", kinds, err))
	}
}

func (a *API) getCustomResourceKind(w http.ResponseWriter, r *http.Request) {
	name, err := a.readURLParam(r, "name")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	kind := a.service.GetCustomResourceKind(name)
	if kind == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", name))
		return
	}

	pbKind := &v2alpha1.CustomResourceKind{}
	err = a.convertSpecToPB(kind, pbKind)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", kind, err))
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(pbKind)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", pbKind, err))
	}
}

func (a *API) saveCustomResourceKind(w http.ResponseWriter, r *http.Request, update bool) error {
	pbKind := &v2alpha1.CustomResourceKind{}
	kind := &spec.CustomResourceKind{}

	err := a.readAPISpec(r, pbKind, kind)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return err
	}

	name := kind.Name
	if len(kind.JSONSchema) > 0 {
		sl := gojsonschema.NewGoLoader(kind.JSONSchema)
		if _, err = gojsonschema.NewSchema(sl); err != nil {
			err = fmt.Errorf("invalid JSONSchema: %s", err.Error())
			api.HandleAPIError(w, r, http.StatusBadRequest, err)
			return err
		}
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldKind := a.service.GetCustomResourceKind(name)
	if update && (oldKind == nil) {
		err = fmt.Errorf("%s not found", name)
		api.HandleAPIError(w, r, http.StatusNotFound, err)
		return err
	}
	if (!update) && (oldKind != nil) {
		err = fmt.Errorf("%s existed", name)
		api.HandleAPIError(w, r, http.StatusConflict, err)
		return err
	}

	a.service.PutCustomResourceKind(kind, update)
	return nil
}

func (a *API) createCustomResourceKind(w http.ResponseWriter, r *http.Request) {
	err := a.saveCustomResourceKind(w, r, false)
	if err == nil {
		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}
}

func (a *API) updateCustomResourceKind(w http.ResponseWriter, r *http.Request) {
	a.saveCustomResourceKind(w, r, true)
}

func (a *API) deleteCustomResourceKind(w http.ResponseWriter, r *http.Request) {
	name, err := a.readURLParam(r, "name")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldKind := a.service.GetCustomResourceKind(name)
	if oldKind == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", name))
		return
	}

	a.service.DeleteCustomResourceKind(name)
	// TODO: remove custom resources?
}

func (a *API) listAllCustomResources(w http.ResponseWriter, r *http.Request) {
	resources := a.service.ListCustomResources("")
	sort.Slice(resources, func(i, j int) bool {
		k1, k2 := resources[i].GetString("kind"), resources[j].GetString("kind")
		if k1 < k2 {
			return true
		}
		if k1 > k2 {
			return false
		}
		return resources[i].GetString("name") < resources[j].GetString("name")
	})

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(resources)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", resources, err))
	}
}

func (a *API) listCustomResources(w http.ResponseWriter, r *http.Request) {
	kind, err := a.readURLParam(r, "kind")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	if a.service.GetCustomResourceKind(kind) == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("kind %s not found", kind))
		return
	}

	resources := a.service.ListCustomResources(kind)
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].GetString("name") < resources[j].GetString("name")
	})

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(resources)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", resources, err))
	}
}

func (a *API) getCustomResource(w http.ResponseWriter, r *http.Request) {
	kind, err := a.readURLParam(r, "kind")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	name, err := a.readURLParam(r, "name")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	if a.service.GetCustomResourceKind(kind) == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("kind %s not found", kind))
		return
	}

	resource := a.service.GetCustomResource(kind, name)
	if resource == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", name))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(resource)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", resource, err))
	}
}

func (a *API) saveCustomResource(w http.ResponseWriter, r *http.Request, update bool) error {
	resource := spec.CustomResource{}
	err := json.NewDecoder(r.Body).Decode(&resource)
	if err != nil {
		err = fmt.Errorf("unmarshal custom resource failed: %v", err)
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return err
	}

	kind, name := resource.GetString("kind"), resource.GetString("name")
	if kind == "" {
		err = fmt.Errorf("kind cannot be empty")
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return err
	}
	if name == "" {
		err = fmt.Errorf("name cannot be empty")
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return err
	}

	k := a.service.GetCustomResourceKind(kind)
	if k == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("kind %s not found", kind))
		return err
	}

	if len(k.JSONSchema) > 0 {
		schema := gojsonschema.NewGoLoader(k.JSONSchema)
		doc := gojsonschema.NewGoLoader(resource)
		res, err := gojsonschema.Validate(schema, doc)
		if err != nil {
			err = fmt.Errorf("validation failed: %v", err)
			api.HandleAPIError(w, r, http.StatusBadRequest, err)
			return err
		}
		if !res.Valid() {
			err = fmt.Errorf("invalid custom resource: %v", res.Errors())
			api.HandleAPIError(w, r, http.StatusBadRequest, err)
			return err
		}
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldResource := a.service.GetCustomResource(kind, name)
	if update && (oldResource == nil) {
		err = fmt.Errorf("custom resource %s not found", name)
		api.HandleAPIError(w, r, http.StatusNotFound, err)
		return err
	}
	if (!update) && (oldResource != nil) {
		err = fmt.Errorf("custom resource %s existed", name)
		api.HandleAPIError(w, r, http.StatusConflict, err)
		return err
	}

	a.service.PutCustomResource(resource, update)
	return nil
}

func (a *API) createCustomResource(w http.ResponseWriter, r *http.Request) {
	err := a.saveCustomResource(w, r, false)
	if err == nil {
		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}
}

func (a *API) updateCustomResource(w http.ResponseWriter, r *http.Request) {
	a.saveCustomResource(w, r, true)
}

func (a *API) deleteCustomResource(w http.ResponseWriter, r *http.Request) {
	kind, err := a.readURLParam(r, "kind")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	name, err := a.readURLParam(r, "name")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldResource := a.service.GetCustomResource(kind, name)
	if oldResource == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", name))
		return
	}

	a.service.DeleteCustomResource(kind, name)
}

func (a *API) watchCustomResources(w http.ResponseWriter, r *http.Request) {
	kind, err := a.readURLParam(r, "kind")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	logger.Infof("begin watch custom resources of kind '%s'", kind)

	w.Header().Set("Content-type", "application/octet-stream")
	a.service.WatchCustomResource(r.Context(), kind, func(resources []spec.CustomResource) {
		err = json.NewEncoder(w).Encode(resources)
		if err != nil {
			logger.Errorf("marshal custom resource failed: %v", err)
		}
		w.Write([]byte("\r\n"))
		w.(http.Flusher).Flush()
	})

	logger.Infof("end watch custom resources of kind '%s'", kind)
}

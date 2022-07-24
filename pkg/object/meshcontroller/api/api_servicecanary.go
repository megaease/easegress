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
	"path"

	"github.com/go-chi/chi/v5"
	v2alpha1 "github.com/megaease/easemesh-api/v2alpha1"

	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
)

func (a *API) readServiceCanaryName(r *http.Request) (string, error) {
	serviceName := chi.URLParam(r, "serviceCanaryName")
	if serviceName == "" {
		return "", fmt.Errorf("empty serviceCanary name")
	}

	return serviceName, nil
}

func (a *API) listServiceCanaries(w http.ResponseWriter, r *http.Request) {
	// NOTE: specs has been sorted alread.
	specs := a.service.ListServiceCanaries()

	var apiSpecs []*v2alpha1.ServiceCanary
	for _, v := range specs {
		serviceCanary := &v2alpha1.ServiceCanary{}
		err := a.convertSpecToPB(v, serviceCanary)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		apiSpecs = append(apiSpecs, serviceCanary)
	}
	buff, err := json.Marshal(apiSpecs)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", specs, err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (a *API) createServiceCanary(w http.ResponseWriter, r *http.Request) {
	pbServiceCanarySpec := &v2alpha1.ServiceCanary{}
	serviceCanarySpec := &spec.ServiceCanary{}

	err := a.readAPISpec(r, pbServiceCanarySpec, serviceCanarySpec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	if serviceCanarySpec.Priority == 0 {
		serviceCanarySpec.Priority = 5
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldSpec := a.service.GetServiceCanary(serviceCanarySpec.Name)
	if oldSpec != nil {
		api.HandleAPIError(w, r, http.StatusConflict, fmt.Errorf("%s existed", serviceCanarySpec.Name))
		return
	}

	a.service.PutServiceCanarySpec(serviceCanarySpec)

	w.Header().Set("Location", path.Join(r.URL.Path, serviceCanarySpec.Name))
	w.WriteHeader(http.StatusCreated)
}

func (a *API) getServiceCanary(w http.ResponseWriter, r *http.Request) {
	serviceCanaryName, err := a.readServiceCanaryName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	serviceCanarySpec := a.service.GetServiceCanary(serviceCanaryName)
	if serviceCanarySpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", serviceCanaryName))
		return
	}
	pbServiceCanarySpec := &v2alpha1.ServiceCanary{}
	err = a.convertSpecToPB(serviceCanarySpec, pbServiceCanarySpec)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", serviceCanarySpec, err))
	}

	buff, err := json.Marshal(pbServiceCanarySpec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", pbServiceCanarySpec, err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (a *API) updateServiceCanary(w http.ResponseWriter, r *http.Request) {
	pbServiceCanarySpec := &v2alpha1.ServiceCanary{}
	serviceCanarySpec := &spec.ServiceCanary{}

	serviceCanaryName, err := a.readServiceCanaryName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	err = a.readAPISpec(r, pbServiceCanarySpec, serviceCanarySpec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if serviceCanaryName != serviceCanarySpec.Name {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", serviceCanaryName, serviceCanarySpec.Name))
		return
	}

	if serviceCanarySpec.Priority == 0 {
		serviceCanarySpec.Priority = 5
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldSpec := a.service.GetServiceCanary(serviceCanaryName)
	if oldSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", serviceCanaryName))
		return
	}

	a.service.PutServiceCanarySpec(serviceCanarySpec)
}

func (a *API) deleteServiceCanary(w http.ResponseWriter, r *http.Request) {
	serviceCanaryName, err := a.readServiceCanaryName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldSpec := a.service.GetServiceCanary(serviceCanaryName)
	if oldSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", serviceCanaryName))
		return
	}

	a.service.DeleteServiceCanary(serviceCanaryName)
}

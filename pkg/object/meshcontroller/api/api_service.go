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
	"sort"

	"github.com/go-chi/chi/v5"
	v2alpha1 "github.com/megaease/easemesh-api/v2alpha1"

	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

type servicesByOrder []*spec.Service

func (s servicesByOrder) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s servicesByOrder) Len() int           { return len(s) }
func (s servicesByOrder) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (a *API) readServiceName(r *http.Request) (string, error) {
	serviceName := chi.URLParam(r, "serviceName")
	if serviceName == "" {
		return "", fmt.Errorf("empty service name")
	}

	return serviceName, nil
}

func (a *API) listServices(w http.ResponseWriter, r *http.Request) {
	specs := a.service.ListServiceSpecs()

	sort.Sort(servicesByOrder(specs))

	var apiSpecs []*v2alpha1.Service
	for _, v := range specs {
		service := &v2alpha1.Service{}
		err := a.convertSpecToPB(v, service)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		apiSpecs = append(apiSpecs, service)
	}

	buff, err := json.Marshal(apiSpecs)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", specs, err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (a *API) createService(w http.ResponseWriter, r *http.Request) {
	pbServiceSpec := &v2alpha1.Service{}
	serviceSpec := &spec.Service{}

	err := a.readAPISpec(r, pbServiceSpec, serviceSpec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldSpec := a.service.GetServiceSpec(serviceSpec.Name)
	if oldSpec != nil {
		api.HandleAPIError(w, r, http.StatusConflict, fmt.Errorf("%s existed", serviceSpec.Name))
		return
	}

	tenantSpec := a.service.GetTenantSpec(serviceSpec.RegisterTenant)
	if tenantSpec == nil {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("tenant %s not found", serviceSpec.RegisterTenant))
		return
	}

	tenantSpec.Services = append(tenantSpec.Services, serviceSpec.Name)

	a.service.PutServiceSpec(serviceSpec)
	a.service.PutTenantSpec(tenantSpec)

	w.Header().Set("Location", path.Join(r.URL.Path, serviceSpec.Name))
	w.WriteHeader(http.StatusCreated)
}

func (a *API) getService(w http.ResponseWriter, r *http.Request) {
	serviceName, err := a.readServiceName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	serviceSpec := a.service.GetServiceSpec(serviceName)
	if serviceSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", serviceName))
		return
	}

	pbServiceSpec := &v2alpha1.Service{}
	err = a.convertSpecToPB(serviceSpec, pbServiceSpec)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", serviceSpec, err))
	}

	buff, err := json.Marshal(pbServiceSpec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", serviceSpec, err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (a *API) updateService(w http.ResponseWriter, r *http.Request) {
	pbServiceSpec := &v2alpha1.Service{}
	serviceSpec := &spec.Service{}

	serviceName, err := a.readServiceName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	err = a.readAPISpec(r, pbServiceSpec, serviceSpec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if serviceName != serviceSpec.Name {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", serviceName, serviceSpec.Name))
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldSpec := a.service.GetServiceSpec(serviceName)
	if oldSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", serviceName))
		return
	}

	if serviceSpec.RegisterTenant != oldSpec.RegisterTenant {
		newTenantSpec := a.service.GetTenantSpec(serviceSpec.RegisterTenant)
		if newTenantSpec == nil {
			api.HandleAPIError(w, r, http.StatusBadRequest,
				fmt.Errorf("tenant %s not found", serviceSpec.RegisterTenant))
			return
		}
		newTenantSpec.Services = append(newTenantSpec.Services, serviceSpec.Name)

		oldTenantSpec := a.service.GetTenantSpec(oldSpec.RegisterTenant)
		if oldTenantSpec == nil {
			panic(fmt.Errorf("tenant %s not found", oldSpec.RegisterTenant))
		}
		oldTenantSpec.Services = stringtool.DeleteStrInSlice(oldTenantSpec.Services, serviceName)

		a.service.PutTenantSpec(newTenantSpec)
		a.service.PutTenantSpec(oldTenantSpec)
	}

	a.service.PutServiceSpec(serviceSpec)
}

func (a *API) deleteService(w http.ResponseWriter, r *http.Request) {
	serviceName, err := a.readServiceName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldSpec := a.service.GetServiceSpec(serviceName)
	if oldSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", serviceName))
		return
	}

	tenantSpec := a.service.GetTenantSpec(oldSpec.RegisterTenant)
	if tenantSpec == nil {
		panic(fmt.Errorf("tenant %s not found", oldSpec.RegisterTenant))
	}

	tenantSpec.Services = stringtool.DeleteStrInSlice(tenantSpec.Services, serviceName)

	a.service.PutTenantSpec(tenantSpec)
	a.service.DeleteServiceSpec(serviceName)
}

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
	v2alpha1 "github.com/megaease/easemesh-api/v2alpha1"
)

type serviceInstancesByOrder []*spec.ServiceInstanceSpec

func (s serviceInstancesByOrder) Less(i, j int) bool {
	return s[i].ServiceName < s[j].ServiceName || s[i].InstanceID < s[j].InstanceID
}
func (s serviceInstancesByOrder) Len() int      { return len(s) }
func (s serviceInstancesByOrder) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (a *API) readServiceInstanceInfo(w http.ResponseWriter, r *http.Request) (string, string, error) {
	serviceName := chi.URLParam(r, "serviceName")
	if serviceName == "" {
		return "", "", fmt.Errorf("empty service name")
	}

	instanceID := chi.URLParam(r, "instanceID")
	if instanceID == "" {
		return "", "", fmt.Errorf("empty instance id")
	}

	return serviceName, instanceID, nil
}

func (a *API) listServiceInstanceSpecs(w http.ResponseWriter, r *http.Request) {
	specs := a.service.ListAllServiceInstanceSpecs()

	sort.Sort(serviceInstancesByOrder(specs))

	var apiSpecs []*v2alpha1.ServiceInstance
	for _, v := range specs {
		instance := &v2alpha1.ServiceInstance{}
		err := a.convertSpecToPB(v, instance)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		apiSpecs = append(apiSpecs, instance)
	}

	buff, err := json.Marshal(apiSpecs)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", specs, err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (a *API) getServiceInstanceSpec(w http.ResponseWriter, r *http.Request) {
	serviceName, instanceID, err := a.readServiceInstanceInfo(w, r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	instanceSpec := a.service.GetServiceInstanceSpec(serviceName, instanceID)
	if instanceSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s/%s not found", serviceName, instanceID))
		return
	}

	pbInstanceSpec := &v2alpha1.ServiceInstance{}
	err = a.convertSpecToPB(instanceSpec, pbInstanceSpec)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", instanceSpec, err))
	}

	buff, err := json.Marshal(pbInstanceSpec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", instanceSpec, err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (a *API) offlineServiceInstance(w http.ResponseWriter, r *http.Request) {
	serviceName, instanceID, err := a.readServiceInstanceInfo(w, r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	instanceSpec := a.service.GetServiceInstanceSpec(serviceName, instanceID)
	if instanceSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s/%s not found", serviceName, instanceID))
		return
	}

	instanceSpec.Status = spec.ServiceStatusOutOfService
	a.service.PutServiceInstanceSpec(instanceSpec)
}

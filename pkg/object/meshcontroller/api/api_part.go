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

	v2alpha1 "github.com/megaease/easemesh-api/v2alpha1"

	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
)

type (
	newPartFunc func() interface{}
	partOfFunc  func(serviceSpec *spec.Service) (interface{}, bool)
	setPartFunc func(serviceSpec *spec.Service, part interface{})

	partMeta struct {
		partName string
		newPart  newPartFunc
		partOf   partOfFunc
		setPart  setPartFunc
		// for protobuf API
		pbSt      interface{}
		newPartPB newPartFunc
	}
)

var (
	mockMeta = &partMeta{
		partName: "mock",
		newPart: func() interface{} {
			return &spec.Mock{}
		},
		partOf: func(serviceSpec *spec.Service) (interface{}, bool) {
			return serviceSpec.Mock, serviceSpec.Mock != nil
		},
		setPart: func(serviceSpec *spec.Service, part interface{}) {
			if part == nil {
				serviceSpec.Mock = nil
				return
			}
			serviceSpec.Mock = part.(*spec.Mock)
		},
		pbSt: v2alpha1.Mock{},
		newPartPB: func() interface{} {
			return &v2alpha1.Mock{}
		},
	}

	resilienceMeta = &partMeta{
		partName: "resilience",
		newPart: func() interface{} {
			return &spec.Resilience{}
		},
		partOf: func(serviceSpec *spec.Service) (interface{}, bool) {
			return serviceSpec.Resilience, serviceSpec.Resilience != nil
		},
		setPart: func(serviceSpec *spec.Service, part interface{}) {
			if part == nil {
				serviceSpec.Resilience = nil
			}
			serviceSpec.Resilience = part.(*spec.Resilience)
		},
		pbSt: v2alpha1.Resilience{},
		newPartPB: func() interface{} {
			return &v2alpha1.Resilience{}
		},
	}

	loadBalanceMeta = &partMeta{
		partName: "loadBalance",
		newPart: func() interface{} {
			return &spec.LoadBalance{}
		},
		partOf: func(serviceSpec *spec.Service) (interface{}, bool) {
			return serviceSpec.LoadBalance, serviceSpec.LoadBalance != nil
		},
		setPart: func(serviceSpec *spec.Service, part interface{}) {
			if part == nil {
				serviceSpec.LoadBalance = nil
				return
			}
			serviceSpec.LoadBalance = part.(*spec.LoadBalance)
		},
		pbSt: v2alpha1.LoadBalance{},
		newPartPB: func() interface{} {
			return &v2alpha1.LoadBalance{}
		},
	}

	outputServerMeta = &partMeta{
		partName: "outputServer",
		newPart: func() interface{} {
			return &spec.ObservabilityOutputServer{}
		},
		partOf: func(serviceSpec *spec.Service) (interface{}, bool) {
			if serviceSpec.Observability == nil {
				return nil, false
			}
			return serviceSpec.Observability.OutputServer, serviceSpec.Observability.OutputServer != nil
		},
		setPart: func(serviceSpec *spec.Service, part interface{}) {
			if serviceSpec.Observability == nil {
				serviceSpec.Observability = &spec.Observability{}
			}
			if part == nil {
				serviceSpec.Observability.OutputServer = nil
				return
			}
			serviceSpec.Observability.OutputServer = part.(*spec.ObservabilityOutputServer)
		},
		pbSt: v2alpha1.ObservabilityOutputServer{},
		newPartPB: func() interface{} {
			return &v2alpha1.ObservabilityOutputServer{}
		},
	}

	tracingsMeta = &partMeta{
		partName: "tracings",
		newPart: func() interface{} {
			return &spec.ObservabilityTracings{}
		},
		partOf: func(serviceSpec *spec.Service) (interface{}, bool) {
			if serviceSpec.Observability == nil {
				return nil, false
			}
			return serviceSpec.Observability.Tracings, serviceSpec.Observability.Tracings != nil
		},
		setPart: func(serviceSpec *spec.Service, part interface{}) {
			if serviceSpec.Observability == nil {
				serviceSpec.Observability = &spec.Observability{}
			}
			if part == nil {
				serviceSpec.Observability.Tracings = nil
				return
			}
			serviceSpec.Observability.Tracings = part.(*spec.ObservabilityTracings)
		},
		pbSt: v2alpha1.ObservabilityTracings{},
		newPartPB: func() interface{} {
			return &v2alpha1.ObservabilityTracings{}
		},
	}

	metricsMeta = &partMeta{
		partName: "metrics",
		newPart: func() interface{} {
			return &spec.ObservabilityMetrics{}
		},
		partOf: func(serviceSpec *spec.Service) (interface{}, bool) {
			if serviceSpec.Observability == nil {
				return nil, false
			}
			return serviceSpec.Observability.Metrics, serviceSpec.Observability.Metrics != nil
		},
		setPart: func(serviceSpec *spec.Service, part interface{}) {
			if serviceSpec.Observability == nil {
				serviceSpec.Observability = &spec.Observability{}
			}
			if part == nil {
				serviceSpec.Observability.Metrics = nil
				return
			}
			serviceSpec.Observability.Metrics = part.(*spec.ObservabilityMetrics)
		},
		pbSt: v2alpha1.ObservabilityMetrics{},
		newPartPB: func() interface{} {
			return &v2alpha1.ObservabilityMetrics{}
		},
	}
)

func (a *API) getPartOfService(meta *partMeta) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serviceName, err := a.readServiceName(r)
		if err != nil {
			api.HandleAPIError(w, r, http.StatusBadRequest, err)
			return
		}

		// NOTE: No need to lock.
		serviceSpec := a.service.GetServiceSpec(serviceName)
		if serviceSpec == nil {
			api.HandleAPIError(w, r, http.StatusNotFound,
				fmt.Errorf("service %s not found", serviceName))
			return
		}

		part, existed := meta.partOf(serviceSpec)
		if !existed {
			api.HandleAPIError(w, r, http.StatusNotFound,
				fmt.Errorf("%s of service %s not found", meta.partName, serviceName))
			return
		}

		partPB := meta.newPartPB()
		err = a.convertSpecToPB(part, partPB)
		if err != nil {
			panic(err)
		}

		buff, err := json.Marshal(partPB)
		if err != nil {
			panic(err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(buff)
	})
}

func (a *API) createPartOfService(meta *partMeta) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serviceName, err := a.readServiceName(r)
		if err != nil {
			api.HandleAPIError(w, r, http.StatusBadRequest, err)
			return
		}

		part := meta.newPart()
		partPB := meta.newPartPB()

		err = a.readAPISpec(r, partPB, part)
		if err != nil {
			api.HandleAPIError(w, r, http.StatusBadRequest, err)
			return
		}

		a.service.Lock()
		defer a.service.Unlock()

		serviceSpec := a.service.GetServiceSpec(serviceName)
		if serviceSpec == nil {
			api.HandleAPIError(w, r, http.StatusNotFound,
				fmt.Errorf("service %s not found", serviceName))
			return
		}

		_, existed := meta.partOf(serviceSpec)
		if existed {
			api.HandleAPIError(w, r, http.StatusConflict,
				fmt.Errorf("%s of service %s existed", meta.partName, serviceName))
			return
		}

		meta.setPart(serviceSpec, part)

		a.service.PutServiceSpec(serviceSpec)

		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	})
}

func (a *API) updatePartOfService(meta *partMeta) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serviceName, err := a.readServiceName(r)
		if err != nil {
			api.HandleAPIError(w, r, http.StatusBadRequest, err)
			return
		}

		part := meta.newPart()
		partPB := meta.newPartPB()

		err = a.readAPISpec(r, partPB, part)
		if err != nil {
			api.HandleAPIError(w, r, http.StatusBadRequest, err)
			return
		}

		a.service.Lock()
		defer a.service.Unlock()

		serviceSpec := a.service.GetServiceSpec(serviceName)
		if serviceSpec == nil {
			api.HandleAPIError(w, r, http.StatusNotFound,
				fmt.Errorf("service %s not found", serviceName))
			return
		}

		_, existed := meta.partOf(serviceSpec)
		if !existed {
			api.HandleAPIError(w, r, http.StatusNotFound,
				fmt.Errorf("%s of service %s found", meta.partName, serviceName))
			return
		}

		meta.setPart(serviceSpec, part)
		a.service.PutServiceSpec(serviceSpec)
	})
}

func (a *API) deletePartOfService(meta *partMeta) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serviceName, err := a.readServiceName(r)
		if err != nil {
			api.HandleAPIError(w, r, http.StatusBadRequest, err)
			return
		}

		a.service.Lock()
		defer a.service.Unlock()

		serviceSpec := a.service.GetServiceSpec(serviceName)
		if serviceSpec == nil {
			api.HandleAPIError(w, r, http.StatusNotFound,
				fmt.Errorf("service %s not found", serviceName))
			return
		}

		_, existed := meta.partOf(serviceSpec)
		if !existed {
			api.HandleAPIError(w, r, http.StatusNotFound,
				fmt.Errorf("%s of service %s found", meta.partName, serviceName))
			return
		}

		meta.setPart(serviceSpec, nil)
		a.service.PutServiceSpec(serviceSpec)
	})
}

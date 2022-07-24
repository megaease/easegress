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
)

type ingressesByOrder []*spec.Ingress

func (s ingressesByOrder) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s ingressesByOrder) Len() int           { return len(s) }
func (s ingressesByOrder) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (a *API) readIngressName(r *http.Request) (string, error) {
	serviceName := chi.URLParam(r, "ingressName")
	if serviceName == "" {
		return "", fmt.Errorf("empty ingress name")
	}

	return serviceName, nil
}

func (a *API) listIngresses(w http.ResponseWriter, r *http.Request) {
	specs := a.service.ListIngressSpecs()

	sort.Sort(ingressesByOrder(specs))
	var apiSpecs []*v2alpha1.Ingress
	for _, v := range specs {
		ingress := &v2alpha1.Ingress{}
		err := a.convertSpecToPB(v, ingress)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		apiSpecs = append(apiSpecs, ingress)
	}
	buff, err := json.Marshal(apiSpecs)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", specs, err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (a *API) createIngress(w http.ResponseWriter, r *http.Request) {
	pbIngressSpec := &v2alpha1.Ingress{}
	ingressSpec := &spec.Ingress{}

	err := a.readAPISpec(r, pbIngressSpec, ingressSpec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldSpec := a.service.GetIngressSpec(ingressSpec.Name)
	if oldSpec != nil {
		api.HandleAPIError(w, r, http.StatusConflict, fmt.Errorf("%s existed", ingressSpec.Name))
		return
	}

	a.service.PutIngressSpec(ingressSpec)

	w.Header().Set("Location", path.Join(r.URL.Path, ingressSpec.Name))
	w.WriteHeader(http.StatusCreated)
}

func (a *API) getIngress(w http.ResponseWriter, r *http.Request) {
	ingressName, err := a.readIngressName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	ingressSpec := a.service.GetIngressSpec(ingressName)
	if ingressSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", ingressName))
		return
	}
	pbIngressSpec := &v2alpha1.Ingress{}
	err = a.convertSpecToPB(ingressSpec, pbIngressSpec)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", ingressSpec, err))
	}

	buff, err := json.Marshal(pbIngressSpec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", pbIngressSpec, err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (a *API) updateIngress(w http.ResponseWriter, r *http.Request) {
	pbIngressSpec := &v2alpha1.Ingress{}
	ingressSpec := &spec.Ingress{}

	ingressName, err := a.readIngressName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	err = a.readAPISpec(r, pbIngressSpec, ingressSpec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if ingressName != ingressSpec.Name {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", ingressName, ingressSpec.Name))
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldSpec := a.service.GetIngressSpec(ingressName)
	if oldSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", ingressName))
		return
	}

	a.service.PutIngressSpec(ingressSpec)
}

func (a *API) deleteIngress(w http.ResponseWriter, r *http.Request) {
	ingressName, err := a.readIngressName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldSpec := a.service.GetIngressSpec(ingressName)
	if oldSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", ingressName))
		return
	}

	a.service.DeleteIngressSpec(ingressName)
}

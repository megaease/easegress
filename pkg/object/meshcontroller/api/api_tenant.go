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
	"time"

	"github.com/go-chi/chi/v5"
	v2alpha1 "github.com/megaease/easemesh-api/v2alpha1"

	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
)

type tenantsByOrder []*spec.Tenant

func (s tenantsByOrder) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s tenantsByOrder) Len() int           { return len(s) }
func (s tenantsByOrder) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (a *API) readTenantName(r *http.Request) (string, error) {
	serviceName := chi.URLParam(r, "tenantName")
	if serviceName == "" {
		return "", fmt.Errorf("empty tenant name")
	}

	return serviceName, nil
}

func (a *API) listTenants(w http.ResponseWriter, r *http.Request) {
	specs := a.service.ListTenantSpecs()

	sort.Sort(tenantsByOrder(specs))

	var apiSpecs []*v2alpha1.Tenant
	for _, v := range specs {
		tenant := &v2alpha1.Tenant{}
		err := a.convertSpecToPB(v, &tenant)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		apiSpecs = append(apiSpecs, tenant)
	}

	buff, err := json.Marshal(apiSpecs)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", specs, err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (a *API) createTenant(w http.ResponseWriter, r *http.Request) {
	pbTenantSpec := &v2alpha1.Tenant{}
	tenantSpec := &spec.Tenant{}

	err := a.readAPISpec(r, pbTenantSpec, tenantSpec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	if len(tenantSpec.Services) > 0 {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("services are not empty"))
		return
	}
	tenantSpec.CreatedAt = time.Now().Format(time.RFC3339)

	a.service.Lock()
	defer a.service.Unlock()

	oldSpec := a.service.GetTenantSpec(tenantSpec.Name)
	if oldSpec != nil {
		api.HandleAPIError(w, r, http.StatusConflict, fmt.Errorf("%s existed", tenantSpec.Name))
		return
	}

	a.service.PutTenantSpec(tenantSpec)

	w.Header().Set("Location", path.Join(r.URL.Path, tenantSpec.Name))
	w.WriteHeader(http.StatusCreated)
}

func (a *API) getTenant(w http.ResponseWriter, r *http.Request) {
	tenantName, err := a.readTenantName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	tenantSpec := a.service.GetTenantSpec(tenantName)
	if tenantSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", tenantName))
		return
	}

	pbTenantSpec := &v2alpha1.Tenant{}
	err = a.convertSpecToPB(tenantSpec, pbTenantSpec)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", tenantSpec, err))
	}

	buff, err := json.Marshal(pbTenantSpec)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", pbTenantSpec, err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buff)
}

func (a *API) updateTenant(w http.ResponseWriter, r *http.Request) {
	pbTenantSpec := &v2alpha1.Tenant{}
	tenantSpec := &spec.Tenant{}

	tenantName, err := a.readTenantName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	err = a.readAPISpec(r, pbTenantSpec, tenantSpec)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if tenantName != tenantSpec.Name {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("name conflict: %s %s", tenantName, tenantSpec.Name))
		return
	}

	if len(tenantSpec.Services) > 0 {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("services are not empty"))
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldSpec := a.service.GetTenantSpec(tenantName)
	if oldSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", tenantName))
		return
	}

	// NOTE: The fields below can't be updated.
	tenantSpec.Services, tenantSpec.CreatedAt = oldSpec.Services, oldSpec.CreatedAt

	a.service.PutTenantSpec(tenantSpec)
}

func (a *API) deleteTenant(w http.ResponseWriter, r *http.Request) {
	tenantName, err := a.readTenantName(r)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldSpec := a.service.GetTenantSpec(tenantName)
	if oldSpec == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", tenantName))
		return
	}

	if len(oldSpec.Services) != 0 {
		api.HandleAPIError(w, r, http.StatusBadRequest,
			fmt.Errorf("%s got services: %v", tenantName, oldSpec.Services))
		return
	}

	a.service.DeleteTenantSpec(tenantName)
}

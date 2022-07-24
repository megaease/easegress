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

	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easemesh-api/v2alpha1"
)

func (a *API) listHTTPRouteGroups(w http.ResponseWriter, r *http.Request) {
	groups := a.service.ListHTTPRouteGroups()
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	var pbGroups []*v2alpha1.HTTPRouteGroup
	for _, v := range groups {
		group := &v2alpha1.HTTPRouteGroup{}
		err := a.convertSpecToPB(v, group)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		pbGroups = append(pbGroups, group)
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(pbGroups)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", groups, err))
	}
}

func (a *API) getHTTPRouteGroup(w http.ResponseWriter, r *http.Request) {
	name, err := a.readURLParam(r, "name")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	group := a.service.GetHTTPRouteGroup(name)
	if group == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", name))
		return
	}

	pbGroup := &v2alpha1.HTTPRouteGroup{}
	err = a.convertSpecToPB(group, pbGroup)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", group, err))
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(pbGroup)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", pbGroup, err))
	}
}

func (a *API) saveHTTPRouteGroup(w http.ResponseWriter, r *http.Request, update bool) error {
	pbGroup := &v2alpha1.HTTPRouteGroup{}
	group := &spec.HTTPRouteGroup{}

	err := a.readAPISpec(r, pbGroup, group)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return err
	}

	name := group.Name

	a.service.Lock()
	defer a.service.Unlock()

	oldGroup := a.service.GetHTTPRouteGroup(name)
	if update && (oldGroup == nil) {
		err = fmt.Errorf("%s not found", name)
		api.HandleAPIError(w, r, http.StatusNotFound, err)
		return err
	}
	if (!update) && (oldGroup != nil) {
		err = fmt.Errorf("%s existed", name)
		api.HandleAPIError(w, r, http.StatusConflict, err)
		return err
	}

	a.service.PutHTTPRouteGroup(group)
	return nil
}

func (a *API) createHTTPRouteGroup(w http.ResponseWriter, r *http.Request) {
	err := a.saveHTTPRouteGroup(w, r, false)
	if err == nil {
		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}
}

func (a *API) updateHTTPRouteGroup(w http.ResponseWriter, r *http.Request) {
	a.saveHTTPRouteGroup(w, r, true)
}

func (a *API) deleteHTTPRouteGroup(w http.ResponseWriter, r *http.Request) {
	name, err := a.readURLParam(r, "name")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldGroup := a.service.GetHTTPRouteGroup(name)
	if oldGroup == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", name))
		return
	}

	a.service.DeleteHTTPRouteGroup(name)
}

func (a *API) listTrafficTargets(w http.ResponseWriter, r *http.Request) {
	tts := a.service.ListTrafficTargets()
	sort.Slice(tts, func(i, j int) bool {
		return tts[i].Name < tts[j].Name
	})

	var pbTrafficTargets []*v2alpha1.TrafficTarget
	for _, v := range tts {
		tt := &v2alpha1.TrafficTarget{}
		err := a.convertSpecToPB(v, tt)
		if err != nil {
			logger.Errorf("convert spec %#v to pb spec failed: %v", v, err)
			continue
		}
		pbTrafficTargets = append(pbTrafficTargets, tt)
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(pbTrafficTargets)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", tts, err))
	}
}

func (a *API) getTrafficTarget(w http.ResponseWriter, r *http.Request) {
	name, err := a.readURLParam(r, "name")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	tt := a.service.GetTrafficTarget(name)
	if tt == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", name))
		return
	}

	pbTrafficTarget := &v2alpha1.TrafficTarget{}
	err = a.convertSpecToPB(tt, pbTrafficTarget)
	if err != nil {
		panic(fmt.Errorf("convert spec %#v to pb failed: %v", tt, err))
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(pbTrafficTarget)
	if err != nil {
		panic(fmt.Errorf("marshal %#v to json failed: %v", pbTrafficTarget, err))
	}
}

func (a *API) saveTrafficTarget(w http.ResponseWriter, r *http.Request, update bool) error {
	pbTrafficTarget := &v2alpha1.TrafficTarget{}
	tt := &spec.TrafficTarget{}

	err := a.readAPISpec(r, pbTrafficTarget, tt)
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return err
	}
	name := tt.Name

	a.service.Lock()
	defer a.service.Unlock()

	oldTrafficTarget := a.service.GetTrafficTarget(name)
	if update && (oldTrafficTarget == nil) {
		err = fmt.Errorf("%s not found", name)
		api.HandleAPIError(w, r, http.StatusNotFound, err)
		return err
	}
	if (!update) && (oldTrafficTarget != nil) {
		err = fmt.Errorf("%s existed", name)
		api.HandleAPIError(w, r, http.StatusConflict, err)
		return err
	}

	a.service.PutTrafficTarget(tt)
	return nil
}

func (a *API) createTrafficTarget(w http.ResponseWriter, r *http.Request) {
	err := a.saveTrafficTarget(w, r, false)
	if err == nil {
		w.Header().Set("Location", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}
}

func (a *API) updateTrafficTarget(w http.ResponseWriter, r *http.Request) {
	a.saveTrafficTarget(w, r, true)
}

func (a *API) deleteTrafficTarget(w http.ResponseWriter, r *http.Request) {
	name, err := a.readURLParam(r, "name")
	if err != nil {
		api.HandleAPIError(w, r, http.StatusBadRequest, err)
		return
	}

	a.service.Lock()
	defer a.service.Unlock()

	oldTrafficTarget := a.service.GetTrafficTarget(name)
	if oldTrafficTarget == nil {
		api.HandleAPIError(w, r, http.StatusNotFound, fmt.Errorf("%s not found", name))
		return
	}

	a.service.DeleteTrafficTarget(name)
}

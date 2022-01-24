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

package headertojson

import (
	"bytes"
	"io"
	"net/http"

	json "github.com/goccy/go-json"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/object/httppipeline"
)

const (
	// Kind is the kind of Kafka
	Kind = "HeaderToJSON"

	resultJSONEncodeDecodeErr = "jsonEncodeDecodeErr"
	resultBodyReadErr         = "bodyReadErr"
)

func init() {
	httppipeline.Register(&HeaderToJSON{})
}

type (
	// HeaderToJSON put http request headers into body as JSON fields.
	HeaderToJSON struct {
		filterSpec *httppipeline.FilterSpec
		spec       *Spec
		headerMap  map[string]string
	}
)

var _ httppipeline.Filter = (*HeaderToJSON)(nil)

// Kind return kind of HeaderToJSON
func (h *HeaderToJSON) Kind() string {
	return Kind
}

// DefaultSpec return default spec of HeaderToJSON
func (h *HeaderToJSON) DefaultSpec() interface{} {
	return &Spec{}
}

// Description return description of HeaderToJSON
func (h *HeaderToJSON) Description() string {
	return "HeaderToJSON convert http request header to json"
}

// Results return possible results of HeaderToJSON
func (h *HeaderToJSON) Results() []string {
	return []string{resultJSONEncodeDecodeErr, resultBodyReadErr}
}

func (h *HeaderToJSON) init() {
	h.headerMap = make(map[string]string)
	for _, header := range h.spec.HeaderMap {
		h.headerMap[http.CanonicalHeaderKey(header.Header)] = header.JSON
	}
}

// Init init HeaderToJSON
func (h *HeaderToJSON) Init(filterSpec *httppipeline.FilterSpec) {
	h.filterSpec, h.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	h.init()
}

// Inherit init HeaderToJSON based on previous generation
func (h *HeaderToJSON) Inherit(filterSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	previousGeneration.Close()
	h.Init(filterSpec)
}

// Close close HeaderToJSON
func (h *HeaderToJSON) Close() {
}

// Status return status of HeaderToJSON
func (h *HeaderToJSON) Status() interface{} {
	return nil
}

func (h *HeaderToJSON) decodeJSON(body []byte) (map[string]interface{}, error) {
	res := make(map[string]interface{})
	err := json.Unmarshal(body, &res)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return res, nil
}

func (h *HeaderToJSON) decodeArrayJSON(body []byte) ([]map[string]interface{}, error) {
	res := []map[string]interface{}{}
	err := json.Unmarshal(body, &res)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return res, nil
}

// Handle handle HTTPContext
func (h *HeaderToJSON) Handle(ctx context.HTTPContext) string {
	result := h.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (h *HeaderToJSON) handleBodyMap(ctx context.HTTPContext, bodyMap map[string]interface{}, headerMap map[string]interface{}) string {
	for k, v := range headerMap {
		bodyMap[k] = v
	}
	body, err := json.Marshal(bodyMap)
	if err != nil {
		return resultJSONEncodeDecodeErr
	}
	ctx.Request().SetBody(bytes.NewReader(body))
	return ""
}

func (h *HeaderToJSON) handleBodyArray(ctx context.HTTPContext, bodyArray []map[string]interface{}, headerMap map[string]interface{}) string {
	for i := range bodyArray {
		for k, v := range headerMap {
			bodyArray[i][k] = v
		}
	}
	body, err := json.Marshal(bodyArray)
	if err != nil {
		return resultJSONEncodeDecodeErr
	}
	ctx.Request().SetBody(bytes.NewReader(body))
	return ""
}

func (h *HeaderToJSON) handle(ctx context.HTTPContext) string {
	headerMap := make(map[string]interface{})
	for header, json := range h.headerMap {
		value := ctx.Request().Header().Get(header)
		if value != "" {
			headerMap[json] = value
		}
	}
	if len(headerMap) == 0 {
		return ""
	}

	reqBody, err := io.ReadAll(ctx.Request().Body())
	if err != nil {
		return resultBodyReadErr
	}
	if len(reqBody) == 0 {
		m := make(map[string]interface{})
		return h.handleBodyMap(ctx, m, headerMap)
	}

	bodyMap, err := h.decodeJSON(reqBody)
	if err == nil {
		return h.handleBodyMap(ctx, bodyMap, headerMap)
	}
	bodyArray, err := h.decodeArrayJSON(reqBody)
	if err == nil {
		return h.handleBodyArray(ctx, bodyArray, headerMap)
	}
	return resultJSONEncodeDecodeErr
}

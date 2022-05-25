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

package corsadaptor

import (
	"net/http"

	"github.com/rs/cors"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/object/httppipeline"
)

const (
	// Kind is kind of CORSAdaptor.
	Kind = "CORSAdaptor"

	resultPreflighted = "preflighted"
)

var results = []string{resultPreflighted}

func init() {
	httppipeline.Register(&CORSAdaptor{})
}

type (
	// CORSAdaptor is filter for CORS request.
	CORSAdaptor struct {
		filterSpec *httppipeline.FilterSpec
		spec       *Spec

		cors *cors.Cors
	}

	// Spec describes of CORSAdaptor.
	Spec struct {
		AllowedOrigins   []string `yaml:"allowedOrigins" jsonschema:"omitempty"`
		AllowedMethods   []string `yaml:"allowedMethods" jsonschema:"omitempty,uniqueItems=true,format=httpmethod-array"`
		AllowedHeaders   []string `yaml:"allowedHeaders" jsonschema:"omitempty"`
		AllowCredentials bool     `yaml:"allowCredentials" jsonschema:"omitempty"`
		ExposedHeaders   []string `yaml:"exposedHeaders" jsonschema:"omitempty"`
		MaxAge           int      `yaml:"maxAge" jsonschema:"omitempty"`
		// If true, handle requests with 'Origin' header. https://fetch.spec.whatwg.org/#http-requests
		// By default, only CORS-preflight requests are handled.
		SupportCORSRequest bool `yaml:"supportCORSRequest" jsonschema:"omitempty"`
	}
)

// Kind returns the kind of CORSAdaptor.
func (a *CORSAdaptor) Kind() string {
	return Kind
}

// DefaultSpec returns default spec of CORSAdaptor.
func (a *CORSAdaptor) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of CORSAdaptor.
func (a *CORSAdaptor) Description() string {
	return "CORSAdaptor adapts CORS stuff."
}

// Results returns the results of CORSAdaptor.
func (a *CORSAdaptor) Results() []string {
	return results
}

// Init initializes CORSAdaptor.
func (a *CORSAdaptor) Init(filterSpec *httppipeline.FilterSpec) {
	a.filterSpec, a.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	a.reload()
}

// Inherit inherits previous generation of CORSAdaptor.
func (a *CORSAdaptor) Inherit(filterSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	previousGeneration.Close()
	a.Init(filterSpec)
}

func (a *CORSAdaptor) reload() {
	a.cors = cors.New(cors.Options{
		AllowedOrigins:   a.spec.AllowedOrigins,
		AllowedMethods:   a.spec.AllowedMethods,
		AllowedHeaders:   a.spec.AllowedHeaders,
		AllowCredentials: a.spec.AllowCredentials,
		ExposedHeaders:   a.spec.ExposedHeaders,
		MaxAge:           a.spec.MaxAge,
	})
}

// Handle handles simple cross-origin requests or directs.
func (a *CORSAdaptor) Handle(ctx context.HTTPContext) string {
	if a.spec.SupportCORSRequest {
		result := a.handleCORS(ctx)
		return ctx.CallNextHandler(result)
	}
	result := a.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (a *CORSAdaptor) handle(ctx context.HTTPContext) string {
	r := ctx.Request()
	w := ctx.Response()
	method := r.Method()
	headerAllowMethod := r.Header().Get("Access-Control-Request-Method")
	if method == http.MethodOptions && headerAllowMethod != "" {
		a.cors.HandlerFunc(w.Std(), r.Std())
		return resultPreflighted
	}
	return ""
}

func (a *CORSAdaptor) handleCORS(ctx context.HTTPContext) string {
	r := ctx.Request()
	w := ctx.Response()
	method := r.Method()
	isCorsRequest := r.Header().Get("Origin") != ""
	isPreflight := method == http.MethodOptions && r.Header().Get("Access-Control-Request-Method") != ""
	// set CORS headers to response
	a.cors.HandlerFunc(w.Std(), r.Std())
	if !isCorsRequest {
		return "" // next filter
	}
	if isPreflight {
		return resultPreflighted // pipeline jumpIf skips following filters
	}
	return "" // next filter
}

// Status return status.
func (a *CORSAdaptor) Status() interface{} {
	return nil
}

// Close closes CORSAdaptor.
func (a *CORSAdaptor) Close() {}

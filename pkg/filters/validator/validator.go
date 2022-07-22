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

package validator

import (
	"net/http"

	"fmt"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpheader"
	"github.com/megaease/easegress/pkg/util/signer"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

const (
	// Kind is the kind of Validator.
	Kind = "Validator"

	resultInvalid = "invalid"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "Validator validates HTTP request.",
	Results:     []string{resultInvalid},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &Validator{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// Validator is filter Validator.
	Validator struct {
		spec *Spec

		headers   *httpheader.Validator
		jwt       *JWTValidator
		signer    *signer.Signer
		oauth2    *OAuth2Validator
		basicAuth *BasicAuthValidator
	}

	// Spec describes the Validator.
	Spec struct {
		filters.BaseSpec `yaml:",inline"`

		Headers   *httpheader.ValidatorSpec `yaml:"headers,omitempty" jsonschema:"omitempty"`
		JWT       *JWTValidatorSpec         `yaml:"jwt,omitempty" jsonschema:"omitempty"`
		Signature *signer.Spec              `yaml:"signature,omitempty" jsonschema:"omitempty"`
		OAuth2    *OAuth2ValidatorSpec      `yaml:"oauth2,omitempty" jsonschema:"omitempty"`
		BasicAuth *BasicAuthValidatorSpec   `yaml:"basicAuth,omitempty" jsonschema:"omitempty"`
	}
)

// Validate verifies that at least one of the validations is defined.
func (spec Spec) Validate() error {
	if spec == (Spec{}) {
		return fmt.Errorf("none of the validations are defined")
	}
	return nil
}

// Name returns the name of the Validator filter instance.
func (v *Validator) Name() string {
	return v.spec.Name()
}

// Kind returns the kind of Validator.
func (v *Validator) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the Validator
func (v *Validator) Spec() filters.Spec {
	return v.spec
}

// Init initializes Validator.
func (v *Validator) Init() {
	v.reload()
}

// Inherit inherits previous generation of Validator.
func (v *Validator) Inherit(previousGeneration filters.Filter) {
	v.reload()
}

func (v *Validator) reload() {
	if v.spec.Headers != nil {
		v.headers = httpheader.NewValidator(v.spec.Headers)
	}
	if v.spec.JWT != nil {
		v.jwt = NewJWTValidator(v.spec.JWT)
	}
	if v.spec.Signature != nil {
		v.signer = signer.CreateFromSpec(v.spec.Signature)
	}
	if v.spec.OAuth2 != nil {
		v.oauth2 = NewOAuth2Validator(v.spec.OAuth2)
	}
	if v.spec.BasicAuth != nil {
		v.basicAuth = NewBasicAuthValidator(v.spec.BasicAuth, v.spec.Super())
	}
}

// Handle validates the request in the context.
func (v *Validator) Handle(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*httpprot.Request)

	prepareErrorResponse := func(status int, tagPrefix string, err error) {
		resp, _ := httpprot.NewResponse(nil)
		resp.SetStatusCode(status)
		ctx.SetOutputResponse(resp)
		ctx.AddTag(stringtool.Cat(tagPrefix, err.Error()))
	}

	if v.headers != nil {
		if err := v.headers.Validate(httpheader.New(req.Std().Header)); err != nil {
			prepareErrorResponse(http.StatusBadRequest, "header validator: ", err)
			return resultInvalid
		}
	}
	if v.jwt != nil {
		if err := v.jwt.Validate(req); err != nil {
			prepareErrorResponse(http.StatusUnauthorized, "JWT validator: ", err)
			return resultInvalid
		}
	}
	if v.signer != nil {
		vCtx := v.signer.NewVerificationContext()
		if req.IsStream() {
			vCtx.ExcludeBody(true)
		}
		if err := vCtx.Verify(req.Std(), req.GetPayload); err != nil {
			prepareErrorResponse(http.StatusUnauthorized, "signature validator: ", err)
			return resultInvalid
		}
	}
	if v.oauth2 != nil {
		if err := v.oauth2.Validate(req); err != nil {
			prepareErrorResponse(http.StatusUnauthorized, "oauth2 validator: ", err)
			return resultInvalid
		}
	}
	if v.basicAuth != nil {
		if err := v.basicAuth.Validate(req); err != nil {
			prepareErrorResponse(http.StatusUnauthorized, "http basic validator: ", err)
			return resultInvalid
		}
	}

	return ""
}

// Status returns status.
func (v *Validator) Status() interface{} { return nil }

// Close closes validations.
func (v *Validator) Close() {
	if v.basicAuth != nil {
		v.basicAuth.Close()
	}
}

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

package requestadaptor

import (
	"fmt"
	"io"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpheader"
	"github.com/megaease/easegress/pkg/util/pathadaptor"
	"github.com/megaease/easegress/pkg/util/readers"
	"github.com/megaease/easegress/pkg/util/signer"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

const (
	// Kind is the kind of RequestAdaptor.
	Kind = "RequestAdaptor"

	resultDecompressFailed = "decompressFailed"
	resultCompressFailed   = "compressFailed"
	resultSignFailed       = "signFailed"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "RequestAdaptor adapts request.",
	Results: []string{
		resultDecompressFailed,
		resultCompressFailed,
	},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &RequestAdaptor{spec: spec.(*Spec)}
	},
}

var signerConfigs = map[string]signerConfig{
	"aws4": {
		literal: &signer.Literal{
			ScopeSuffix:      "aws4_request",
			AlgorithmName:    "X-Amz-Algorithm",
			AlgorithmValue:   "AWS4-HMAC-SHA256",
			SignedHeaders:    "X-Amz-SignedHeaders",
			Signature:        "X-Amz-Signature",
			Date:             "X-Amz-Date",
			Expires:          "X-Amz-Expires",
			Credential:       "X-Amz-Credential",
			ContentSHA256:    "X-Amz-Content-Sha256",
			SigningKeyPrefix: "AWS4",
		},

		headerHoisting: &signer.HeaderHoisting{
			AllowedPrefix:    []string{"X-Amz-"},
			DisallowedPrefix: []string{"X-Amz-Meta-"},
			Disallowed: []string{
				"Cache-Control",
				"Content-Disposition",
				"Content-Encoding",
				"Content-Language",
				"Content-Md5",
				"Content-Type",
				"Expires",
				"If-Match",
				"If-Modified-Since",
				"If-None-Match",
				"If-Unmodified-Since",
				"Range",
				"X-Amz-Acl",
				"X-Amz-Copy-Source",
				"X-Amz-Copy-Source-If-Match",
				"X-Amz-Copy-Source-If-Modified-Since",
				"X-Amz-Copy-Source-If-None-Match",
				"X-Amz-Copy-Source-If-Unmodified-Since",
				"X-Amz-Copy-Source-Range",
				"X-Amz-Copy-Source-Server-Side-Encryption-Customer-Algorithm",
				"X-Amz-Copy-Source-Server-Side-Encryption-Customer-Key",
				"X-Amz-Copy-Source-Server-Side-Encryption-Customer-Key-Md5",
				"X-Amz-Grant-Full-control",
				"X-Amz-Grant-Read",
				"X-Amz-Grant-Read-Acp",
				"X-Amz-Grant-Write",
				"X-Amz-Grant-Write-Acp",
				"X-Amz-Metadata-Directive",
				"X-Amz-Mfa",
				"X-Amz-Request-Payer",
				"X-Amz-Server-Side-Encryption",
				"X-Amz-Server-Side-Encryption-Aws-Kms-Key-Id",
				"X-Amz-Server-Side-Encryption-Customer-Algorithm",
				"X-Amz-Server-Side-Encryption-Customer-Key",
				"X-Amz-Server-Side-Encryption-Customer-Key-Md5",
				"X-Amz-Storage-Class",
				"X-Amz-Tagging",
				"X-Amz-Website-Redirect-Location",
				"X-Amz-Content-Sha256",
			},
		},
	},
}

func init() {
	filters.Register(kind)
}

type (
	// RequestAdaptor is filter RequestAdaptor.
	RequestAdaptor struct {
		spec *Spec

		pa     *pathadaptor.PathAdaptor
		signer *signer.Signer
	}

	// Spec is HTTPAdaptor Spec.
	Spec struct {
		filters.BaseSpec `yaml:",inline"`

		Host       string                `yaml:"host" jsonschema:"omitempty"`
		Method     string                `yaml:"method" jsonschema:"omitempty,format=httpmethod"`
		Path       *pathadaptor.Spec     `yaml:"path,omitempty" jsonschema:"omitempty"`
		Header     *httpheader.AdaptSpec `yaml:"header,omitempty" jsonschema:"omitempty"`
		Body       string                `yaml:"body" jsonschema:"omitempty"`
		Compress   string                `yaml:"compress" jsonschema:"omitempty"`
		Decompress string                `yaml:"decompress" jsonschema:"omitempty"`
		Sign       *SignerSpec           `yaml:"sign,omitempty" jsonschema:"omitempty"`
	}

	// SignerSpec is the spec of the request signer.
	SignerSpec struct {
		signer.Spec `yaml:",inline"`
		For         string   `yaml:"for" jsonschema:"omitempty,enum=,enum=aws4"`
		Scopes      []string `yaml:"scopes" jsonschema:"omitempty"`
	}

	signerConfig struct {
		literal        *signer.Literal
		headerHoisting *signer.HeaderHoisting
	}
)

// Validate verifies that at least one of the validations is defined.
func (spec *Spec) Validate() error {
	if spec.Decompress != "" && spec.Decompress != "gzip" {
		return fmt.Errorf("RequestAdaptor only support decompress type of gzip")
	}
	if spec.Compress != "" && spec.Compress != "gzip" {
		return fmt.Errorf("RequestAdaptor only support decompress type of gzip")
	}
	if spec.Compress != "" && spec.Decompress != "" {
		return fmt.Errorf("RequestAdaptor can only do compress or decompress for given request body, not both")
	}
	if spec.Body != "" && spec.Decompress != "" {
		return fmt.Errorf("No need to decompress when body is specified in RequestAdaptor spec")
	}
	if spec.Sign == nil {
		return nil
	}
	s := spec.Sign
	if s.For != "" {
		if _, ok := signerConfigs[s.For]; !ok {
			return fmt.Errorf("%q is not a predefined signer configuration", s.For)
		}
	}

	return nil
}

// Name returns the name of the RequestAdaptor filter instance.
func (ra *RequestAdaptor) Name() string {
	return ra.spec.Name()
}

// Kind returns the kind of RequestAdaptor.
func (ra *RequestAdaptor) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the RequestAdaptor
func (ra *RequestAdaptor) Spec() filters.Spec {
	return ra.spec
}

// Init initializes RequestAdaptor.
func (ra *RequestAdaptor) Init() {
	ra.reload()
}

// Inherit inherits previous generation of RequestAdaptor.
func (ra *RequestAdaptor) Inherit(previousGeneration filters.Filter) {
	ra.Init()
}

func (ra *RequestAdaptor) reload() {
	if ra.spec.Path != nil {
		ra.pa = pathadaptor.New(ra.spec.Path)
	}
	if s := ra.spec.Sign; s != nil {
		sc, ok := signerConfigs[s.For]
		if ok {
			s.Literal = sc.literal
			s.HeaderHoisting = sc.headerHoisting
		}
		ra.signer = signer.CreateFromSpec(&s.Spec)
	}
}

func adaptHeader(req *httpprot.Request, as *httpheader.AdaptSpec) {
	h := req.Std().Header
	for _, key := range as.Del {
		h.Del(key)
	}
	for key, value := range as.Set {
		h.Set(key, value)
	}
	for key, value := range as.Add {
		h.Add(key, value)
	}
}

// Handle adapts request.
func (ra *RequestAdaptor) Handle(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*httpprot.Request)
	method, path := req.Method(), req.Path()

	if ra.spec.Method != "" && ra.spec.Method != method {
		ctx.AddTag(stringtool.Cat("requestAdaptor: method ", method, " adapted to ", ra.spec.Method))
		req.SetMethod(ra.spec.Method)
	}

	if ra.pa != nil {
		adaptedPath := ra.pa.Adapt(path)
		if adaptedPath != path {
			ctx.AddTag(stringtool.Cat("requestAdaptor: path ", path, " adapted to ", adaptedPath))
		}
		req.SetPath(adaptedPath)
	}

	if ra.spec.Header != nil {
		adaptHeader(req, ra.spec.Header)
	}

	if len(ra.spec.Body) != 0 {
		req.SetPayload([]byte(ra.spec.Body))
		req.Std().Header.Del("Content-Encoding")
	}

	if len(ra.spec.Host) != 0 {
		req.SetHost(ra.spec.Host)
	}

	if ra.spec.Compress != "" {
		res := ra.processCompress(req)
		if res != "" {
			return res
		}
	}

	if ra.spec.Decompress != "" {
		res := ra.processDecompress(req)
		if res != "" {
			return res
		}
	}

	if ra.signer != nil {
		res := ra.signRequest(req)
		if res != "" {
			return res
		}
	}

	return ""
}

func (ra *RequestAdaptor) processCompress(req *httpprot.Request) string {
	encoding := req.HTTPHeader().Get("Content-Encoding")
	if encoding != "" {
		return ""
	}

	zr := readers.NewGZipCompressReader(req.GetPayload())
	if req.IsStream() {
		req.SetPayload(zr)
	} else {
		data, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			logger.Errorf("compress request body failed: %v", err)
			return resultCompressFailed
		}
		req.SetPayload(data)
	}

	req.HTTPHeader().Set("Content-Encoding", "gzip")
	return ""
}

func (ra *RequestAdaptor) processDecompress(req *httpprot.Request) string {
	encoding := req.HTTPHeader().Get("Content-Encoding")
	if ra.spec.Decompress != "gzip" || encoding != "gzip" {
		return ""
	}

	zr, err := readers.NewGZipDecompressReader(req.GetPayload())
	if err != nil {
		return resultDecompressFailed
	}

	if req.IsStream() {
		req.SetPayload(zr)
	} else {
		data, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			logger.Errorf("decompress request body failed: %v", err)
			return resultDecompressFailed
		}
		req.SetPayload(data)
	}

	req.HTTPHeader().Del("Content-Encoding")
	return ""
}

func (ra *RequestAdaptor) signRequest(req *httpprot.Request) string {
	sCtx := ra.signer.NewSigningContext(time.Now(), ra.spec.Sign.Scopes...)
	if req.IsStream() {
		sCtx.ExcludeBody(true)
	}
	err := sCtx.Sign(req.Std(), req.GetPayload)
	if err != nil {
		logger.Errorf("sign request failed: %v", err)
		return resultSignFailed
	}
	return ""
}

// Status returns status.
func (ra *RequestAdaptor) Status() interface{} {
	return nil
}

// Close closes RequestAdaptor.
func (ra *RequestAdaptor) Close() {
}

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

package httpprot

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/megaease/easegress/pkg/protocols"
	"github.com/megaease/easegress/pkg/util/readers"
	"github.com/tomasen/realip"
)

// Request wraps http.Request.
//
// The payload of the request can be replaced with a new one, but it will
// never replace the body of the underlying http.Request.
//
// Code should always use payload functions of this request to read the
// body of the original request, and never use the Body of the original
// request directly.
type Request struct {
	*http.Request
	stream  *readers.ByteCountReader
	payload []byte
	realIP  string
}

var (
	// ErrRequestEntityTooLarge means the request entity is too large.
	ErrRequestEntityTooLarge = fmt.Errorf("request entity too large")

	methods = map[string]struct{}{
		http.MethodGet:     {},
		http.MethodHead:    {},
		http.MethodPost:    {},
		http.MethodPut:     {},
		http.MethodPatch:   {},
		http.MethodDelete:  {},
		http.MethodConnect: {},
		http.MethodOptions: {},
		http.MethodTrace:   {},
	}

	_ protocols.Request = (*Request)(nil)
)

// NewRequest creates a new request from a standard request. If stdr is not
// nil, FetchPayload must be called before any read of the request body.
func NewRequest(stdr *http.Request) (*Request, error) {
	if stdr == nil {
		stdr = &http.Request{Body: http.NoBody}
		return &Request{Request: stdr}, nil
	}

	r := &Request{Request: stdr}
	r.realIP = realip.FromRequest(stdr)
	return r, nil
}

// IsStream returns whether the payload of the request is a stream.
func (r *Request) IsStream() bool {
	return r.stream != nil
}

// FetchPayload reads the body of the underlying http.Request and initializes
// the payload.
//
// if maxPayloadSize is a negative number, the payload is treated as a stream.
// if maxPayloadSize is zero, DefaultMaxPayloadSize is used.
func (r *Request) FetchPayload(maxPayloadSize int64) error {
	if maxPayloadSize == 0 {
		maxPayloadSize = DefaultMaxPayloadSize
	}

	stdr := r.Request

	if maxPayloadSize < 0 {
		// For an HTTP request, it is the caller's responsibility to close
		// its body, wrap it with io.NopCloser, so httpprot.Request will
		// never close it.
		r.stream = readers.NewByteCountReader(io.NopCloser(stdr.Body))
		return nil
	}

	if stdr.ContentLength > maxPayloadSize {
		return ErrRequestEntityTooLarge
	}

	if stdr.ContentLength > 0 {
		payload := make([]byte, stdr.ContentLength)
		n, err := io.ReadFull(stdr.Body, payload)
		payload = payload[:n]
		r.SetPayload(payload)
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}

	if stdr.ContentLength == 0 {
		r.SetPayload(nil)
		return nil
	}

	payload, err := io.ReadAll(io.LimitReader(stdr.Body, maxPayloadSize))
	r.SetPayload(payload)
	if err != nil {
		return err
	}

	if len(payload) < int(maxPayloadSize) {
		return nil
	}

	// try read extra bytes to check if the payload is too large.
	n, err := io.Copy(io.Discard, stdr.Body)
	if n > 0 {
		return ErrRequestEntityTooLarge
	}

	return err
}

// SetPayload set the payload of the request to payload. The payload
// could be a string, a byte slice, or an io.Reader, and if it is an
// io.Reader, it will be treated as a stream, if this is not desired,
// please read the data to a byte slice, and set the byte slice as
// the payload.
func (r *Request) SetPayload(payload interface{}) {
	r.stream = nil
	r.payload = nil

	if payload == nil {
		return
	}

	switch p := payload.(type) {
	case []byte:
		r.payload = p
	case string:
		r.payload = []byte(p)
	case io.Reader:
		if bcr, ok := p.(*readers.ByteCountReader); ok {
			r.stream = bcr
		} else {
			r.stream = readers.NewByteCountReader(p)
		}
	default:
		panic("unknown payload type")
	}
}

// GetPayload returns a payload reader. For non-stream payload, the
// returned reader is always a new one, which contains the full data.
// For stream payload, the function always returns the same reader.
func (r *Request) GetPayload() io.Reader {
	if r.stream != nil {
		return r.stream
	}
	if len(r.payload) == 0 {
		return http.NoBody
	}
	return bytes.NewReader(r.payload)
}

// RawPayload returns the payload in []byte, the caller should not
// modify its content. The function panic if the payload is a stream.
func (r *Request) RawPayload() []byte {
	if r.stream == nil {
		return r.payload
	}
	panic("the payload is a large one")
}

// PayloadSize returns the size of the payload. If the payload is a
// stream, it returns the bytes count that have been currently read
// out.
func (r *Request) PayloadSize() int64 {
	if r.stream == nil {
		return int64(len(r.payload))
	}
	return int64(r.stream.BytesRead())
}

// ToBuilderRequest wraps the request and returns the wrapper, the
// return value can be used in the template of the Builder filters.
func (r *Request) ToBuilderRequest(name string) interface{} {
	var rawBody []byte

	if r.IsStream() {
		rawBody = []byte(fmt.Sprintf("the body of request %q is a stream", name))
	} else {
		rawBody = r.RawPayload()
	}

	return &builderRequest{
		Request: r.Std(),
		rawBody: rawBody,
	}
}

// Close closes the request.
func (r *Request) Close() {
	if r.stream != nil {
		r.stream.Close()
	}
}

// MetaSize returns the meta data size of the request.
func (r *Request) MetaSize() int64 {
	// Reference: https://tools.ietf.org/html/rfc2616#section-5
	//
	// meta length is the length of:
	// w.stdr.Method + " "
	// + stdr.URL.RequestURI() + " "
	// + stdr.Proto + "\r\n",
	// + w.Header().Dump() + "\r\n\r\n"
	//
	// but to improve performance, we won't build this string

	size := len(r.Method()) + 1
	size += len(r.Std().URL.RequestURI()) + 1
	size += len(r.Proto()) + 2

	lines := 0
	for key, values := range r.HTTPHeader() {
		for _, value := range values {
			lines++
			size += len(key) + len(value)
		}
	}

	size += lines * 2 // ": "
	if lines > 1 {
		size += (lines - 1) * 2 // "\r\n"
	}

	return int64(size + 4)
}

// HTTPHeader returns the header of the request in type http.Header.
func (r *Request) HTTPHeader() http.Header {
	return r.Std().Header
}

// Header returns the header of the request in type protocols.Header.
func (r *Request) Header() protocols.Header {
	return newHeader(r.HTTPHeader())
}

// RequestScheme returns the scheme of the request.
func RequestScheme(r *http.Request) string {
	if s := r.URL.Scheme; s != "" {
		return s
	}
	if s := r.Header.Get("X-Forwarded-Proto"); s != "" {
		return s
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// Scheme returns the scheme of the request.
func (r *Request) Scheme() string {
	return RequestScheme(r.Std())
}

// RealIP returns the real IP of the request.
// TODO: if a request is cloned and modified, RealIP maybe wrong.
func (r *Request) RealIP() string {
	return r.realIP
}

// Std returns the underlying http.Request.
func (r *Request) Std() *http.Request {
	return r.Request
}

// URL returns url of the request.
func (r *Request) URL() *url.URL {
	return r.Std().URL
}

// Proto returns proto of the request.
func (r *Request) Proto() string {
	return r.Std().Proto
}

// Method returns method of the request.
func (r *Request) Method() string {
	return r.Std().Method
}

// Cookie returns the named cookie.
func (r *Request) Cookie(name string) (*http.Cookie, error) {
	return r.Std().Cookie(name)
}

// Cookies returns all cookies.
func (r *Request) Cookies() []*http.Cookie {
	return r.Std().Cookies()
}

// AddCookie add a cookie to the request.
func (r *Request) AddCookie(cookie *http.Cookie) {
	r.Std().AddCookie(cookie)
}

// Context returns the request context.
func (r *Request) Context() context.Context {
	return r.Std().Context()
}

// SetMethod sets the request method.
func (r *Request) SetMethod(method string) {
	r.Std().Method = method
}

// Host returns host of the request.
func (r *Request) Host() string {
	return r.Std().Host
}

// SetHost sets host.
func (r *Request) SetHost(host string) {
	r.Std().Host = host
}

// Path returns path.
func (r *Request) Path() string {
	return r.Std().URL.Path
}

// SetPath sets path of the request.
func (r *Request) SetPath(path string) {
	r.Std().URL.Path = path
}

// builderRequest is a wrapper of http.Request which can be used in the
// template of the Builder filters.
type builderRequest struct {
	*http.Request
	rawBody    []byte
	parsedBody interface{}
}

// RawBody returns the body as raw bytes.
func (r *builderRequest) RawBody() []byte {
	return r.rawBody
}

// Body returns the body as a string.
func (r *builderRequest) Body() string {
	return string(r.rawBody)
}

// JSONBody parses the body as a JSON object and returns the result.
// The function only parses the body if it is not already parsed.
func (r *builderRequest) JSONBody() (interface{}, error) {
	if r.parsedBody != nil {
		return r.parsedBody, nil
	}

	v, err := parseJSONBody(r.rawBody)
	r.parsedBody = v
	return v, err
}

// YAMLBody parses the body as a YAML object and returns the result.
// The function only parses the body if it is not already parsed.
// The function can only handle simple YAML objects, that is, the keys
// of the YAML object must be strings.
func (r *builderRequest) YAMLBody() (interface{}, error) {
	if r.parsedBody != nil {
		return r.parsedBody, nil
	}

	v, err := parseYAMLBody(r.rawBody)
	r.parsedBody = v
	return v, err
}

// requestInfo stores the information of a request.
type requestInfo struct {
	Method   string              `json:"method" jsonschema:"omitempty"`
	URL      string              `json:"url" jsonschema:"omitempty"`
	Headers  map[string][]string `yaml:"headers" jsonschema:"omitempty"`
	Body     string              `yaml:"body" jsonschema:"omitempty"`
	FormData map[string]field    `yaml:"formData" jsonschema:"omitempty"`
}

// field stores the information of a form field.
type field struct {
	FileName string `yaml:"fileName" jsonschema:"omitempty"`
	Value    string `yaml:"value" jsonschema:"omitempty"`
}

// NewRequestInfo returns a new requestInfo.
func (p *Protocol) NewRequestInfo() interface{} {
	return &requestInfo{}
}

// BuildRequest builds and returns a request according to the given reqInfo.
func (p *Protocol) BuildRequest(reqInfo interface{}) (protocols.Request, error) {
	ri, ok := reqInfo.(*requestInfo)
	if !ok {
		return nil, fmt.Errorf("invalid request info type: %T", reqInfo)
	}

	if ri.URL == "" {
		ri.URL = "/"
	}

	if ri.Method == "" {
		ri.Method = http.MethodGet
	} else {
		ri.Method = strings.ToUpper(ri.Method)
	}
	if _, ok := methods[ri.Method]; !ok {
		return nil, fmt.Errorf("invalid method: %s", ri.Method)
	}

	stdReq, err := http.NewRequest(ri.Method, ri.URL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create new request: %v", err)
	}

	for k, vs := range ri.Headers {
		for _, v := range vs {
			stdReq.Header.Add(k, v)
		}
	}

	req, _ := NewRequest(stdReq)
	if ri.Body != "" {
		req.SetPayload([]byte(ri.Body))
		return req, nil
	}

	if ri.FormData == nil {
		return req, nil
	}

	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)
	for name, p := range ri.FormData {
		var f io.Writer
		if p.FileName == "" {
			f, _ = form.CreateFormField(name)
		} else {
			f, _ = form.CreateFormFile(name, p.FileName)
		}
		f.Write([]byte(p.Value))
	}
	form.Close()
	req.HTTPHeader().Set("Content-Type", form.FormDataContentType())
	req.SetPayload(buf.Bytes())

	return req, nil
}

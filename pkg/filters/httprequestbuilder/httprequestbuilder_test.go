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

package requestbuilder

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitMock()
}

func getRequestBuilder(spec *Spec) *HTTPRequestBuilder {
	rb := &HTTPRequestBuilder{spec: spec}
	rb.Init()
	return rb
}

func setRequest(t *testing.T, ctx *context.Context, id string, req *http.Request) {
	r, err := httpprot.NewRequest(req)
	assert.Nil(t, err)
	ctx.SetRequest(id, r)
}

func TestMethod(t *testing.T) {
	assert := assert.New(t)

	// get method from request
	{
		spec := &Spec{
			ID:     "test",
			Method: "{{ .Requests.request1.Method }}",
		}
		rb := getRequestBuilder(spec)
		assert.True(rb.methodBuilder.useTempalte)
		defer rb.Close()

		ctx := context.New(nil)

		req1, err := http.NewRequest(http.MethodDelete, "http://www.google.com?field1=value1&field2=value2", nil)
		assert.Nil(err)
		setRequest(t, ctx, "request1", req1)

		res := rb.Handle(ctx)
		assert.Empty(res)
		testReq := ctx.GetRequest("test").(*httpprot.Request).Std()
		assert.Equal(http.MethodDelete, testReq.Method)
	}

	// set method directly
	{
		spec := &Spec{
			ID:     "test",
			Method: "get",
		}
		rb := getRequestBuilder(spec)
		assert.False(rb.methodBuilder.useTempalte)
		defer rb.Close()

		ctx := context.New(nil)

		req1, err := http.NewRequest(http.MethodDelete, "http://www.google.com?field1=value1&field2=value2", nil)
		assert.Nil(err)
		setRequest(t, ctx, "request1", req1)

		res := rb.Handle(ctx)
		assert.Empty(res)
		testReq := ctx.GetRequest("test").(*httpprot.Request).Std()
		assert.Equal(http.MethodGet, testReq.Method)
	}

	// invalid method
	{
		spec := &Spec{
			ID:     "test",
			Method: "what",
		}
		assert.Panics(func() {
			getRequestBuilder(spec)
		})
	}
}

func TestURL(t *testing.T) {
	assert := assert.New(t)

	// get url from request
	{
		spec := &Spec{
			ID:     "test",
			Method: "Delete",
			URL:    "http://www.facebook.com?field1={{index .Requests.request1.URL.Query.field2 0}}",
		}
		rb := getRequestBuilder(spec)
		assert.True(rb.urlBuilder.useTempalte)
		defer rb.Close()

		ctx := context.New(nil)

		req1, err := http.NewRequest(http.MethodDelete, "http://www.google.com?field1=value1&field2=value2", nil)
		assert.Nil(err)
		setRequest(t, ctx, "request1", req1)

		res := rb.Handle(ctx)
		assert.Empty(res)
		testReq := ctx.GetRequest("test").(*httpprot.Request).Std()
		assert.Equal(http.MethodDelete, testReq.Method)
		assert.Equal("http://www.facebook.com?field1=value2", testReq.URL.String())
	}

	// set url directly
	{
		spec := &Spec{
			ID:     "test",
			Method: "Put",
			URL:    "http://www.facebook.com",
		}
		rb := getRequestBuilder(spec)
		assert.False(rb.urlBuilder.useTempalte)
		defer rb.Close()

		ctx := context.New(nil)

		req1, err := http.NewRequest(http.MethodDelete, "http://www.google.com?field1=value1&field2=value2", nil)
		assert.Nil(err)
		setRequest(t, ctx, "request1", req1)

		res := rb.Handle(ctx)
		assert.Empty(res)
		testReq := ctx.GetRequest("test").(*httpprot.Request).Std()
		assert.Equal(http.MethodPut, testReq.Method)
		assert.Equal("http://www.facebook.com", testReq.URL.String())
	}
}

func TestHeader(t *testing.T) {
	assert := assert.New(t)

	// get header from request and response
	{
		spec := &Spec{
			ID:     "test",
			Method: "Delete",
			URL:    "http://www.facebook.com",
			Headers: []Header{
				{"X-Request", `{{index (index .Requests.request1.Header "X-Request") 0}}`},
				{"X-Response", `{{index (index .Responses.response1.Header "X-Response") 0}}`},
			},
		}
		rb := getRequestBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		req1, err := http.NewRequest(http.MethodDelete, "http://www.google.com?field1=value1&field2=value2", nil)
		assert.Nil(err)
		req1.Header.Add("X-Request", "from-request1")
		setRequest(t, ctx, "request1", req1)

		resp1 := &http.Response{}
		resp1.Header = http.Header{}
		resp1.Header.Add("X-Response", "from-response1")
		httpresp1, err := httpprot.NewResponse(resp1)
		assert.Nil(err)
		ctx.SetResponse("response1", httpresp1)

		res := rb.Handle(ctx)
		assert.Empty(res)
		testReq := ctx.GetRequest("test").(*httpprot.Request).Std()
		assert.Equal(http.MethodDelete, testReq.Method)
		assert.Equal("http://www.facebook.com", testReq.URL.String())
		assert.Equal("from-request1", testReq.Header.Get("X-Request"))
		assert.Equal("from-response1", testReq.Header.Get("X-Response"))
	}
}

func TestBody(t *testing.T) {
	assert := assert.New(t)

	// directly set body
	{
		spec := &Spec{
			ID:     "test",
			Method: "Delete",
			URL:    "http://www.facebook.com",
			Body: &BodySpec{
				Body: "body",
			},
		}
		rb := getRequestBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		res := rb.Handle(ctx)
		assert.Empty(res)
		testReq := ctx.GetRequest("test").(*httpprot.Request).Std()
		data, err := io.ReadAll(testReq.Body)
		assert.Nil(err)
		assert.Equal("body", string(data))
	}

	// set body by using other body
	{
		spec := &Spec{
			ID:     "test",
			Method: "Delete",
			URL:    "http://www.facebook.com",
			Body: &BodySpec{
				Requests: []*ReqRespBody{
					{"request1", false},
				},
				Body: "body {{ .ReqBodies.request1.Body}}",
			},
		}
		rb := getRequestBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		req1, err := http.NewRequest(http.MethodDelete, "http://www.google.com", strings.NewReader("123"))
		assert.Nil(err)
		setRequest(t, ctx, "request1", req1)

		res := rb.Handle(ctx)
		assert.Empty(res)
		testReq := ctx.GetRequest("test").(*httpprot.Request).Std()
		data, err := io.ReadAll(testReq.Body)
		assert.Nil(err)
		assert.Equal("body 123", string(data))
	}

	// set body by using other body map
	{
		spec := &Spec{
			ID:     "test",
			Method: "Delete",
			URL:    "http://www.facebook.com",
			Body: &BodySpec{
				Requests: []*ReqRespBody{
					{"request1", true},
				},
				Body: "body {{ .ReqBodies.request1.Map.field1 }} {{ .ReqBodies.request1.Map.field2 }}",
			},
		}
		rb := getRequestBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		req1, err := http.NewRequest(http.MethodDelete, "http://www.google.com", strings.NewReader(`{"field1":"value1", "field2": "value2"}`))
		assert.Nil(err)
		setRequest(t, ctx, "request1", req1)

		res := rb.Handle(ctx)
		assert.Empty(res)
		testReq := ctx.GetRequest("test").(*httpprot.Request).Std()
		data, err := io.ReadAll(testReq.Body)
		assert.Nil(err)
		assert.Equal("body value1 value2", string(data))
	}
}

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
	"net/http/httptest"
	"testing"

	json "github.com/goccy/go-json"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

func defaultFilterSpec(spec *Spec) *httppipeline.FilterSpec {
	meta := &httppipeline.FilterMetaSpec{
		Name:     "header-to-json",
		Kind:     Kind,
		Pipeline: "pipeline-demo",
	}
	filterSpec := httppipeline.MockFilterSpec(nil, nil, "", meta, spec)
	return filterSpec
}

func TestHeaderToJSON(t *testing.T) {
	assert := assert.New(t)
	h := &HeaderToJSON{}
	filterSpec := defaultFilterSpec(&Spec{})
	h.Init(filterSpec)

	assert.NotEmpty(h.Description())
	assert.Nil(h.Status())

	newh := HeaderToJSON{}
	newh.Inherit(filterSpec, h)
	newh.Close()
}

func TestHandleHTTP(t *testing.T) {
	assert := assert.New(t)
	spec := &Spec{
		HeaderMap: []*HeaderMap{
			{Header: "x-username", JSON: "username"},
		},
	}
	filterSpec := defaultFilterSpec(spec)

	h2j := HeaderToJSON{}
	h2j.Init(filterSpec)

	{
		// test http request with body
		bodyMap := map[string]interface{}{
			"topic": "log",
			"id":    "abc123",
		}
		body, err := json.Marshal(bodyMap)
		assert.Nil(err)

		req, err := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(body))
		assert.Nil(err)
		req.Header.Add("x-username", "clientA")

		w := httptest.NewRecorder()
		ctx := context.New(w, req, tracing.NoopTracing, "no trace")
		ctx.SetHandlerCaller(func(lastResult string) string {
			return lastResult
		})

		ans := h2j.Handle(ctx)
		assert.Equal("", ans)
		ctx.Finish()

		body, err = io.ReadAll(ctx.Request().Body())
		assert.Nil(err)

		res := map[string]interface{}{}
		err = json.Unmarshal(body, &res)
		assert.Nil(err)
		assert.Equal("log", res["topic"])
		assert.Equal("abc123", res["id"])
		assert.Equal("clientA", res["username"])
	}

	{
		// test http request without body
		req, err := http.NewRequest(http.MethodPost, "127.0.0.1", nil)
		assert.Nil(err)

		req.Header.Add("x-username", "clientA")
		w := httptest.NewRecorder()
		ctx := context.New(w, req, tracing.NoopTracing, "no trace")
		ctx.SetHandlerCaller(func(lastResult string) string {
			return lastResult
		})

		ans := h2j.Handle(ctx)
		assert.Equal("", ans)

		body, err := io.ReadAll(ctx.Request().Body())
		assert.Nil(err)

		res := map[string]interface{}{}
		err = json.Unmarshal(body, &res)
		assert.Nil(err)
		assert.Equal("clientA", res["username"])
	}

	{
		// test http request with array body
		bodyMap := []map[string]interface{}{
			{"log": "123", "id": "abc"},
			{"log": "456", "id": "def"},
		}
		body, err := json.Marshal(bodyMap)
		assert.Nil(err)

		req, err := http.NewRequest(http.MethodPost, "127.0.0.1", bytes.NewReader(body))
		assert.Nil(err)
		req.Header.Add("x-username", "clientA")

		w := httptest.NewRecorder()
		ctx := context.New(w, req, tracing.NoopTracing, "no trace")
		ctx.SetHandlerCaller(func(lastResult string) string {
			return lastResult
		})

		ans := h2j.Handle(ctx)
		assert.Equal("", ans)
		ctx.Finish()

		body, err = io.ReadAll(ctx.Request().Body())
		assert.Nil(err)

		res := []map[string]interface{}{}
		err = json.Unmarshal(body, &res)
		assert.Nil(err)
		for _, r := range res {
			if r["log"] == "123" {
				assert.Equal("abc", r["id"])
				assert.Equal("clientA", r["username"])

			} else if r["log"] == "456" {
				assert.Equal("def", r["id"])
				assert.Equal("clientA", r["username"])
			} else {
				t.Error("wrong result")
			}
		}
	}
}

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

package proxy

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/megaease/easegress/pkg/logger"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestAcceptGzip(t *testing.T) {
	c := newCompression(&CompressionSpec{MinLength: 100})

	req, _ := http.NewRequest(http.MethodGet, "https://megaease.com", nil)
	if !c.acceptGzip(req) {
		t.Error("accept gzip should be true")
	}

	req.Header.Add(keyAcceptEncoding, "text/text")
	if c.acceptGzip(req) {
		t.Error("accept gzip should be false")
	}

	req.Header.Add(keyAcceptEncoding, "*/*")
	if !c.acceptGzip(req) {
		t.Error("accept gzip should be true")
	}

	req.Header.Del(keyAcceptEncoding)
	req.Header.Add(keyAcceptEncoding, "gzip")
	if !c.acceptGzip(req) {
		t.Error("accept gzip should be true")
	}
}

func TestAlreadyGziped(t *testing.T) {
	c := newCompression(&CompressionSpec{MinLength: 100})

	resp := &http.Response{Header: http.Header{}}

	if c.alreadyGziped(resp) {
		t.Error("already gziped should be false")
	}

	resp.Header.Add(keyContentEncoding, "text")
	if c.alreadyGziped(resp) {
		t.Error("already gziped should be false")
	}

	resp.Header.Add(keyContentEncoding, "gzip")
	if !c.alreadyGziped(resp) {
		t.Error("already gziped should be true")
	}
}

func TestParseContentLength(t *testing.T) {
	c := newCompression(&CompressionSpec{MinLength: 100})

	resp := &http.Response{Header: http.Header{}}

	if c.parseContentLength(resp) != -1 {
		t.Error("content length should be -1")
	}

	resp.Header.Set(keyContentLength, "abc")
	if c.parseContentLength(resp) != -1 {
		t.Error("content length should be -1")
	}

	resp.Header.Set(keyContentLength, "100")
	if c.parseContentLength(resp) != 100 {
		t.Error("content length should be 100")
	}
}

func TestCompress(t *testing.T) {
	c := newCompression(&CompressionSpec{MinLength: 100})

	req, _ := http.NewRequest(http.MethodGet, "https://megaease.com", nil)
	resp := &http.Response{Header: http.Header{}}

	resp.Header.Set(keyContentLength, "20")

	rawBody := strings.Repeat("this is the raw body. ", 100)
	resp.Body = io.NopCloser(strings.NewReader(rawBody))

	c.compress(req, resp)
	if resp.Header.Get(keyContentEncoding) == "gzip" {
		t.Error("body should not be gziped")
	}

	resp.Body = http.NoBody

	resp.Header.Set(keyContentLength, "120")
	c.compress(req, resp)
	if resp.Header.Get(keyContentEncoding) != "gzip" {
		t.Error("body should be gziped")
	}
}

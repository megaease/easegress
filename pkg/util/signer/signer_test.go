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

package signer

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// We are trying best to be compatible with Amazon Signature Version 4,
// so we use Amazon literals here, and some test cases are also revised
// versions of Amazon's.
var awsSpec = &Spec{
	Literal: &Literal{
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
	HeaderHoisting: &HeaderHoisting{
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
	AccessKeys:      map[string]string{"AKID": "SECRET"},
	AccessKeyID:     "AKID",
	AccessKeySecret: "SECRET",
}

func buildRequest(serviceName, region, payload string) *http.Request {
	var body io.Reader
	body = strings.NewReader(payload)

	var bodyLen int

	type lenner interface {
		Len() int
	}
	if lr, ok := body.(lenner); ok {
		bodyLen = lr.Len()
	}

	endpoint := "https://" + serviceName + "." + region + ".amazonaws.com"
	req, _ := http.NewRequest("POST", endpoint, body)
	req.URL.Opaque = "//example.org/bucket/key-._~,!@#$%^&*()"
	req.Header.Set("X-Amz-Target", "prefix.Operation")
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")

	if bodyLen > 0 {
		req.Header.Set("Content-Length", strconv.Itoa(bodyLen))
	}

	req.Header.Set("X-Amz-Meta-Other-Header", "some-value=!@#$%^&* (+)")
	req.Header.Add("X-Amz-Meta-Other-Header_With_Underscore", "some-value=!@#$%^&* (+)")
	req.Header.Add("X-amz-Meta-Other-Header_With_Underscore", "some-value=!@#$%^&* (+)")
	req.Header.Set("X-Amz-Security-Token", "SESSION")

	return req
}

func epochTime() time.Time {
	return time.Unix(0, 0)
}

func TestBuildCanonicalHeaderValue(t *testing.T) {
	strs := []string{
		"",
		"123",
		"1 2 3",
		"1 2 3 ",
		"  1 2 3",
		"1  2 3",
		"1  23",
		"1  2  3",
		"1  2  ",
		" 1  2  ",
		"12   3",
		"12   3   1",
		"12           3     1",
		"12     3       1abc123",
	}

	expected := ",123,1 2 3,1 2 3,1 2 3,1 2 3,1 23,1 2 3,1 2,1 2,12 3,12 3 1,12 3 1,12 3 1abc123"

	if e, a := expected, buildCanonicalHeaderValue(strs); e != a {
		t.Errorf("\nexpect: %v\nactual: %v\n", e, a)
	}
}

func TestSignRequest(t *testing.T) {
	req := buildRequest("dynamodb", "us-east-1", "{}")

	expectedDate := "19700101T000000Z"
	expectedSig := "AWS4-HMAC-SHA256 Credential=AKID/19700101/us-east-1/dynamodb/aws4_request, SignedHeaders=content-length;content-type;host;x-amz-date;x-amz-meta-other-header;x-amz-meta-other-header_with_underscore;x-amz-security-token;x-amz-target, Signature=a518299330494908a70222cec6899f6f32f297f8595f6df1776d998936652ad9"

	signer := CreateFromSpec(awsSpec)
	signer.NewSigningContext(epochTime(), "us-east-1", "dynamodb").Sign(req, nil)

	q := req.Header
	if e, a := expectedSig, q.Get("Authorization"); e != a {
		t.Errorf("\nexpect: %v\nactual: %v\n", e, a)
	}
	if e, a := expectedDate, q.Get("X-Amz-Date"); e != a {
		t.Errorf("\nexpect: %v\nactual: %v\n", e, a)
	}

	if e := signer.NewVerificationContext().Verify(req, nil); e != nil {
		t.Errorf("signature verification failed: %v", e.Error())
	}
}

func TestPresignRequest(t *testing.T) {
	req := buildRequest("dynamodb", "us-east-1", "{}")

	signer := CreateFromSpec(awsSpec)
	signer.NewSigningContext(epochTime(), "us-east-1", "dynamodb").Presign(req, 300*time.Second)

	expectedDate := "19700101T000000Z"
	expectedHeaders := "content-length;content-type;host;x-amz-meta-other-header;x-amz-meta-other-header_with_underscore"
	expectedSig := "122f0b9e091e4ba84286097e2b3404a1f1f4c4aad479adda95b7dff0ccbe5581"
	expectedCred := "AKID/19700101/us-east-1/dynamodb/aws4_request"
	expectedTarget := "prefix.Operation"

	q := req.URL.Query()
	if e, a := expectedSig, q.Get("X-Amz-Signature"); e != a {
		t.Errorf("\nexpect: %v\nactual: %v\n", e, a)
	}
	if e, a := expectedCred, q.Get("X-Amz-Credential"); e != a {
		t.Errorf("\nexpect: %v\nactual: %v\n", e, a)
	}
	if e, a := expectedHeaders, q.Get("X-Amz-SignedHeaders"); e != a {
		t.Errorf("\nexpect: %v\nactual: %v\n", e, a)
	}
	if e, a := expectedDate, q.Get("X-Amz-Date"); e != a {
		t.Errorf("\nexpect: %v\nactual: %v\n", e, a)
	}
	if a := q.Get("X-Amz-Meta-Other-Header"); len(a) != 0 {
		t.Errorf("expect %v to be empty", a)
	}
	if e, a := expectedTarget, q.Get("X-Amz-Target"); e != a {
		t.Errorf("\nexpect: %v\nactual: %v\n", e, a)
	}
}

func TestPresignBodyWithArrayRequest(t *testing.T) {
	req := buildRequest("dynamodb", "us-east-1", "{}")
	req.URL.RawQuery = "Foo=z&Foo=o&Foo=m&Foo=a"

	signer := CreateFromSpec(awsSpec)
	signer.NewSigningContext(epochTime(), "us-east-1", "dynamodb").Presign(req, 300*time.Second)

	expectedDate := "19700101T000000Z"
	expectedHeaders := "content-length;content-type;host;x-amz-meta-other-header;x-amz-meta-other-header_with_underscore"
	expectedSig := "e3ac55addee8711b76c6d608d762cff285fe8b627a057f8b5ec9268cf82c08b1"
	expectedCred := "AKID/19700101/us-east-1/dynamodb/aws4_request"
	expectedTarget := "prefix.Operation"

	q := req.URL.Query()
	if e, a := expectedSig, q.Get("X-Amz-Signature"); e != a {
		t.Errorf("\nexpect: %v\nactual: %v\n", e, a)
	}
	if e, a := expectedCred, q.Get("X-Amz-Credential"); e != a {
		t.Errorf("\nexpect: %v\nactual: %v\n", e, a)
	}
	if e, a := expectedHeaders, q.Get("X-Amz-SignedHeaders"); e != a {
		t.Errorf("\nexpect: %v\nactual: %v\n", e, a)
	}
	if e, a := expectedDate, q.Get("X-Amz-Date"); e != a {
		t.Errorf("\nexpect: %v\nactual: %v\n", e, a)
	}
	if a := q.Get("X-Amz-Meta-Other-Header"); len(a) != 0 {
		t.Errorf("expect %v to be empty, was not", a)
	}
	if e, a := expectedTarget, q.Get("X-Amz-Target"); e != a {
		t.Errorf("\nexpect: %v\nactual: %v\n", e, a)
	}
}

func TestPresign_UnsignedPayload(t *testing.T) {
	req := buildRequest("service-name", "us-east-1", "hello")

	signer := CreateFromSpec(awsSpec).ExcludeBody(true)
	signer.NewSigningContext(time.Now(), "us-east-1", "service-name").Presign(req, 5*time.Minute)

	hash := req.Header.Get("X-Amz-Content-Sha256")
	if e, a := "UNSIGNED-PAYLOAD", hash; e != a {
		t.Errorf("\nexpect: %v\nactual: %v\n", e, a)
	}

	if e := signer.NewVerificationContext().Verify(req, nil); e != nil {
		t.Errorf("signature verification failed: %v", e.Error())
	}
}

func TestSignPrecomputedBodyChecksum(t *testing.T) {
	req := buildRequest("dynamodb", "us-east-1", "hello")
	req.Header.Set("X-Amz-Content-Sha256", "PRECOMPUTED")

	signer := CreateFromSpec(awsSpec)
	signer.NewSigningContext(time.Now(), "us-east-1", "dynamodb").Sign(req, nil)

	hash := req.Header.Get("X-Amz-Content-Sha256")
	if e, a := "PRECOMPUTED", hash; e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
}

func TestSignVerify(t *testing.T) {
	req := buildRequest("dynamodb", "us-east-1", "{}")

	signer := CreateFromSpec(awsSpec)
	signer.NewSigningContext(time.Now().Add(-16*time.Minute), "us-east-1", "dynamodb").Sign(req, nil)

	if e := signer.NewVerificationContext().Verify(req, nil); e != nil {
		t.Errorf("verification failed: %s", e.Error())
	}

	signer.SetTTL(15 * time.Minute)
	if e := signer.NewVerificationContext().Verify(req, nil); e == nil {
		t.Errorf("verification should failed, but didn't")
	}

	signer.SetTTL(0)
	req.Header.Set("X-Amz-Security-Token", "SESSION1")
	if e := signer.NewVerificationContext().Verify(req, nil); e == nil {
		t.Errorf("verification should failed, but didn't")
	}
}

func TestPresignVerify(t *testing.T) {
	signer := CreateFromSpec(awsSpec)

	req := buildRequest("dynamodb", "us-east-1", "{}")
	ctx := signer.NewSigningContext(time.Now().Add(-16*time.Minute), "us-east-1", "dynamodb")

	ctx.Presign(req, 10*time.Minute)
	if e := signer.NewVerificationContext().Verify(req, nil); e == nil {
		t.Errorf("verification should failed, but didn't")
	}

	ctx.Presign(req, 20*time.Minute)
	if e := signer.NewVerificationContext().Verify(req, nil); e != nil {
		t.Errorf("verification failed: %s", e.Error())
	}

	signer.SetTTL(15 * time.Minute)
	if e := signer.NewVerificationContext().Verify(req, nil); e == nil {
		t.Errorf("verification should failed, but didn't")
	}

	signer.SetTTL(0)
	req.Body = io.NopCloser(strings.NewReader("aaaaa"))
	if e := signer.NewVerificationContext().Verify(req, nil); e == nil {
		t.Errorf("verification should failed, but didn't")
	}
}

func BenchmarkPresignRequest(b *testing.B) {
	req := buildRequest("dynamodb", "us-east-1", "{}")

	signer := CreateFromSpec(awsSpec)
	for i := 0; i < b.N; i++ {
		signer.NewSigningContext(time.Now(), "us-east-1", "dynamodb").Presign(req, 300*time.Second)
	}
}

func BenchmarkSignRequest(b *testing.B) {
	req := buildRequest("dynamodb", "us-east-1", "{}")

	signer := CreateFromSpec(awsSpec)
	for i := 0; i < b.N; i++ {
		signer.NewSigningContext(time.Now(), "us-east-1", "dynamodb").Sign(req, nil)
	}
}

func BenchmarkVerify(b *testing.B) {
	req := buildRequest("dynamodb", "us-east-1", "{}")

	signer := CreateFromSpec(awsSpec)
	signer.NewSigningContext(time.Now(), "us-east-1", "dynamodb").Sign(req, nil)

	for i := 0; i < b.N; i++ {
		signer.NewVerificationContext().Verify(req, nil)
	}
}

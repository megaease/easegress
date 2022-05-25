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

package websocketserver

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpecValidate(t *testing.T) {
	tests := []struct {
		spec  *Spec
		valid bool
	}{
		{
			spec: &Spec{
				Port:    10081,
				HTTPS:   false,
				Backend: "127.0.0.1:8888",
			},
			valid: false,
		},
		{
			spec: &Spec{
				Port:    10081,
				HTTPS:   false,
				Backend: "http://127.0.0.1:8888",
			},
			valid: false,
		},
		{
			spec: &Spec{
				Port:    10081,
				HTTPS:   true,
				Backend: "ws://127.0.0.1:8888",
			},
			valid: false,
		},
		{
			spec: &Spec{
				Port:    10081,
				HTTPS:   false,
				Backend: "wss://127.0.0.1:8888",
			},
			valid: false,
		},
	}
	for _, testCase := range tests {
		err := testCase.spec.Validate()
		if testCase.valid {
			assert.Nil(t, err)
		} else {
			assert.NotNil(t, err)
		}
	}
}

// this certPem and keyPem come from golang crypto/tls/testdata
// with original name: example-cert.pem and example-key.pem
const certPem = `
-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----
`

const keyPem = `
-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----
`

func TestSpecTLS(t *testing.T) {
	cert := base64.StdEncoding.EncodeToString([]byte(certPem))
	key := base64.StdEncoding.EncodeToString([]byte(keyPem))
	spec := &Spec{
		CertBase64:    cert,
		KeyBase64:     key,
		WssCertBase64: cert,
		WssKeyBase64:  key,
	}
	_, err := spec.wssTLSConfig()
	assert.Nil(t, err)
	_, err = spec.tlsConfig()
	assert.Nil(t, err)

	spec = &Spec{}
	_, err = spec.wssTLSConfig()
	assert.NotNil(t, err)
	_, err = spec.tlsConfig()
	assert.NotNil(t, err)

	spec = &Spec{
		CertBase64:    "cert",
		KeyBase64:     "key",
		WssCertBase64: "cert",
		WssKeyBase64:  "key",
	}
	_, err = spec.wssTLSConfig()
	assert.NotNil(t, err)
	_, err = spec.tlsConfig()
	assert.NotNil(t, err)
}

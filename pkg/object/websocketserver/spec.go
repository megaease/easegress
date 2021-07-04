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
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"strings"
)

type (
	// Spec describes the HTTPServer.
	Spec struct {
		Port    uint16 `yaml:"port" jsonschema:"required,minimum=1"`
		HTTPS   bool   `yaml:"https" jsonschema:"required"`
		Backend string `yaml:"backend" jsonschema:"required"`

		CertBase64 string `yaml:"certBase64" jsonschema:"omitempty,format=base64"`
		KeyBase64  string `yaml:"keyBase64" jsonschema:"omitempty,format=base64"`

		wssCertBase64 string `yaml:"certBase64" jsonschema:"omitempty,format=base64"`
		wssKeyBase64  string `yaml:"keyBase64" jsonschema:"omitempty,format=base64"`
	}
)

// Validate validates WebSocketServerSpec.
func (spec *Spec) Validate() error {
	if spec.HTTPS {
		if len(spec.CertBase64) == 0 || len(spec.KeyBase64) == 0 {
			return fmt.Errorf("invalid certbase64 or keybase64 with https enable, spec:%#v", spec)
		}
	}

	if strings.HasPrefix(spec.Backend, "wss") {
		if len(spec.wssCertBase64) == 0 || len(spec.wssKeyBase64) == 0 {
			return fmt.Errorf("invalid wssCertbase64 or wssKeybase64 with wss enable, spec:%#v", spec)
		}
	}
	return nil
}

func (spec *Spec) wssTLSConfig() (*tls.Config, error) {
	var certificates []tls.Certificate
	if len(spec.wssCertBase64) != 0 && len(spec.wssKeyBase64) != 0 {
		certPem, _ := base64.StdEncoding.DecodeString(spec.wssCertBase64)
		keyPem, _ := base64.StdEncoding.DecodeString(spec.wssKeyBase64)
		cert, err := tls.X509KeyPair(certPem, keyPem)
		if err != nil {
			return nil, fmt.Errorf("generate x509 key pair failed: %v", err)
		}
		certificates = append(certificates, cert)
	} else {
		return nil, fmt.Errorf("wss cert/key base64 empty, certBase64: %s, keyBase64: %s", spec.wssKeyBase64, spec.wssCertBase64)
	}
	return &tls.Config{Certificates: certificates}, nil
}

func (spec *Spec) tlsConfig() (*tls.Config, error) {
	var certificates []tls.Certificate
	if spec.CertBase64 != "" && spec.KeyBase64 != "" {
		certPem, _ := base64.StdEncoding.DecodeString(spec.CertBase64)
		keyPem, _ := base64.StdEncoding.DecodeString(spec.KeyBase64)
		cert, err := tls.X509KeyPair(certPem, keyPem)
		if err != nil {
			return nil, fmt.Errorf("generate x509 key pair failed: %v", err)
		}
		certificates = append(certificates, cert)
	} else {
		return nil, fmt.Errorf("cert/key base64 empty, certBase64: %s, keyBase64: %s", spec.wssKeyBase64, spec.wssCertBase64)
	}
	return &tls.Config{Certificates: certificates}, nil
}

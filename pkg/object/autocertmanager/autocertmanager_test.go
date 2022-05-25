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

package autocertmanager

import (
	"context"
	"crypto"
	"crypto/dsa"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/libdns/libdns"
	cluster "github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/option"
	"github.com/megaease/easegress/pkg/supervisor"
	"golang.org/x/crypto/acme"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestSpecValidate(t *testing.T) {
	t.Run("no enabled challenge", func(t *testing.T) {
		yaml := `
name: autocert
kind: AutoCertManager
email: someone@megaease.com
renewBefore: 720h
enableDNS01: false
enableHTTP01: false
enableTLSALPN01: false
domains:
  - name: "*.megaease.com"
`
		_, err := supervisor.NewSpec(yaml)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
	})

	t.Run("invalid characters", func(t *testing.T) {
		yaml := `
name: autocert
kind: AutoCertManager
email: someone@megaease.com
renewBefore: 720h
enableDNS01: false
domains:
  - name: "*.meg@ease.com"
    dnsProvider:
      name: alidns
      zone: megaease.com
`
		_, err := supervisor.NewSpec(yaml)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
	})

	t.Run("wildcard without DNS01", func(t *testing.T) {
		yaml := `
name: autocert
kind: AutoCertManager
email: someone@megaease.com
renewBefore: 720h
enableDNS01: false
domains:
  - name: "*.megaease.com"
    dnsProvider:
      name: alidns
      zone: megaease.com
`
		_, err := supervisor.NewSpec(yaml)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
	})

	t.Run("unknow DNS provider", func(t *testing.T) {
		yaml := `
name: autocert
kind: AutoCertManager
email: someone@megaease.com
renewBefore: 720h
domains:
  - name: "*.megaease.com"
    dnsProvider:
      name: unknown
      zone: megaease.com
`
		_, err := supervisor.NewSpec(yaml)
		if err == nil {
			t.Errorf("spec creation should have failed")
		}
	})

	t.Run("normal", func(t *testing.T) {
		yaml := `
name: autocert
kind: AutoCertManager
email: someone@megaease.com
renewBefore: 720h
domains:
  - name: "www.megaease.com"
`
		_, err := supervisor.NewSpec(yaml)
		if err != nil {
			t.Errorf("spec creation should have succeeded: %v", err)
		}
	})
}

func TestDNSProvider(t *testing.T) {
	spec := &DomainSpec{
		Name: "www.megaease.com",
	}
	t.Run("no DNS provider", func(t *testing.T) {
		_, err := newDNSProvider(spec)
		if err == nil {
			t.Errorf("DNS provider creation should have failed")
		}
	})

	t.Run("no DNS provider name", func(t *testing.T) {
		spec.DNSProvider = map[string]string{}
		_, err := newDNSProvider(spec)
		if err == nil {
			t.Errorf("DNS provider creation should have failed")
		}
	})

	t.Run("empty DNS provider name", func(t *testing.T) {
		spec.DNSProvider["name"] = ""
		_, err := newDNSProvider(spec)
		if err == nil {
			t.Errorf("DNS provider creation should have failed")
		}
	})

	t.Run("no zone", func(t *testing.T) {
		spec.DNSProvider["name"] = "alidns"
		_, err := newDNSProvider(spec)
		if err == nil {
			t.Errorf("DNS provider creation should have failed")
		}
	})

	t.Run("unknown DNS provider", func(t *testing.T) {
		spec.DNSProvider["name"] = "unknown"
		spec.DNSProvider["zone"] = "megaease.com"
		_, err := newDNSProvider(spec)
		if err == nil {
			t.Errorf("DNS provider creation should have failed")
		}
	})

	t.Run("lack required fields", func(t *testing.T) {
		spec.DNSProvider["name"] = "alidns"
		_, err := newDNSProvider(spec)
		if err == nil {
			t.Errorf("DNS provider creation should have failed")
		}
	})

	t.Run("alidns", func(t *testing.T) {
		spec.DNSProvider["name"] = "alidns"
		spec.DNSProvider["accessKeyId"] = "accessKeyId"
		spec.DNSProvider["accessKeySecret"] = "accessKeySecret"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	t.Run("azure", func(t *testing.T) {
		spec.DNSProvider["name"] = "azure"
		spec.DNSProvider["tenantId"] = "tenantId"
		spec.DNSProvider["clientId"] = "clientId"
		spec.DNSProvider["clientSecret"] = "clientSecret"
		spec.DNSProvider["subscriptionId"] = "subscriptionId"
		spec.DNSProvider["resourceGroupName"] = "resourceGroupName"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	t.Run("cloudflare", func(t *testing.T) {
		spec.DNSProvider["name"] = "cloudflare"
		spec.DNSProvider["apiToken"] = "apiToken"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	t.Run("digitalocean", func(t *testing.T) {
		spec.DNSProvider["name"] = "digitalocean"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	t.Run("dnspod", func(t *testing.T) {
		spec.DNSProvider["name"] = "dnspod"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	t.Run("duckdns", func(t *testing.T) {
		spec.DNSProvider["name"] = "duckdns"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	/*
		t.Run("google", func(t *testing.T) {
			spec.DNSProvider["name"] = "google"
			spec.DNSProvider["project"] = "project"
			_, err := newDNSProvider(spec)
			if err != nil {
				t.Errorf("DNS provider creation should have succeeded: %v", err)
			}
		})
	*/

	t.Run("hetzner", func(t *testing.T) {
		spec.DNSProvider["name"] = "hetzner"
		spec.DNSProvider["authApiToken"] = "authApiToken"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	t.Run("route53", func(t *testing.T) {
		spec.DNSProvider["name"] = "route53"
		spec.DNSProvider["accessKeyId"] = "accessKeyId"
		spec.DNSProvider["secretAccessKey"] = "secretAccessKey"
		spec.DNSProvider["awsProfile"] = "awsProfile"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})

	t.Run("vultr", func(t *testing.T) {
		spec.DNSProvider["name"] = "vultr"
		_, err := newDNSProvider(spec)
		if err != nil {
			t.Errorf("DNS provider creation should have succeeded: %v", err)
		}
	})
}

// https://github.com/golang/crypto/blob/5e0467b6c7cee3ce8969a8b584d9e6ab01d074f7/acme/autocert/autocert_test.go#L44
var discoTmpl = template.Must(template.New("disco").Parse(`{
	"new-reg": "{{.}}/new-reg",
	"newAccount": "{{.}}/new-reg",
	"new-authz": "{{.}}/new-authz",
	"newAuthz": "{{.}}/new-authz",
	"new-cert": "{{.}}/new-cert",
	"newOrder": "{{.}}/new-cert"
}`))

// https://github.com/golang/crypto/blob/5e0467b6c7cee3ce8969a8b584d9e6ab01d074f7/acme/autocert/autocert_test.go#L170
func decodePayload(v interface{}, r io.Reader) error {
	var req struct{ Payload string }
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		return err
	}
	payload, err := base64.RawURLEncoding.DecodeString(req.Payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, v)
}

// Copied from https://github.com/golang/crypto/blob/5e0467b6c7cee3ce8969a8b584d9e6ab01d074f7/acme/autocert/autocert_test.go#L146
func dummyCert(pub interface{}, san ...string) ([]byte, error) {
	return dateDummyCert(pub, time.Now(), time.Now().Add(90*24*time.Hour), san...)
}

func dateDummyCert(pub interface{}, start, end time.Time, san ...string) ([]byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	t := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             start,
		NotAfter:              end,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageKeyEncipherment,
		DNSNames:              san,
	}
	if pub == nil {
		pub = &key.PublicKey
	}
	return x509.CreateCertificate(rand.Reader, t, t, pub, key)
}

func createChallenges(url string) map[string]interface{} {
	resp := make(map[string]interface{})
	resp["challenges"] = []struct {
		Type  string `json:"type"`
		URI   string `json:"uri"`
		Token string `json:"token"`
	}{
		{
			Type:  "http-01",
			URI:   fmt.Sprintf("%s/http-01-accepted", url),
			Token: "t01",
		},
		{
			Type:  "dns-01",
			URI:   fmt.Sprintf("%s/dns-01-accepted", url),
			Token: "t02",
		},
		{
			Type:  "tls-alpn-01",
			URI:   fmt.Sprintf("%s/tls-alpn-01-accepted", url),
			Token: "t03",
		},
		{
			Type:  "unexisting-challenge",
			URI:   "",
			Token: "",
		},
	}
	return resp
}

// Inspired by https://github.com/golang/crypto/blob/5e0467b6c7cee3ce8969a8b584d9e6ab01d074f7/acme/autocert/autocert_test.go#L477
func startACMEServerStub(
	t *testing.T,
	domain string,
	wg *sync.WaitGroup,
	acceptChallenge bool,
	acceptAuth bool) (url string, finish func()) {
	csrContainer := make([]byte, 0)
	challengeAccepted := false
	challengeStatus := "accepted"
	challengeStatusCode := http.StatusOK
	if !acceptChallenge {
		challengeStatus = "error"
		challengeStatusCode = http.StatusNotFound
	}
	// ACME CA server stub
	var ca *httptest.Server
	ca = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Replay-Nonce", "nonce")
		// Directory request.
		if r.Method == "HEAD" {
			return // a nonce request
		}
		switch r.URL.Path {
		// discovery
		case "/":
			if err := discoTmpl.Execute(w, ca.URL); err != nil {
				t.Errorf("discoTmpl: %v", err)
			}
		// client key registration
		case "/new-reg":
			w.Header().Set("Location", ca.URL+"/new-reg/1")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte("{}"))
		// cert request
		case "/new-cert":
			w.Header().Set("Location", ca.URL+"/order/1")
			w.WriteHeader(http.StatusCreated)
			challengeAccepted = false
			w.Write([]byte(
				fmt.Sprintf(`{"status": "valid", "finalize": "%s/finalize/1", "Authorizations": ["%s/authz/1"]}`,
					ca.URL, ca.URL)))
		// CA chain cert
		case "/order/1":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "valid"}`))
		case "/authz/1":
			statusCode := http.StatusOK
			if !acceptAuth {
				statusCode = http.StatusNotFound
			}
			w.WriteHeader(statusCode)
			status := "pending"
			if challengeAccepted {
				status = "valid"
			}
			resp := createChallenges(ca.URL)
			resp["status"] = status
			body, _ := json.Marshal(resp)
			w.Write(body)
			if !acceptAuth {
				wg.Done()
			}
		case "/http-01-accepted":
			challengeAccepted = true
			w.WriteHeader(challengeStatusCode)
			w.Write([]byte(fmt.Sprintf(`{"status": "%s"}`, challengeStatus)))
		case "/dns-01-accepted":
			w.WriteHeader(challengeStatusCode)
			w.Write([]byte(fmt.Sprintf(`{"status": "%s"}`, challengeStatus)))
		case "/tls-alpn-01-accepted":
			w.WriteHeader(challengeStatusCode)
			w.Write([]byte(fmt.Sprintf(`{"status": "%s"}`, challengeStatus)))
			if !acceptChallenge {
				wg.Done() // no certificates this time, stop early
			}
		case "/finalize/1":
			var req struct {
				CSR string `json:"csr"`
			}
			err := decodePayload(&req, r.Body)
			if err != nil {
				t.Errorf("base64 decoding failed: %v", err)
			}
			b, err := base64.RawURLEncoding.DecodeString(req.CSR)
			if err != nil {
				t.Errorf("base64 decoding failed: %v", err)
			}
			csr, err := x509.ParseCertificateRequest(b)
			if err != nil {
				t.Errorf("new-cert: CSR: %v", err)
			}
			der, err := dummyCert(csr.PublicKey, domain)
			if err != nil {
				t.Errorf("ca-cert: dummyCert: %v", err)
			}
			csrContainer = der
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf(`{"status": "valid", "certificate": "%s/cert/1"}`, ca.URL)))
		case "/cert/1":
			w.Header().Set("Content-Type", "application/pem-certificate-chain")
			pem.Encode(w, &pem.Block{Type: "CERTIFICATE", Bytes: csrContainer})
			wg.Done()
		default:
			t.Errorf("unrecognized r.URL.Path: %s", r.URL.Path)
		}
	}))
	finish = func() {
		ca.Close()
	}
	return ca.URL, finish
}

func helloInfo(sni string) *tls.ClientHelloInfo {
	return &tls.ClientHelloInfo{
		ServerName:   sni,
		CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305},
	}
}

type dnsProvideMock struct{}

func (dpm *dnsProvideMock) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	return nil, nil
}

func (dpm *dnsProvideMock) AppendRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	return nil, fmt.Errorf("append error")
}

func (dpm *dnsProvideMock) SetRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	return nil, nil
}

func (dpm *dnsProvideMock) DeleteRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	return nil, nil
}

func mockDBSprovider(provider string) {
	dnsProviderCreators[provider] = &dnsProviderCreator{
		requiredFields: []string{},
		creatorFn: func(d *DomainSpec) (dnsProvider, error) {
			return &dnsProvideMock{}, nil
		},
	}
}

func TestAutoCertManager(t *testing.T) {
	acmWg := &sync.WaitGroup{}
	domainCnt := 2
	acmWg.Add(domainCnt)
	mockDBSprovider("customDNS")

	url, finish := startACMEServerStub(t, "www.megaease.com", acmWg, true, true)
	defer finish()

	yaml := `
name: autocert
kind: AutoCertManager
email: someone@megaease.com
renewBefore: 720h
domains:
  - name: "www.megaease.com"
  - name: "*.megaease.com"
    dnsProvider:
      name: customDNS
      zone: megaease.com
directoryURL: ` + url
	etcdDirName, err := ioutil.TempDir("", "autocertmanager-test")
	if err != nil {
		t.Errorf(err.Error())
	}
	defer os.RemoveAll(etcdDirName)

	cls := cluster.CreateClusterForTest(etcdDirName)
	supervisor.MustNew(&option.Options{}, cls)
	spec, err := supervisor.NewSpec(yaml)
	if err != nil {
		t.Errorf("spec creation should have succeeded: %v", err)
	}

	acm := &AutoCertManager{}
	acm.Init(spec)
	acmWg.Wait()
	time.Sleep(1 * time.Second)

	if _, err := GetCertificate(helloInfo(""), false); err == nil {
		t.Errorf("GetCertificate should fail")
	}
	if _, err := GetCertificate(helloInfo("."), false); err == nil {
		t.Errorf("GetCertificate should fail")
	}
	hello := helloInfo("example.org")
	hello.SupportedProtos = []string{acme.ALPNProto}
	if _, err := GetCertificate(hello, false); err == nil {
		t.Errorf("GetCertificate should fail")
	}

	hello = helloInfo(".megaease.com")
	hello.SupportedProtos = []string{acme.ALPNProto}
	if _, err := GetCertificate(hello, false); err == nil {
		t.Errorf("GetCertificate should fail")
	}
	acm.spec.EnableTLSALPN01 = false
	if _, err := GetCertificate(hello, false); err == nil {
		t.Errorf("GetCertificate should fail")
	}
	acm.spec.EnableTLSALPN01 = true

	hello.SupportedProtos = []string{"proto"}
	if _, err := GetCertificate(hello, false); err != nil {
		t.Errorf("GetCertificate failed; %v", err.Error())
	}
	if _, err := GetCertificate(hello, true); err == nil {
		t.Errorf("GetCertificate should fail")
	}
	hello = helloInfo("unexistingdomain.io")
	hello.SupportedProtos = []string{"proto"}
	if _, err := GetCertificate(hello, false); err != nil {
		t.Errorf("GetCertificate failed; %v", err.Error())
	}

	w := httptest.NewRecorder()
	r, err := http.NewRequest("GET", "http://example.org/challenge-suffix", nil)
	if err != nil {
		t.Errorf(err.Error())
	}
	acm.spec.EnableHTTP01 = false
	HandleHTTP01Challenge(w, r)
	if !strings.Contains(w.Body.String(), "HTTP01 challenge is disabled") {
		t.Error("should be disabled")
	}
	acm.spec.EnableHTTP01 = true
	w = httptest.NewRecorder()
	HandleHTTP01Challenge(w, r)
	if !strings.Contains(w.Body.String(), `host "example.org" is not configured`) {
		t.Error("host should not exist")
	}
	// fake host
	r.Host = "*.megaease.com"
	w = httptest.NewRecorder()
	HandleHTTP01Challenge(w, r)
	if !strings.Contains(w.Body.String(), `token does not exist`) {
		t.Error("token should not exist")
	}

	key := "autocert/http/*.megaease.com//challenge-suffix"
	token := "asdlijasdoiashvouid"
	err = cls.Put(key, token) // add data for http01 challenge
	if err != nil {
		t.Errorf(err.Error())
	}

	w = httptest.NewRecorder()
	HandleHTTP01Challenge(w, r)
	if !strings.Contains(w.Body.String(), token) {
		t.Error("token should exist")
	}

	if len(acm.Status().ObjectStatus.(*Status).Domains) != 2 {
		t.Error("bad status")
	}
	if acm.spec.Domains[0].Zone() != "" {
		t.Error("bad status")
	}

	acm.Close()

	// Test inherit
	acm = &AutoCertManager{}
	acm.Inherit(spec, acm)
	acm.Close()

	closeWG := &sync.WaitGroup{}
	closeWG.Add(1)
	cls.CloseServer(closeWG)
	closeWG.Wait()
	time.Sleep(100 * time.Millisecond)
}

func TestAutoCertManagerChallengeFailures(t *testing.T) {
	acmWg := &sync.WaitGroup{}
	domainCnt := 2
	acmWg.Add(domainCnt)

	mockDBSprovider("customDNS")

	url, finish := startACMEServerStub(t, "www.megaease.com", acmWg, false, true)
	defer finish()

	yaml := `
name: autocert
kind: AutoCertManager
email: someone@megaease.com
renewBefore: 720h
domains:
  - name: "notexisting.com"
  - name: "*.megaease.com"
    dnsProvider:
      name: customDNS
      zone: megaease.com
directoryURL: ` + url
	etcdDirName, err := ioutil.TempDir("", "autocertmanager-test")
	if err != nil {
		t.Errorf(err.Error())
	}
	defer os.RemoveAll(etcdDirName)

	cls := cluster.CreateClusterForTest(etcdDirName)
	supervisor.MustNew(&option.Options{}, cls)
	spec, err := supervisor.NewSpec(yaml)
	if err != nil {
		t.Errorf("spec creation should have succeeded: %v", err)
	}

	acm := &AutoCertManager{}
	acm.Init(spec)
	acmWg.Wait()
	time.Sleep(100 * time.Millisecond)

	acm.Close()

	closeWG := &sync.WaitGroup{}
	closeWG.Add(1)
	cls.CloseServer(closeWG)
	closeWG.Wait()
}

func TestAutoCertManagerNoAuthz(t *testing.T) {
	acmWg := &sync.WaitGroup{}
	domainCnt := 2
	acmWg.Add(domainCnt)

	mockDBSprovider("customDNS")

	url, finish := startACMEServerStub(t, "www.megaease.com", acmWg, false, false)
	defer finish()

	yaml := `
name: autocert
kind: AutoCertManager
email: someone@megaease.com
renewBefore: 720h
domains:
  - name: "notexisting.com"
  - name: "*.megaease.com"
    dnsProvider:
      name: customDNS
      zone: megaease.com
directoryURL: ` + url
	etcdDirName, err := ioutil.TempDir("", "autocertmanager-test")
	if err != nil {
		t.Errorf(err.Error())
	}
	defer os.RemoveAll(etcdDirName)

	cls := cluster.CreateClusterForTest(etcdDirName)
	supervisor.MustNew(&option.Options{}, cls)

	spec, err := supervisor.NewSpec(yaml)
	if err != nil {
		t.Errorf("spec creation should have succeeded: %v", err)
	}

	acm := &AutoCertManager{}
	acm.Init(spec)
	acmWg.Wait()
	time.Sleep(100 * time.Millisecond)

	acm.Close()

	closeWG := &sync.WaitGroup{}
	closeWG.Add(1)
	cls.CloseServer(closeWG)
	closeWG.Wait()
}

func TestAutoCertManagerNotRunning(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleHTTP01Challenge(w, r)
		wg.Done()
	}))
	defer ts.Close()
	_, err := http.Get(ts.URL)
	if err != nil {
		t.Errorf(err.Error())
	}
	wg.Wait()
	GetCertificate(helloInfo("example.org"), false)
}

func TestCertificateHelpers(t *testing.T) {
	keys := make([]crypto.PrivateKey, 4)
	success := []bool{true, true, false, true}

	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	keys[0] = rsaKey
	ecdsaKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	keys[1] = ecdsaKey
	dsaKey := &dsa.PrivateKey{}
	_ = dsa.GenerateKey(dsaKey, rand.Reader)
	keys[2] = dsaKey
	rsaKey2, _ := rsa.GenerateKey(rand.Reader, 2048)
	rsaKey2.D = nil
	keys[3] = rsaKey2
	for i := 0; i < len(keys); i++ {
		cert := &tls.Certificate{
			PrivateKey:  keys[i],
			Certificate: nil,
			Leaf:        nil,
		}
		encodedCert, err := encodeCertificate(cert)
		if success[i] {
			if err != nil {
				t.Errorf("encode cert failed")
			}
		} else {
			if err == nil {
				t.Errorf("encode should fail")
			}
		}
		if _, err := decodeCertificate(encodedCert); err == nil {
			t.Errorf("decode should fail") // all certs missing public key
		}
	}

	der := make([]byte, 10)
	if _, err := parsePrivateKey(der); err == nil {
		t.Errorf("parsePrivateKey should fail")
	}

	id := acme.AuthzID{Value: "dnsName"}
	csr, certkey, err := newCSR(id)
	if err != nil {
		t.Errorf("newCSR failed %v", err)
	}
	certDer, _ := dummyCert(csr, "example.org")
	if err != nil {
		t.Errorf("dummyCert failed %v", err)
	}

	var unvalidDer [][]byte
	unvalidDer = append(unvalidDer, certDer)
	if _, err := validCert(unvalidDer, certkey); err == nil {
		t.Errorf("validCert should fail")
	}
}

func waitDNSRecordTest(t *testing.T, d Domain) {
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("waitDNSRecord should have failed")
		}
	}()
	d.waitDNSRecord("")
}

func TestDomain(t *testing.T) {
	etcdDirName, err := ioutil.TempDir("", "autocertmanager-domain-test")
	if err != nil {
		t.Errorf(err.Error())
	}
	defer os.RemoveAll(etcdDirName)
	cls := cluster.CreateClusterForTest(etcdDirName)

	t.Run("waitDNSRecord", func(t *testing.T) {
		d := Domain{}
		waitDNSRecordTest(t, d)
		d = Domain{
			nameInPunyCode: "name",
		}
		waitDNSRecordTest(t, d)
		d = Domain{
			nameInPunyCode: "*.name",
		}
		waitDNSRecordTest(t, d)
	})

	t.Run("renewCert", func(t *testing.T) {
		var ca *httptest.Server
		ca = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Replay-Nonce", "nonce")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("{}"))
		}))

		fakeAcm := &AutoCertManager{}
		fakeAcm.stopCtx, fakeAcm.cancel = context.WithCancel(context.Background())
		key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

		acmWg := &sync.WaitGroup{}
		acmWg.Add(1)
		fakeAcm.client = &acme.Client{Key: key, DirectoryURL: ca.URL}
		d := Domain{nameInPunyCode: "name"}
		if err := d.renewCert(fakeAcm); err == nil {
			t.Errorf("should have failed")
		}
		ca.Close()
	})

	t.Run("runHTTP01", func(t *testing.T) {
		fakeAcm := &AutoCertManager{}
		fakeAcm.stopCtx, fakeAcm.cancel = context.WithCancel(context.Background())
		key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		fakeAcm.client = &acme.Client{Key: key, DirectoryURL: ""}
		chal := &acme.Challenge{Token: "asdoilj"}
		fakeAcm.storage = &storage{cls: cls}
		// let's close the cluster to make runHTTP01 fail
		closeWG := &sync.WaitGroup{}
		closeWG.Add(1)
		cls.CloseServer(closeWG)
		closeWG.Wait()
		d := Domain{nameInPunyCode: "name"}
		if err := d.runHTTP01(fakeAcm, chal); err == nil {
			t.Errorf("should have failed")
		}
	})

	t.Run("runTLSALPN01", func(t *testing.T) {
		fakeAcm := &AutoCertManager{}
		fakeAcm.stopCtx, fakeAcm.cancel = context.WithCancel(context.Background())
		key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		fakeAcm.client = &acme.Client{Key: key, DirectoryURL: ""}
		chal := &acme.Challenge{Token: "asdoilj"}
		auth := &acme.Authorization{Identifier: acme.AuthzID{Value: "authzID"}}
		fakeAcm.storage = &storage{cls: cls}
		// let's close the cluster to make runTLSALPN01 fail
		closeWG := &sync.WaitGroup{}
		closeWG.Add(1)
		cls.CloseServer(closeWG)
		closeWG.Wait()
		d := Domain{nameInPunyCode: "name"}
		if err := d.runTLSALPN01(fakeAcm, auth, chal); err == nil {
			t.Errorf("should have failed")
		}
	})
}

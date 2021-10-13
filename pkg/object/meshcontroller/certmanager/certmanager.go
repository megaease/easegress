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

package certmanager

import (
	"reflect"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
)

const (
	typeCert                     = "CERTIFICATE"
	typeKey                      = "RSA PRIVATE KEY"
	defaultRootCertCountry       = "cn"
	defaultRootCertLocality      = "beijing"
	defaultRootCertOrganization  = "megaease"
	defaultRsaBits               = 2046
	defaultSerialNumber          = 202100
	defaultIngressControllerName = "mesh-deafult-ingress-controller"
)

type (

	// CertManager manages the mesh-wide mTLS cert/keys's refreshing, storing into local Etcd.
	CertManager struct {
		Provider    CertProvider
		service     *service.Service
		appCertTTL  time.Duration
		rootCertTTL time.Duration
	}

	// CertProvider is the interface declaring the methods for the Certificate provider, such as
	//   easemesh-self-Sign, Valt, and so on.
	CertProvider interface {
		// SignAppCertAndKey  Signs a cert, key pair for one service
		SignAppCertAndKey(serviceName string, ttl time.Duration) (cert *spec.Certificate, err error)

		// SignRootCertAndKey Signs a cert, key pair for root
		SignRootCertAndKey(time.Duration) (cert *spec.Certificate, err error)

		// GetAppCertAndKey get cert and key for one service
		GetAppCertAndKey(serviceName string) (cert *spec.Certificate, err error)

		// GetRootCertAndKey get root ca cert and key
		GetRootCertAndKey() (cert *spec.Certificate, err error)

		// ReleaseAppCertAndKey releases one service's cert and key
		ReleaseAppCertAndKey(serviceName string) error

		// ReleaseRootCertAndKey releases root CA cert and key
		ReleaseRootCertAndKey() error

		// SetRootCertAndKey sets exists app cert
		SetAppCertAndKey(serviceName string, cert *spec.Certificate) error

		// SetRootCertAndKey sets exists root cert into provider
		SetRootCertAndKey(cert *spec.Certificate) error
	}
)

// NewCertManager creates a initialed certmanager.
func NewCertManager(service *service.Service, certProviderType string, appCertTTL, rootCertTTL time.Duration) *CertManager {
	certManager := &CertManager{
		service:     service,
		appCertTTL:  appCertTTL,
		rootCertTTL: rootCertTTL,
	}

	switch certProviderType {
	case spec.CertProviderSelfSign:
		fallthrough
	default:
		certManager.Provider = NewMeshCertProvider()
	}

	go certManager.init()
	return certManager
}

// sign root/ingress/all services certs
func (cm *CertManager) init() {
	err := cm.SignRootCert()
	if err != nil {
		logger.Errorf("certmanager sign root cert failed: %v", err)
		return
	}

	serviceSpecs := cm.service.ListServiceSpecs()
	err = cm.SignServices(serviceSpecs)
	if err != nil {
		logger.Errorf("certmanager sign all service failed: %v", err)
		return
	}
}

// CleanAllCerts cleans all exist cert records in Mesh Etcd.
func (cm *CertManager) CleanAllCerts() error {
	rootCert := cm.service.GetRootCert()
	if rootCert != nil {
		cm.service.DelRootCert()
		cm.Provider.ReleaseRootCertAndKey()
	}

	serviceCerts := cm.service.ListServiceCerts()
	for _, v := range serviceCerts {
		if v != nil {
			cm.service.DeleteServiceCert(v.ServiceName)
			cm.Provider.ReleaseAppCertAndKey(v.ServiceName)
		}
	}
	return nil
}

// needSign will check the cert's TTL, if it's expired, then return true.
// also, if some certs' formats are incorrect, then it will return true for resigning.
func (cm *CertManager) needSign(cert *spec.Certificate) bool {
	if cert == nil {
		return true
	}
	timeNow := time.Now()

	signTime, err := time.Parse(time.RFC3339, cert.SignTime)
	if err != nil {
		logger.Errorf("service: %s has invalid sign time: %s, err: %v, need to resign", cert.ServiceName, cert.SignTime, err)
		return true
	}
	gap := timeNow.Sub(signTime)
	ttl, err := time.ParseDuration(cert.TTL)
	if err != nil {
		logger.Errorf("service: %s has invalid cert ttl: %s, err: %v, need to resign", cert.ServiceName, cert.TTL, err)
		return true
	}

	// expired, need resign
	if gap > ttl {
		logger.Infof("service: %s need to resign cert, gap: %s, need to resign", cert.ServiceName, gap.String())
		return true
	}

	return false
}

// SignRootCert signs the root cert, once the root cert had been resigned
// it will cause the whole system's application certs to be resigned.
func (cm *CertManager) SignRootCert() error {
	var err error
	rootCert := cm.service.GetRootCert()

	if cm.needSign(rootCert) {
		rootCert, err = cm.Provider.SignRootCertAndKey(cm.rootCertTTL)
		if err != nil {
			return err
		}
		cm.service.PutRootCert(rootCert)
		cm.ForceSignAllServices()
	} else {
		// set cert from Etcd to provider manually
		if providerCert, err := cm.Provider.GetRootCertAndKey(); err != nil || !reflect.DeepEqual(providerCert, rootCert) {
			cm.Provider.SetRootCertAndKey(rootCert)
			cm.ForceSignAllServices()
		}
	}
	return nil
}

// SignIngressController signs ingress controller's cert.
func (cm *CertManager) SignIngressController() error {
	var err error
	cert := cm.service.GetIngressControllerCert()

	if cm.needSign(cert) {
		cert, err = cm.Provider.SignAppCertAndKey(defaultIngressControllerName, cm.appCertTTL)
		if err != nil {
			return err
		}
		cm.service.PutIngressControllerCert(cert)
	} else {
		// set cert from Etcd to provider manually
		if providerCert, err := cm.Provider.GetAppCertAndKey(defaultIngressControllerName); err != nil || !reflect.DeepEqual(providerCert, cert) {
			cm.Provider.SetAppCertAndKey(defaultIngressControllerName, cert)
		}
	}
	return nil
}

// ForceSignAllServices resigns all services inside mesh regradless it's expired or not.
func (cm *CertManager) ForceSignAllServices() {
	serviceSpecs := cm.service.ListServiceSpecs()
	for _, v := range serviceSpecs {
		newCert, err := cm.Provider.SignAppCertAndKey(v.Name, cm.appCertTTL)
		if err != nil {
			logger.Errorf("service: %s sign cert failed, err: %v", v.Name, err)
			continue
		}

		cm.service.PutServiceCert(newCert)
	}

	cert, err := cm.Provider.SignAppCertAndKey(defaultIngressControllerName, cm.appCertTTL)
	if err != nil {
		logger.Errorf("sign ingress controller cert failed, err: %v", err)
		return
	}
	cm.service.PutIngressControllerCert(cert)
}

// SignServices signs all services' cert in mesh.
func (cm *CertManager) SignServices(serviceSpecs []*spec.Service) error {
	var needSignServer []string
	for _, v := range serviceSpecs {
		originCert := cm.service.GetServiceCert(v.Name)
		if originCert == nil {
			needSignServer = append(needSignServer, v.Name)
			continue
		}

		if cm.needSign(originCert) {
			needSignServer = append(needSignServer, v.Name)
			continue
		}

		if providerCert, err := cm.Provider.GetAppCertAndKey(v.Name); err != nil || !reflect.DeepEqual(originCert, providerCert) {
			// correct the provider's cert value according to Mesh Etcd's
			cm.Provider.SetAppCertAndKey(v.Name, originCert)
		}
	}

	for _, v := range needSignServer {
		newCert, err := cm.Provider.SignAppCertAndKey(v, cm.appCertTTL)
		if err != nil {
			logger.Errorf("%s sign cert failed, err: %v", v)
			continue
		}

		cm.service.PutServiceCert(newCert)
	}

	// sign ingress controller
	if err := cm.SignIngressController(); err != nil {
		return err
	}
	return nil
}

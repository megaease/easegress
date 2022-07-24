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

package cluster

import "fmt"

// Cluster store tree layout.
// Status means dynamic, different in every member.
// Config means static, same in every member.
const (
	// NamespaceDefault is the default namespace of object.
	NamespaceDefault = "default"
	// NamespaceSystemPrefix is the prefix of system namespace.
	// all namespaces started with this prefix is reserved for system.
	// The users should avoid creating namespaces started with this prefix.
	NamespaceSystemPrefix  = "eg-"
	NamespacetrafficPrefix = "eg-traffic-"

	leaseFormat          = "/leases/%s" //+memberName
	statusMemberPrefix   = "/status/members/"
	statusMemberFormat   = "/status/members/%s" // +memberName
	statusObjectPrefix   = "/status/objects/"
	statusObjectFormat   = "/status/objects/%s/%s/%s" // +namespace +objectName +memberName
	configObjectPrefix   = "/config/objects/"
	configObjectFormat   = "/config/objects/%s" // +objectName
	configVersion        = "/config/version"
	wasmCodeEvent        = "/wasm/code"
	wasmDataPrefixFormat = "/wasm/data/%s/%s/" // + pipelineName + filterName
	customDataKindPrefix = "/custom-data-kinds/"
	customDataPrefix     = "/custom-data/"

	// the cluster name of this eg group will be registered under this path in etcd
	// any new member(primary or secondary ) will be rejected if it is configured a different cluster name
	clusterNameKey = "/eg/cluster/name"
)

type (
	// Layout represents storage tree layout.
	Layout struct {
		memberName string
	}
)

// SystemNamespace returns the system namespace.
func SystemNamespace(name string) string {
	return NamespaceSystemPrefix + name
}

// TrafficNamespace returns the traffic namespace.
func TrafficNamespace(name string) string {
	return NamespacetrafficPrefix + name
}

func (c *cluster) initLayout() {
	c.layout = &Layout{
		memberName: c.opt.Name,
	}
}

// Layout returns cluster layout.
func (c *cluster) Layout() *Layout {
	return c.layout
}

// ClusterNameKey returns the key of the cluster name.
func (l *Layout) ClusterNameKey() string {
	return clusterNameKey
}

// Lease returns the key of own member lease.
func (l *Layout) Lease() string {
	return fmt.Sprintf(leaseFormat, l.memberName)
}

// OtherLease returns the key of the given member lease.
func (l *Layout) OtherLease(memberName string) string {
	return fmt.Sprintf(leaseFormat, memberName)
}

// StatusMemberPrefix returns the prefix of member status.
func (l *Layout) StatusMemberPrefix() string {
	return statusMemberPrefix
}

// StatusMemberKey returns the key of own member status.
func (l *Layout) StatusMemberKey() string {
	return fmt.Sprintf(statusMemberFormat, l.memberName)
}

// OtherStatusMemberKey returns the key of given member status.
func (l *Layout) OtherStatusMemberKey(memberName string) string {
	return fmt.Sprintf(statusMemberFormat, memberName)
}

// StatusObjectsPrefix returns the prefix of objects status.
func (l *Layout) StatusObjectsPrefix() string {
	return statusObjectPrefix
}

// StatusObjectPrefix returns the prefix of object status.
func (l *Layout) StatusObjectPrefix(name string) string {
	return statusObjectPrefix
}

// StatusObjectKey returns the key of object status.
func (l *Layout) StatusObjectKey(namespace, name string) string {
	return fmt.Sprintf(statusObjectFormat, namespace, name, l.memberName)
}

// ConfigObjectPrefix returns the prefix of object config.
func (l *Layout) ConfigObjectPrefix() string {
	return configObjectPrefix
}

// ConfigObjectKey returns the key of object config.
func (l *Layout) ConfigObjectKey(name string) string {
	return fmt.Sprintf(configObjectFormat, name)
}

// ConfigVersion returns the key of config version.
func (l *Layout) ConfigVersion() string {
	return configVersion
}

// WasmCodeEvent returns the key of wasm code event
func (l *Layout) WasmCodeEvent() string {
	return wasmCodeEvent
}

// WasmDataPrefix returns the prefix of wasm data
func (l *Layout) WasmDataPrefix(pipeline string, name string) string {
	return fmt.Sprintf(wasmDataPrefixFormat, pipeline, name)
}

// CustomDataPrefix returns the prefix of all custom data
func (l *Layout) CustomDataPrefix() string {
	return customDataPrefix
}

// CustomDataKindPrefix returns the prefix of all custom data kind
func (l *Layout) CustomDataKindPrefix() string {
	return customDataKindPrefix
}

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

package label

const (
	// KeyRole is the key of role
	KeyRole = "mesh-role"
	// KeyServiceName is the key of service name
	KeyServiceName = "mesh-service-name"
	// KeyServiceLabels is the key of service label
	KeyServiceLabels = "mesh-service-labels"
	// KeyApplicationPort is the key of application port
	KeyApplicationPort = "mesh-application-port"
	// KeyAliveProbe is the key of keepalive probe
	KeyAliveProbe = "mesh-alive-probe"

	// ValueRoleMaster is the name of master
	ValueRoleMaster = "master"
	// ValueRoleWorker is the name of worker
	ValueRoleWorker = "worker"
	// ValueRoleIngressController is the name of ingress controller
	ValueRoleIngressController = "ingress-controller"
)

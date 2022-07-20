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

package command

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	yamljsontool "github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type (
	// GlobalFlags is the global flags for the whole client.
	GlobalFlags struct {
		URL          string
		OutputFormat string
	}

	// APIErr is the standard return of error.
	APIErr struct {
		Code    int    `yaml:"code"`
		Message string `yaml:"message"`
	}
)

// CommandlineGlobalFlags is the singleton of GlobalFlags.
var CommandlineGlobalFlags GlobalFlags

const (
	apiURL = "/apis/v1"

	healthURL = apiURL + "/healthz"

	membersURL = apiURL + "/status/members"
	memberURL  = apiURL + "/status/members/%s"

	objectKindsURL = apiURL + "/object-kinds"
	objectsURL     = apiURL + "/objects"
	objectURL      = apiURL + "/objects/%s"

	statusObjectURL  = apiURL + "/status/objects/%s"
	statusObjectsURL = apiURL + "/status/objects"

	wasmCodeURL = apiURL + "/wasm/code"
	wasmDataURL = apiURL + "/wasm/data/%s/%s"

	customDataKindURL     = apiURL + "/customdatakinds"
	customDataKindItemURL = apiURL + "/customdatakinds/%s"
	customDataURL         = apiURL + "/customdata/%s"
	customDataItemURL     = apiURL + "/customdata/%s/%s"

	profileURL      = apiURL + "/profile"
	profileStartURL = apiURL + "/profile/start/%s"
	profileStopURL  = apiURL + "/profile/stop"

	// MeshTenantsURL is the mesh tenant prefix.
	MeshTenantsURL = apiURL + "/mesh/tenants"

	// MeshTenantURL is the mesh tenant path.
	MeshTenantURL = apiURL + "/mesh/tenants/%s"

	// MeshServicesURL is mesh service prefix.
	MeshServicesURL = apiURL + "/mesh/services"

	// MeshServiceURL is the mesh service path.
	MeshServiceURL = apiURL + "/mesh/services/%s"

	// MeshServiceCanaryURL is the mesh service canary path.
	MeshServiceCanaryURL = apiURL + "/mesh/services/%s/canary"

	// MeshServiceResilienceURL is the mesh service resilience path.
	MeshServiceResilienceURL = apiURL + "/mesh/services/%s/resilience"

	// MeshServiceLoadBalanceURL is the mesh service load balance path.
	MeshServiceLoadBalanceURL = apiURL + "/mesh/services/%s/loadbalance"

	// MeshServiceOutputServerURL is the mesh service output server path.
	MeshServiceOutputServerURL = apiURL + "/mesh/services/%s/outputserver"

	// MeshServiceTracingsURL is the mesh service tracings path.
	MeshServiceTracingsURL = apiURL + "/mesh/services/%s/tracings"

	// MeshServiceMetricsURL is the mesh service metrics path.
	MeshServiceMetricsURL = apiURL + "/mesh/services/%s/metrics"

	// MeshServiceInstancesURL is the mesh service prefix.
	MeshServiceInstancesURL = apiURL + "/mesh/serviceinstances"

	// MeshServiceInstanceURL is the mesh service path.
	MeshServiceInstanceURL = apiURL + "/mesh/serviceinstances/%s/%s"

	// MeshIngressesURL is the mesh ingress prefix.
	MeshIngressesURL = apiURL + "/mesh/ingresses"

	// MeshIngressURL is the mesh ingress path.
	MeshIngressURL = apiURL + "/mesh/ingresses/%s"
)

func makeURL(urlTemplate string, a ...interface{}) string {
	return CommandlineGlobalFlags.URL + fmt.Sprintf(urlTemplate, a...)
}

func successfulStatusCode(code int) bool {
	return code >= 200 && code < 300
}

func handleRequest(httpMethod string, url string, reqBody []byte, cmd *cobra.Command) {
	req, err := http.NewRequest(httpMethod, url, bytes.NewReader(reqBody))
	if err != nil {
		ExitWithError(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		ExitWithErrorf("%s failed: %v", cmd.Short, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ExitWithErrorf("%s failed: %v", cmd.Short, err)
	}

	if !successfulStatusCode(resp.StatusCode) {
		msg := string(body)
		apiErr := &APIErr{}
		err = yaml.Unmarshal(body, apiErr)
		if err == nil {
			msg = apiErr.Message
		}
		ExitWithErrorf("%d: %s", apiErr.Code, msg)
	}

	if len(body) != 0 {
		printBody(body)
	}
}

func printBody(body []byte) {
	var output []byte
	switch CommandlineGlobalFlags.OutputFormat {
	case "yaml":
		output = body
	case "json":
		var err error
		output, err = yamljsontool.YAMLToJSON(body)
		if err != nil {
			ExitWithErrorf("yaml %s to json failed: %v", body, err)
		}
	}

	fmt.Printf("%s", output)
}

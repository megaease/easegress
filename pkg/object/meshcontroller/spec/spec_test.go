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

package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/megaease/easegress/pkg/filters/mock"
	"github.com/megaease/easegress/pkg/filters/proxy"
	"github.com/megaease/easegress/pkg/filters/ratelimiter"
	"github.com/megaease/easegress/pkg/logger"
	_ "github.com/megaease/easegress/pkg/object/httpserver"
	"github.com/megaease/easegress/pkg/resilience"
	"github.com/megaease/easegress/pkg/util/urlrule"
	v2alpha1 "github.com/megaease/easemesh-api/v2alpha1"
	"gopkg.in/yaml.v2"
)

var (
	RootCertBase64 = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURmVENDQW1XZ0F3SUJBZ0lEQXhWME1BMEdDU3FHU0liM0RRRUJDd1VBTUVreEN6QUpCZ05WQkFZVEFtTnUKTVJBd0RnWURWUVFIRXdkaVpXbHFhVzVuTVJFd0R3WURWUVFLRXdodFpXZGhaV0Z6WlRFVk1CTUdBMVVFQXhNTQpiV1Z6YUMxeWIyOTBMV05oTUI0WERUSXhNVEF4TkRBMU5EQTFPRm9YRFRNeE1UQXhNakExTkRBMU9Gb3dTVEVMCk1Ba0dBMVVFQmhNQ1kyNHhFREFPQmdOVkJBY1RCMkpsYVdwcGJtY3hFVEFQQmdOVkJBb1RDRzFsWjJGbFlYTmwKTVJVd0V3WURWUVFERXd4dFpYTm9MWEp2YjNRdFkyRXdnZ0VoTUEwR0NTcUdTSWIzRFFFQkFRVUFBNElCRGdBdwpnZ0VKQW9JQkFEMmxSeWNrSDRIRngyQVVSeVVwLy8rakx4c2tKT3BkVTRiNlpwSnZZbXlkYlRkTFg5MVFWeWh4CmdGby95ZzJsdG9FVjV4Y2hZL2RTZmEreFpyTGx3dkZ4TG9tcjRFdVdRZFMrQnBWZjVwd2RRd2pLMHNDcjhYaXYKSVREMmp4dExmZkpqNUhOQVM0eWhWUXhPMjJtM1o2ZXFwbVpBV3lCSjVrYjRFY3QyZGcrQnlPY1UyMzNSM3JKMQpWTkNpLzZCM1NWOVY5WmZ6OER6Y3BZRG9qMEhTRTJIM2hOTFU4VW1YRnBDaWk1ei9IRkN4bW15ckZFRGdCdkovCmxUS0RadGIzZzRBdWdiSkM3Mng0UmhjWUcwYjVPTXJUampKNnhhKzFBL2lGSlJoUXQ2cHQydzhNcVN3QnFjRXoKUmZhZXNEZ2hHNG5DNHhxbHErQzc0R0U3Q1hrdEFnc0NBd0VBQWFOdk1HMHdEZ1lEVlIwUEFRSC9CQVFEQWdFRwpNQjBHQTFVZEpRUVdNQlFHQ0NzR0FRVUZCd01DQmdnckJnRUZCUWNEQVRBUEJnTlZIUk1CQWY4RUJUQURBUUgvCk1CMEdBMVVkRGdRV0JCUmRoZi9oL2NWWHlvQncxWmI4bmhWc0FkUjRiekFNQmdOVkhSRUVCVEFEZ2dFcU1BMEcKQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUFJOXVXK1ZMTGl0Y3JKQzk3VFhzOE5MMUxWV3JqakFORXMyOXJrenFubgpMQjBGQ0tpdjB3TGN5WlExU0hvd0U0a0dSMnp3SjRGMmxyOGpMTWF5bzdoY1k1M0d3VUFkRXJVU3R6dGFqV2FlCkRmRFJYWUtoSW12NGY1TDFNSEo1eE96TUZFeW9oK3RZbi81THhUS3VZU1lGMnhuY3RodnUwSHNqOXhtNTVNR2QKZzJYRytLN0ZFNUpaN2RQdm1OT2pub1pDTTd1NEpVV0N3eE8rZmFrWk9qYU5wNzlTNTF4TlFuTG5CK3ZlbExiWAp0WU1hSk0yN1ZaWmFFKzV2Mm56OVZxVlFSZ0tQLzNjdXMyNXhLemhHY1Ewc1hLdWZ0Mm8yNnNHamowbmRwemozCmZKQnIzWDBKMFF1bjUxSUtxZURWZHBiMHJTcFlNblE1WWh0YXdSR1M3WG95Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K"
	RootKeyBase64  = "LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFbndJQkFBS0NBUUE5cFVjbkpCK0J4Y2RnRkVjbEtmLy9veThiSkNUcVhWT0crbWFTYjJKc25XMDNTMS9kClVGY29jWUJhUDhvTnBiYUJGZWNYSVdQM1VuMnZzV2F5NWNMeGNTNkpxK0JMbGtIVXZnYVZYK2FjSFVNSXl0TEEKcS9GNHJ5RXc5bzhiUzMzeVkrUnpRRXVNb1ZVTVR0dHB0MmVucXFabVFGc2dTZVpHK0JITGRuWVBnY2puRk50OQowZDZ5ZFZUUW92K2dkMGxmVmZXWDgvQTgzS1dBNkk5QjBoTmg5NFRTMVBGSmx4YVFvb3VjL3h4UXNacHNxeFJBCjRBYnlmNVV5ZzJiVzk0T0FMb0d5UXU5c2VFWVhHQnRHK1RqSzA0NHllc1d2dFFQNGhTVVlVTGVxYmRzUERLa3MKQWFuQk0wWDJuckE0SVJ1Snd1TWFwYXZndStCaE93bDVMUUlMQWdNQkFBRUNnZ0VBRXlqWWFZam5wZnpqajdBZAp3S1pDSTZFRFZnc3cwZ3E1bUQwaFBpZ1NUakhMclNEbkpiRC90ZGs1REZQQkorYTJSMzZZT1c4dVU4TTJ2ekdDCit0MUFicXcveTVnNCtTVTFScnJjN3ZaRWhZYnV1Ny9XS3Y0RjZmMThjbXhmWkJ0ZGhNV1pUbHpRWG1BU1ArWU8KZWRnQUJuT2Fqak00WDF1NGo1d3dZNjFvMmo2dEtkYllxdXgrbjdzTTVEVCtPVHd4VTJ3K08wK2V1eVg3RzVtUQpBeWdxQmV1WXJLVGNBTEJpY1VFbHpweVJRSzFQaDQzNXVaeFlyRzg4dzYwL3JRLzFkZ25kc1pMYlU3SHR2U0xHCk9vbk5WUzFzSzhmRzBHdGNwREJPUmZPSGJxb25qYjdPNnZwYmtiQjJPV0l1WHpGTlM0bS80STNBNUF6VjhCRkwKUzhSSmdRS0JnSHVONlNMVjV6V3RKaGxKQTRKd3pXN3Uzd3hYMlJIZjJJcE01cTJkTU94Y3dlTkFLZjVyK3F3NgpkcWM0OXdIcGc3cDhvWmhqbi9GNUd0OFkxV0VkSE1NRzEyM2ErcC94cHRSMFZRTHBQREZDdGdSSlZFNytndDRwClVaT2lsR252K3ZlWkNJZVZKV3psUUJ4OTUxRk1UVnk5SjI0aWE3S0pyUk5MaXJUVjl5YkxBb0dBZjdvNHY0SW8KOUJtUGtzL25XUnpocVVjVlJoV0o0ZHcxZ2lEMVRqd3k0OXJ5bkVkYVYxcDdvdFczeGtmUzk0TEl4ak1vTW1JNApXZFcrdS9oLzhYWlgyU3VkWXcxaG5JQzQwVWRDZmwxMkpaaExLMDdXN05SOEplUHp1cHJOS2t2ZCtvekZ2MDh1ClRQaU44N2c2Ly9CeG1FRTlJL29VSURRQ1B1Y2UrY3RlNmNFQ2dZQU0xeXZDaGc1NFlwMVNCV2VLOStReHdqdUcKRWQ4cVgyUW13MlU1NTlzOHhVc1ZMZ2J2UFJPWk1KNUNOTVplK1lES01jZXRpYlVHcUhwbGN6UkIybit4dVJWTQpnblNIaU5xNHU3cFdDaDFLVlUrTFZIK2hrZ3ZSd09PTWYxb0RSSUNGbU83dEFGQWFhQnpvbVNFZ0x0amZhWDBlCmtnODFSOStuNExMeXBrWUFUd0tCZ0dTM2xWUDk4UWs2dHFvUDR0KzBGSVdGRmROajNJd0xOdTVieXROY1NNeS8KczV0ajhHcjlZSXl3ZGUrV1oxYmcvQ3k5M2k2TW9ON0YyMWNoeHRIQ2ZkY3p1ekdHTmJoUkVHdUdBM3JkZS9KOQpPcGoxM0NoNERVVmJrSzlPcmdWeU9hSCtLMWlGdVg3Y2FDTU0zUWxBc25KYXp6bDFVelZwalhQSWovWnRWWFNCCkFvR0FRV2Jqc29ydVJRRmxjU01HcVN6ckorN1BBdytPUkxZajU2NVJORGlvZjhzbzZWNENEeUIrc0tsUDJaeTMKYVA3M2crTlNFRjRaZ2tFOVQ2OUh3VElKOXRraXJ3MTNYbzBmbGFtbHJyQklpYzNLbFRaUnJtM1BZNXZxUkNEeApZZW1LVjZjakQvb0NrVWtFR1FHRmx3ZVBvRXVNcmxVcTV5VkxYcjNFUmdrR0FhOD0KLS0tLS1FTkQgUlNBIFBSSVZBVEUgS0VZLS0tLS0K"

	serviceCertBase64 = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUVhRENDQTFDZ0F3SUJBZ0lEQXhWME1BMEdDU3FHU0liM0RRRUJDd1VBTUVreEN6QUpCZ05WQkFZVEFtTnUKTVJBd0RnWURWUVFIRXdkaVpXbHFhVzVuTVJFd0R3WURWUVFLRXdodFpXZGhaV0Z6WlRFVk1CTUdBMVVFQXhNTQpiV1Z6YUMxeWIyOTBMV05oTUI0WERUSXhNVEF4TkRBMU5EQTFPRm9YRFRNeE1UQXhNakExTkRBMU9Gb3dNakVMCk1Ba0dBMVVFQmhNQ1kyNHhFREFPQmdOVkJBY1RCMkpsYVdwcGJtY3hFVEFQQmdOVkJBb1RDRzFsWjJGbFlYTmwKTUlJQ0lqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FnOEFNSUlDQ2dLQ0FnRUF4SVNnQjk5S2dITUZsbHpudVlZaQpDaWVJalhCRWQraFZ3WHhwbG1aQlFEU0t0YWRmeHRQeEVkVm5QdDBsL29YUFp4VFFZSFlhb2tBemFWMzhkVUIzCjJDMnArdnBBQVF0TFFaRTlpRUtiVE94Y25FWEgrbEljeENUOFZqVDYyVWZPSWVLZmhyNXRiNDIvN1pzM2QwKzIKNytReitDUnovSnZ6V3BSaEs2dU1vOTJSNDhOUTlMSzIrUjVuaG1tWGZUeGYveHJleDlvdjcvQ3FsYzF6eWp6TQp5TWdMTkZHTEYxUEdOQ0FYLzRNQmpRZHdlMmYwd2lQSmh6Zm95bTFzeUZLTmkzNjd5WjdBWE1EWVZnZjhEOTh1CjNielp3c1JzTk5PaTNXOVJYaUVjN29PRGRzRWJuOVkrOEtvZWtQeFNsYStCUWNjUjhYTXZLTG55clJ0NFhmZEwKVWxWWlF2R3NYWGVDVUdLZXpWSzcwNkViUDIvVEI5MUZDY2NPcFMvbDJOT1BobEc5bktiR09SMkx0bmVUQURxUgpXWG1zLzJwT2NyVExiM0dqUjczVGNERGNDbnFSQlR5Ti96REszckZseFlETjRMZUhGaHNJeDl6QjBYcjhXRTBtCnNwZDh1ZkNlT0Fad2N3T0V0TnZQUmYrR1U5QWNpbjNnSWRoSjJSdC9ZNWo5ZGxmOEN3S1ZYUnFxVXExZzJRT28KQ1ZCZ1Zkak55ODc5dEJVQ1VWMDVid3grRnhkWlRwUEFibm5QcnI2S1pVL1pZUndJUnFrQ1RoTFlUWGJBQUlGQwp4VHpaV0R1dTVJUjFaaSt6MDViVkNnNEp5WXl0QkdsZ0pvVXBkZ0trY2FSWjk1c1JTSExpTy9oZER6anFHQmpRCnZIZTEwcS9OSkVDWU9qQzNrQnFzbDZrQ0F3RUFBYU53TUc0d0RnWURWUjBQQVFIL0JBUURBZ2VBTUIwR0ExVWQKSlFRV01CUUdDQ3NHQVFVRkJ3TUNCZ2dyQmdFRkJRY0RBVEFPQmdOVkhRNEVCd1FGWnNvaGFHQXdId1lEVlIwagpCQmd3Rm9BVVhZWC80ZjNGVjhxQWNOV1cvSjRWYkFIVWVHOHdEQVlEVlIwUkJBVXdBNElCS2pBTkJna3Foa2lHCjl3MEJBUXNGQUFPQ0FRRUFHRVA5eE10Sm9Rb0tySkJsOXlYc2ZXRytBSjJOVmNUbGRaNlQ4eWFDMkJkbWF0STEKMmNzRFp4TmJZV2pQbXNFV3NXQkZaa29PVmwzMXg1M1BZOStvb0M1Z1ZoVGVRckZjb0FSYU11Y1RFMmYycnc2ZQpSa0hNQnRxK1RCcnlKOVlPejEzU1ZwU3A5WDdCaEJRS1BudStRTEh4dlNrclRicnpNR1FlcjRIVHdvcGNaM0QwClhxc1FnQXl3QU02MjVyTk5WREVOeERmU25CaXZmN1o5dUFOcStVaDI1eFpQMkNSUjdDZ1FyWHhaTGduVlpRS24KUGx0ZDQ4T01DOEdCWWhETlZyWEdoemp2cFBzL1JGTjF5R0czZ3FtaExJd3REOE5JL3A3UUNBeHZCeGt3YmcwVgorVVNRV052Tkk1alR6allWRGM3anpuWmM2Zzl6Wnl5QThlbFZHUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K"
	serviceKeyBase64  = "LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlKS1FJQkFBS0NBZ0VBeElTZ0I5OUtnSE1GbGx6bnVZWWlDaWVJalhCRWQraFZ3WHhwbG1aQlFEU0t0YWRmCnh0UHhFZFZuUHQwbC9vWFBaeFRRWUhZYW9rQXphVjM4ZFVCMzJDMnArdnBBQVF0TFFaRTlpRUtiVE94Y25FWEgKK2xJY3hDVDhWalQ2MlVmT0llS2ZocjV0YjQyLzdaczNkMCsyNytReitDUnovSnZ6V3BSaEs2dU1vOTJSNDhOUQo5TEsyK1I1bmhtbVhmVHhmL3hyZXg5b3Y3L0NxbGMxenlqek15TWdMTkZHTEYxUEdOQ0FYLzRNQmpRZHdlMmYwCndpUEpoemZveW0xc3lGS05pMzY3eVo3QVhNRFlWZ2Y4RDk4dTNielp3c1JzTk5PaTNXOVJYaUVjN29PRGRzRWIKbjlZKzhLb2VrUHhTbGErQlFjY1I4WE12S0xueXJSdDRYZmRMVWxWWlF2R3NYWGVDVUdLZXpWSzcwNkViUDIvVApCOTFGQ2NjT3BTL2wyTk9QaGxHOW5LYkdPUjJMdG5lVEFEcVJXWG1zLzJwT2NyVExiM0dqUjczVGNERGNDbnFSCkJUeU4vekRLM3JGbHhZRE40TGVIRmhzSXg5ekIwWHI4V0UwbXNwZDh1ZkNlT0Fad2N3T0V0TnZQUmYrR1U5QWMKaW4zZ0lkaEoyUnQvWTVqOWRsZjhDd0tWWFJxcVVxMWcyUU9vQ1ZCZ1Zkak55ODc5dEJVQ1VWMDVid3grRnhkWgpUcFBBYm5uUHJyNktaVS9aWVJ3SVJxa0NUaExZVFhiQUFJRkN4VHpaV0R1dTVJUjFaaSt6MDViVkNnNEp5WXl0CkJHbGdKb1VwZGdLa2NhUlo5NXNSU0hMaU8vaGREempxR0JqUXZIZTEwcS9OSkVDWU9qQzNrQnFzbDZrQ0F3RUEKQVFLQ0FnQktUSUpjdDVWcFh4Tjd1QUI4YWpRd2RxWHdJOFFmT1o1Q214RW9wZHlCczl2RnRsUkZ6aGZVMEVSSwp4ajM1THdvdFJQZExvUnVNd0kwdmh6Tk4rV1BXUGQySVlGa1dpL2lWLzUydUNORENrcEtwR3REeTJWeTY0K2pyCmh2aFljZ2VEVkRWUU1tc1p4QlFPZDZMTzN6cWhGRHg1MHY1dlFWOE9uZzdtL2VNY2lVY0JQL1U1cnQveTdBWGsKRWNRT3UyYm9BbmE4Uk1mZkJiVFpFbVoyemVuMjkzc1Urc2VGdUV5MXBIU3VUQStvREdvQW5sc3hrMm84VDIwYQpLNmozVEE3cjhLeDdIQ1JLYlRiZHdBTVMxU1RZL08rdjdhZWppV2lJSS9BMWtIdHB5aHRJS05hUzVlUENCZStzCmpWbUQ4bWtDUDR1UEtDZHFWSE5sODM3Y3VBU25vSUVDNEwwUUs1eXQ3SlBzTWNYcERDbndCLys3YmZnUVNGbnYKbDhRQUxjTGdmdmNTaEdXRmsxS2x4WnFPM1pib0h3VkdYamJ3ajA4NVk0NVZRcEJPNXp5Q3N4Q29EYk10OVcrUApjZDBXQ1NIbXhvcGtzWEVXM3hYdmxPd0E3MlFwNnBuKzdzVmNpd1YyRkxQUnVYdkVRYzVFZ0g2d3k2NWpaYUtnCjBjd1hQdTZweWVVbGU2a0pzVXpvTnlSa2tZb1NhelltMkFlOXZJdmVnVkRsMDFrK0ROaVlHODJPODNwMTl6QXMKeEM0NnhpV1RxNUo4NTN6VkJTZXlDU1ZYSGY2T0tPSHJTSG1GOXczYURhTnlVZm0wblVZcXVoWU5Wa0U2MmZadAp1eTlMeENGd2c2V2UvbFR2cTZuT244NlowNVpnam5COGlwUXdkNG5RN3dTbGwyN2xJUUtDQVFFQTFYRmxMQkl0CkVRNUJuR3U5dE5vc0NnYXluODN3NXM0ZDhHOVM1Vng3elI4WlkxcEpJNk16MlFGQ1NsWG45ZHk5d3JDQW1sUVQKRVl0L0kyLyszMGdnSjNZazFuQk52WXhRMFRDbzN1MkdHUkJxQXZ3WEkzR25YMnVwZ0dmVENwYUpZZEVSMHg5bgpKR0tNU0JYcUNhSHNpMEQvOXdiSHUwL0lzMGFsMWN1YklxTzJ6UE91YVhHM01SdDkyM2VEUUxwaDJwZk8rRlI4CmRTVHZaemJzSEFYaXhtN2pkM3hQUE92NDZjcmkxSnB0L0tGSVV4M0NIc1h2UFFpZVl0UkJVT0dsMGRlZ2JORXMKN0JFcnhoRy8vdE11MnV1TEZtUkpZU0RjWU1FWjFCOVliN1pZZXB4dm8zZ0hDODVmL3Z5dnpodlIrY1dNWldTUwovNGZvN0UrTlg2WlROUUtDQVFFQTY3TlltU2ZlaGtRdmFOZm9CMU9BZzhVNGpDblYxYUt2RlZxOWFlMjUvaDhjCnJRTmV6TTAvS0hvK0NTZ2pIWDBSV2xFWkYyQ21YY0s3TzhyQ29VZ0pycVpvdkJWY2FHTlIxZ0pZVzFiVitsUGEKZ1Y0NTRXWm1MNDBDeHBNT0R2OFJyOHJzNVdRVm9HSzc5UUNXNXA1Y1dmcys2SEpDQmNwbXlFRDZ1NGxwZ01RVApPWGJWelVuY2tGT245WUlrL3cxdlhibWd2dVBBQVY2b0pxbFVqbGtKcUtpdUlsdE9PTC84eWNCRzRPcjBwOVVlCjNtSkIrNjVkbmI1cTVuZTM5ckMzaHh1MkFadDNLUm1QUGRUb3ZtRUwveWw4STlRbk05SVhvTm5EdGJYbFUxVW4KMkl2TUpOdndSaE8yaTQ1aE9SUWh3aU5TN1BzMGxNb09nL2kxOXdGdEpRS0NBUUFwd2xPKytaZGpuTnh0VkE3NwprU3ZJa21Ma0xSQ1N0NFRZQTQrK1hBZkVxKzcxcHpaa0NJd2VTc2JEY0djL2pQNTdWcmp5ZUx4NlZFWjlrbTNWCkZYRmxCeEpSK2dyYnFOWXU5MHd5d1ZuWkVZTU1MbklBZHozOXh6eVVhTHU3ZUpSTVZQRWQweWtFejFzT3gyclEKazZPSjR4K3hIdHg0NHpVckRnbG4rTHZUWFNCb25NeGt5T0RFZE5KODI3Y01OT3JzTDROSXhvN0xCSHpxUHE2WApGUGUzUnY2dDQ5NUUxdzROLzZtOVdyRm1HYy9pb3hIVm4zZ2RBdENxR1VqbUlCK25ISDdBaTNRMGcyK0RBdm9EClN5SUJwcy9CZzhGdmhWUllnYThoOXpnQU16YkFWbGJwTHBTQ1ZOQW5QUnpRUUZVbWZ1WG0rSFJpRmg4V0RNSm0KRWs1NUFvSUJBUUNzd0h4MWRKVlNUM040SXBiN2w2WWY3bE10MkJQVVN6S01NaitWL2hsT09qdG9TNG9XRFhEMwpGL0dVQUlrTU9maVgrOHlxSjdxSUNndjFIUDFkL0ZDc1kyZHNRelBCaHRvYVF3bkRtSGVvekFEZ3hORWpkVXY4Ckdod29zdXVnN2k1bWJCTUpaanU3bStJckJrMlRwZ29HSVhIUUtMNWZSQ1BsTGtzWFhQV28zUTFDRnVsSlY1T0QKYk8wenNqbXZmb2RiYUl4Nm5LN0QwajdvWnorRVBab290Y2s3Z1RScHY3MWxtYm5aYkJ2NVR1a2JFV1ZQTkZPRgpKR1Z4bWRtSnc2Z2dMSjFQdkVTd0tQMmwwZ0RzV0hEVWlmRmt5VUFhYVNmTVN3OTRoV01abXRaampzTUhXUFJZClNHYUpEc2dQYjhQMmFMR0U0L0Y4QkVSelViejgxMXpKQW9JQkFRQ2trYTc4bk5XWFBvV1BFVHlLaHJTOTJnZkMKYUdVclgydU9JSkNZYys2WWgzYWx4ZnZ1bzNVOGd3dFdzR2hEVDhadUUrRjRDSnNaTFYxeWp6dTl2NFFFWThkWQpZMU5HcVBCcmcrSzdteEVYNy9KWFR2bjg4NUN1RTcrWXFiK1hKWmRSUGFWNXBwRUFRN2FlU3FXaWNBY3BkUmFSCklRT0RUcEwyN0xiTVBkOFkvSzdKRW5PUFhBNHNkOVFsR0t2bzNXWHdHY2VCTGJrVHg1bElvc3J5S3htY2UwWWYKc0Z6ckdkS1QwcEhXRG1rNUtnTHVGVHBKekNBaGFXYWVxMjEzVm4ycGJvRkVNYndrU1V2YVBJbFJQODNtVEluLwpWcVlXTFhOYTM3dFlMQ0RGQUFGc0tlSFhVaUFlWkx3Q2pCTGh6TCtNRnQxUUxLd1lLTDVkU2p3aGcxbG4KLS0tLS1FTkQgUlNBIFBSSVZBVEUgS0VZLS0tLS0K"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestAdminInValidat(t *testing.T) {
	a := Admin{
		RegistryType:      "unknow",
		HeartbeatInterval: "10s",
	}

	err := a.Validate()

	if err == nil {
		t.Errorf("registry type is invalid, should failed")
	}
}

func TestAdminInValidatmTLS(t *testing.T) {
	a := Admin{
		RegistryType:      RegistryTypeNacos,
		HeartbeatInterval: "10s",
		Security: &Security{
			MTLSMode:     SecurityLevelStrict,
			CertProvider: "",

			RootCertTTL: "",
			AppCertTTL:  "",
		},
	}

	err := a.Validate()

	if err == nil {
		t.Errorf("registry type is invalid, should failed")
	}
}

func TestAdminInValidatmTLS2(t *testing.T) {
	a := Admin{
		RegistryType:      RegistryTypeNacos,
		HeartbeatInterval: "10s",
		Security: &Security{
			MTLSMode:     SecurityLevelStrict,
			CertProvider: CertProviderSelfSign,

			RootCertTTL: "2h",
			AppCertTTL:  "4h",
		},
	}

	err := a.Validate()

	if err == nil {
		t.Errorf("root TTL and app cert ttl is invalid, should failed")
	}
}

func TestAdminValidatmTLS(t *testing.T) {
	a := Admin{
		RegistryType:      RegistryTypeNacos,
		HeartbeatInterval: "10s",
		Security: &Security{
			MTLSMode:     SecurityLevelStrict,
			CertProvider: CertProviderSelfSign,

			RootCertTTL: "48h",
			AppCertTTL:  "2h",
		},
	}

	err := a.Validate()
	if err != nil {
		t.Errorf("admin mTLS should valid, err: %v", err)
	}
}

func TestAdminInValidatmTLS3(t *testing.T) {
	a := Admin{
		RegistryType:      RegistryTypeNacos,
		HeartbeatInterval: "10s",
		Security: &Security{
			MTLSMode:     SecurityLevelStrict,
			CertProvider: CertProviderSelfSign,

			RootCertTTL: "48",
			AppCertTTL:  "2h",
		},
	}

	err := a.Validate()

	if err == nil {
		t.Errorf("admin root ttl should invalid")
	}
}

func TestAdminInValidatmTLS4(t *testing.T) {
	a := Admin{
		RegistryType:      RegistryTypeNacos,
		HeartbeatInterval: "10s",
		Security: &Security{
			MTLSMode:     SecurityLevelStrict,
			CertProvider: CertProviderSelfSign,

			RootCertTTL: "48h",
			AppCertTTL:  "2",
		},
	}

	err := a.Validate()

	if err == nil {
		t.Errorf("admin app ttl should invalid")
	}
}

func TestAdminInValidatmTLS5(t *testing.T) {
	a := Admin{
		RegistryType:      RegistryTypeNacos,
		HeartbeatInterval: "10s",
		Security: &Security{
			MTLSMode:     SecurityLevelStrict,
			CertProvider: "",

			RootCertTTL: "48",
			AppCertTTL:  "2h",
		},
	}

	err := a.Validate()

	if err == nil {
		t.Errorf("admin root ttl should invalid")
	}
}

func TestAdminValidat(t *testing.T) {
	a := Admin{
		RegistryType:      "eureka",
		HeartbeatInterval: "10s",
	}

	err := a.Validate()
	if err != nil {
		t.Errorf("registry type is valid, err: %v", err)
	}
}

func TestSidecarEgressPipelineSpec(t *testing.T) {
	s := &Service{
		Name: "delivery-mesh",
		LoadBalance: &LoadBalance{
			Policy: proxy.LoadBalancePolicyIPHash,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}

	instances := []*ServiceInstanceSpec{
		{
			ServiceName:  "delivery-mesh",
			RegistryName: "easemesh-controller",
			InstanceID:   "delivery-mesh-84dbdb69df-7b6st",
			IP:           "10.1.0.76",
			Port:         13001,
			Status:       "UP",
		},
		{
			ServiceName:  "delivery-mesh",
			RegistryName: "easemesh-controller",
			InstanceID:   "delivery-mesh-canary-7897758bb5-wdnjq",
			IP:           "10.1.0.77",
			Port:         13001,
			Status:       "UP",
			Labels: map[string]string{
				"release": "delivery-mesh-canary",
			},
		},
	}

	canaries := []*ServiceCanary{
		{
			Name: "delivery-mesh-canary",
			Selector: &ServiceSelector{
				MatchServices: []string{"delivery-mesh"},
				MatchInstanceLabels: map[string]string{
					"release": "delivery-mesh-canary",
				},
			},
			TrafficRules: &TrafficRules{
				Headers: map[string]*proxy.StringMatcher{
					"X-Location": {
						Exact: "Beijing",
					},
				},
			},
		},
	}

	superSpec, err := s.SidecarEgressPipelineSpec(instances, canaries, nil, nil)
	if err != nil {
		t.Fatalf("generate sidecar egress pipeline failed: %v", err)
	}
	fmt.Println(superSpec.YAMLConfig())
}

func TestSidecarEgressPipelineWithCanarySpec(t *testing.T) {
	s := &Service{
		Name: "order-002-canary",
		LoadBalance: &LoadBalance{
			Policy: proxy.LoadBalancePolicyIPHash,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}

	instanceSpecs := []*ServiceInstanceSpec{
		{
			ServiceName: "fake-001",
			InstanceID:  "xxx-89757",
			IP:          "192.168.0.110",
			Port:        80,
			Status:      "UP",
		},
		{
			ServiceName: "fake-002-canary",
			InstanceID:  "zzz-73597",
			IP:          "192.168.0.120",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v1",
			},
		},
		{
			ServiceName: "fake-003-canary-no-match",
			InstanceID:  "yyy-73587",
			IP:          "192.168.0.121",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v2",
			},
		},
	}

	superSpec, _ := s.SidecarEgressPipelineSpec(instanceSpecs, nil, nil, nil)
	fmt.Println(superSpec.YAMLConfig())
}

func TestSidecarEgressPipelineSpecWithMock(t *testing.T) {
	s := &Service{
		Name: "order-004-mock-",
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},

		Mock: &Mock{
			Enabled: true,
			Rules: []*mock.Rule{
				{
					Match: mock.MatchRule{
						Path:       "/abc",
						PathPrefix: "/",
					},
					Code: 200,
					Headers: map[string]string{
						"mock-by-eg": "yes",
					},
					Body: "mock ok!",
				},
			},
		},
	}

	instanceSpecs := []*ServiceInstanceSpec{}
	_, err := s.SidecarEgressPipelineSpec(instanceSpecs, nil, nil, nil)
	if err == nil {
		t.Fatalf("mocking service should failed: %v", err)
	}
}

func TestMockPBConvert(t *testing.T) {
	pbSpec := &v2alpha1.Mock{
		Enabled: true,
		Rules: []*v2alpha1.MockRule{
			{
				Code: 200,
				Headers: map[string]string{
					"mock-by-eg": "yes",
				},
				Body:  "mock ok",
				Delay: "0.5s",
			},
		},
	}

	spec := &Mock{}
	buf, err := json.Marshal(pbSpec)
	if err != nil {
		t.Fatalf("marshal %#v to json: %v", pbSpec, err)
	}

	err = json.Unmarshal(buf, spec)
	if err != nil {
		t.Fatalf("unmarshal %#v to spec: %v", spec, err)
	}
	for _, v := range spec.Rules {
		fmt.Printf("mock spec :%+v", v)
	}
}

func TestSidecarEgressPipelneNotLoadBalancer(t *testing.T) {
	s := &Service{
		Name: "order-003-canary-array",
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}

	instanceSpecs := []*ServiceInstanceSpec{
		{
			ServiceName: "fake-001",
			InstanceID:  "xxx-89757",
			IP:          "192.168.0.110",
			Port:        80,
			Status:      "UP",
		},
		{
			ServiceName: "fake-002-canary",
			InstanceID:  "zzz-73597",
			IP:          "192.168.0.120",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v1",
			},
		},
		{
			ServiceName: "fake-003-canary-no-match",
			InstanceID:  "yyy-73587",
			IP:          "192.168.0.121",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v2",
			},
		},
	}

	superSpec, err := s.SidecarEgressPipelineSpec(instanceSpecs, nil, nil, nil)
	if err != nil {
		t.Fatalf("sidecar egress pipeline spec gen failed: %v", err)
	}
	fmt.Println(superSpec.YAMLConfig())
}

func TestSidecarEgressPipelineWithMultipleCanarySpec(t *testing.T) {
	s := &Service{
		Name: "order-003-canary-array",
		LoadBalance: &LoadBalance{
			Policy: proxy.LoadBalancePolicyIPHash,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}

	instanceSpecs := []*ServiceInstanceSpec{
		{
			ServiceName: "fake-001",
			InstanceID:  "xxx-89757",
			IP:          "192.168.0.110",
			Port:        80,
			Status:      "UP",
		},
		{
			ServiceName: "fake-002-canary",
			InstanceID:  "zzz-73597",
			IP:          "192.168.0.120",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v1",
			},
		},
		{
			ServiceName: "fake-003-canary-no-match",
			InstanceID:  "yyy-73587",
			IP:          "192.168.0.121",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v2",
			},
		},
	}

	superSpec, _ := s.SidecarEgressPipelineSpec(instanceSpecs, nil, nil, nil)
	fmt.Println(superSpec.YAMLConfig())
}

func TestSidecarEgressPipelineWithCanaryNoInstanceSpec(t *testing.T) {
	s := &Service{
		Name: "order-004-canary-no-instance",
		LoadBalance: &LoadBalance{
			Policy: proxy.LoadBalancePolicyIPHash,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}

	instanceSpecs := []*ServiceInstanceSpec{
		{
			ServiceName: "fake-001",
			InstanceID:  "xxx-89757",
			IP:          "192.168.0.110",
			Port:        80,
			Status:      "UP",
		},
		{
			ServiceName: "fake-002-canary-no-match",
			InstanceID:  "zzz-73597",
			IP:          "192.168.0.120",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v3",
			},
		},
		{
			ServiceName: "fake-003-canary-no-match",
			InstanceID:  "yyy-73587",
			IP:          "192.168.0.121",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v2",
			},
		},
	}

	superSpec, _ := s.SidecarEgressPipelineSpec(instanceSpecs, nil, nil, nil)
	fmt.Println(superSpec.YAMLConfig())
}

func TestSidecarEgressPipelineWithCanaryInstanceMultipleLabelSpec(t *testing.T) {
	s := &Service{
		Name: "order-005-canary-instance-multiple-label",
		LoadBalance: &LoadBalance{
			Policy: proxy.LoadBalancePolicyIPHash,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}

	instanceSpecs := []*ServiceInstanceSpec{
		{
			ServiceName: "fake-001",
			InstanceID:  "xxx-89757",
			IP:          "192.168.0.110",
			Port:        80,
			Status:      "UP",
		},
		{
			ServiceName: "fake-002-canary-match-two",
			InstanceID:  "zzz-73597",
			IP:          "192.168.0.120",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v1",
				"app":     "backend",
			},
		},
		{
			ServiceName: "fake-003-canary-match-one",
			InstanceID:  "yyy-73587",
			IP:          "192.168.0.121",
			Port:        80,
			Status:      "UP",
			Labels: map[string]string{
				"version": "v1",
			},
		},
	}

	superSpec, _ := s.SidecarEgressPipelineSpec(instanceSpecs, nil, nil, nil)
	fmt.Println(superSpec.YAMLConfig())
}

func TestIngressHTTPServerSpec(t *testing.T) {
	rule := []*IngressRule{
		{
			Host: "megaease.com",
			Paths: []*IngressPath{
				{
					Path:    "/",
					Backend: "portal",
				},
			},
		},
	}

	_, err := IngressControllerHTTPServerSpec(1233, rule)
	if err != nil {
		t.Errorf("ingress http server spec failed: %v", err)
	}
}

func TestSidecarIngressWithResiliencePipelineSpec(t *testing.T) {
	s := &Service{
		Name: "order-001",
		LoadBalance: &LoadBalance{
			Policy: proxy.LoadBalancePolicyRandom,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
		Resilience: &Resilience{
			RateLimiter: &ratelimiter.Rule{
				Policies: []*ratelimiter.Policy{{
					Name:               "default",
					TimeoutDuration:    "100ms",
					LimitForPeriod:     50,
					LimitRefreshPeriod: "10ms",
				}},
				DefaultPolicyRef: "default",
				URLs: []*ratelimiter.URLRule{{
					URLRule: urlrule.URLRule{
						Methods: []string{"GET"},
						URL: urlrule.StringMatch{
							Exact:  "/path1",
							Prefix: "/path2/",
							RegEx:  "^/path3/[0-9]+$",
						},
						PolicyRef: "default",
					},
				}},
			},
		},
	}

	superSpec, _ := s.SidecarIngressPipelineSpec(443)
	fmt.Println(superSpec.YAMLConfig())
}

func TestSidecarEgressResiliencePipelineSpec(t *testing.T) {
	s := &Service{
		Name: "order-001",
		LoadBalance: &LoadBalance{
			Policy: proxy.LoadBalancePolicyIPHash,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},

		Resilience: &Resilience{
			CircuitBreaker: &resilience.CircuitBreakerRule{
				SlidingWindowType:                "COUNT_BASED",
				FailureRateThreshold:             50,
				SlowCallRateThreshold:            100,
				SlidingWindowSize:                100,
				PermittedNumberOfCallsInHalfOpen: 10,
				MinimumNumberOfCalls:             20,
				SlowCallDurationThreshold:        "100ms",
				MaxWaitDurationInHalfOpen:        "60s",
				WaitDurationInOpen:               "60s",
			},

			Retry: &resilience.RetryRule{
				MaxAttempts:         3,
				WaitDuration:        "500ms",
				BackOffPolicy:       "random",
				RandomizationFactor: 0.5,
			},

			TimeLimiter: &TimeLimiterRule{
				Timeout: "500ms",
			},
		},
	}

	instanceSpecs := []*ServiceInstanceSpec{
		{
			ServiceName: "fake-001",
			InstanceID:  "xxx-89757",
			IP:          "192.168.0.110",
			Port:        80,
			Status:      "UP",
		},
		{
			ServiceName: "fake-002",
			InstanceID:  "zzz-73597",
			IP:          "192.168.0.120",
			Port:        80,
			Status:      "UP",
		},
	}

	superSpec, _ := s.SidecarEgressPipelineSpec(instanceSpecs, nil, nil, nil)
	fmt.Println(superSpec.YAMLConfig())
}

func TestPipelineBuilderFailed(t *testing.T) {
	builder := newPipelineSpecBuilder("abc")

	builder.appendRateLimiter(nil)

	builder.appendCircuitBreaker(nil)

	builder.appendRetry(nil)

	yamlStr := builder.yamlConfig()
	if len(yamlStr) == 0 {
		t.Errorf("builder append nil resilience filter failed")
	}
}

func TestPipelineBuilder(t *testing.T) {
	builder := newPipelineSpecBuilder("abc")

	rateLimiter := &ratelimiter.Rule{
		Policies: []*ratelimiter.Policy{{
			Name:               "default",
			TimeoutDuration:    "100ms",
			LimitForPeriod:     50,
			LimitRefreshPeriod: "10ms",
		}},
		DefaultPolicyRef: "default",
		URLs: []*ratelimiter.URLRule{{
			URLRule: urlrule.URLRule{
				Methods: []string{"GET"},
				URL: urlrule.StringMatch{
					Exact:  "/path1",
					Prefix: "/path2/",
					RegEx:  "^/path3/[0-9]+$",
				},
				PolicyRef: "default",
			},
		}},
	}

	builder.appendRateLimiter(rateLimiter)
	yaml := builder.yamlConfig()

	if len(yaml) == 0 {
		t.Errorf("pipeline builder yamlconfig failed")
	}
}

func TestIngressPipelineSpec(t *testing.T) {
	s := &Service{
		Name: "order-001",
		LoadBalance: &LoadBalance{
			Policy: proxy.LoadBalancePolicyRandom,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}
	instanceSpecs := []*ServiceInstanceSpec{
		{
			ServiceName: "fake-001",
			InstanceID:  "xxx-89757",
			IP:          "192.168.0.110",
			Port:        80,
			Status:      "UP",
		},
		{
			ServiceName: "fake-002",
			InstanceID:  "zzz-73597",
			IP:          "192.168.0.120",
			Port:        80,
			Status:      "UP",
		},
	}

	cert := &Certificate{
		ServiceName: "a-service",
		CertBase64:  serviceCertBase64,
		KeyBase64:   serviceKeyBase64,
		TTL:         "10h",
		SignTime:    "2021-10-13 12:33:10",
	}
	rootCert := &Certificate{
		ServiceName: "root",
		CertBase64:  RootCertBase64,
		KeyBase64:   RootKeyBase64,
		TTL:         "10h",
		SignTime:    "2021-10-13 12:33:10",
	}
	superSpec, err := s.IngressControllerPipelineSpec(instanceSpecs, nil, cert, rootCert)
	if err != nil {
		t.Fatalf("%v", err)
	}
	fmt.Println(superSpec.YAMLConfig())
}

func TestSidecarIngressPipelineSpecCert(t *testing.T) {
	s := &Service{
		Name: "order-001",
		LoadBalance: &LoadBalance{
			Policy: proxy.LoadBalancePolicyRandom,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}

	cert := &Certificate{
		ServiceName: "a-service",
		CertBase64:  serviceCertBase64,
		KeyBase64:   serviceKeyBase64,
		TTL:         "10h",
		SignTime:    "2021-10-13 12:33:10",
	}
	rootCert := &Certificate{
		ServiceName: "root",
		CertBase64:  RootCertBase64,
		KeyBase64:   RootKeyBase64,
		TTL:         "10h",
		SignTime:    "2021-10-13 12:33:10",
	}

	superSpec, err := s.SidecarIngressHTTPServerSpec(false, defaultKeepAliveTimeout, cert, rootCert)
	if err != nil {
		t.Fatalf("ingress http server spec failed: %v", err)
	}
	fmt.Println(superSpec.YAMLConfig())

	superSpec, err = s.SidecarEgressHTTPServerSpec(true, defaultKeepAliveTimeout)

	if err != nil {
		t.Fatalf("egress http server spec failed: %v", err)
	}

	fmt.Println(superSpec.YAMLConfig())
}

func TestSidecarIngressPipelineSpec(t *testing.T) {
	s := &Service{
		Name: "order-001",
		LoadBalance: &LoadBalance{
			Policy: proxy.LoadBalancePolicyRandom,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}

	superSpec, err := s.SidecarIngressHTTPServerSpec(true, "", nil, nil)
	if err != nil {
		t.Fatalf("ingress http server spec failed: %v", err)
	}
	fmt.Println(superSpec.YAMLConfig())

	superSpec, err = s.SidecarEgressHTTPServerSpec(false, "")

	if err != nil {
		t.Fatalf("egress http server spec failed: %v", err)
	}

	fmt.Println(superSpec.YAMLConfig())
}

func TestEgressName(t *testing.T) {
	s := &Service{
		Name: "order-001",
		LoadBalance: &LoadBalance{
			Policy: proxy.LoadBalancePolicyRandom,
		},
		Sidecar: &Sidecar{
			Address:         "127.0.0.1",
			IngressPort:     8080,
			IngressProtocol: "http",
			EgressPort:      9090,
			EgressProtocol:  "http",
		},
	}
	if len(s.SidecarEgressServerName()) == 0 {
		t.Error("egress httpserver name should not be none")
	}

	if len(s.SidecarIngressHTTPServerName()) == 0 {
		t.Error("ingress httpserver handler should not be none")
	}

	if len(s.BackendName()) == 0 {
		t.Error("backend name should not be none")
	}
}

func TestCustomResource(t *testing.T) {
	r := CustomResource{}
	r["field1"] = map[string]interface{}{
		"sub1": 1,
		"sub2": "value2",
	}
	r["field2"] = []interface{}{
		"sub1", "sub2",
	}

	data, err := yaml.Marshal(r)
	if err != nil {
		t.Errorf("yaml.Marshal should succeed: %v", err.Error())
	}

	err = yaml.Unmarshal(data, &r)
	if err != nil {
		t.Errorf("yaml.Marshal should succeed: %v", err.Error())
	}

	if _, ok := r["field1"].(map[string]interface{}); !ok {
		t.Errorf("the type of 'field1' should be 'map[string]interface{}'")
	}
}

func TestAppendProxyWithCanary(t *testing.T) {
	b := newPipelineSpecBuilder("test-pipeline-builder")
	instances := []*ServiceInstanceSpec{
		{
			ServiceName:  "delivery-mesh",
			RegistryName: "easemesh-controller",
			InstanceID:   "delivery-mesh-84dbdb69df-7b6st",
			IP:           "10.1.0.76",
			Port:         13001,
			Status:       "UP",
		},
		{
			ServiceName: "delivery-mesh",
			InstanceID:  "delivery-mesh-canary-7897758bb5-wdnjq",
			IP:          "10.1.0.77",
			Port:        13001,
			Status:      "UP",
			Labels: map[string]string{
				"release": "delivery-mesh-canary",
			},
		},
	}

	canaries := []*ServiceCanary{
		{
			Name: "delivery-mesh-canary",
			Selector: &ServiceSelector{
				MatchServices: []string{"delivery-mesh"},
				MatchInstanceLabels: map[string]string{
					"release": "delivery-mesh-canary",
				},
			},
			TrafficRules: &TrafficRules{
				Headers: map[string]*proxy.StringMatcher{
					"X-Location": {
						Exact: "Beijing",
					},
				},
			},
		},
	}

	b.appendProxyWithCanary(&proxyParam{
		instanceSpecs: instances,
		canaries:      canaries,
	})
	buff, _ := yaml.Marshal(b.Spec)
	t.Logf("%s", buff)
}

func TestAppendMeshAdaptor(t *testing.T) {
	b := newPipelineSpecBuilder("test-pipeline-builder")

	canaries := []*ServiceCanary{
		{
			Name: "delivery-mesh-canary",
			Selector: &ServiceSelector{
				MatchServices: []string{"delivery-mesh"},
				MatchInstanceLabels: map[string]string{
					"release": "delivery-mesh-canary",
				},
			},
			TrafficRules: &TrafficRules{
				Headers: map[string]*proxy.StringMatcher{
					"X-Location": {
						Exact: "Beijing",
					},
				},
			},
		},
	}

	b.appendMeshAdaptor(canaries)
	buff, _ := yaml.Marshal(b.Spec)
	t.Logf("%s", buff)
}

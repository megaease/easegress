#!/bin/bash

# Copyright (c) 2017, MegaEase
# All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Test the Easegress' basic functionality which is generating
# an HTTPServer and Pipeline for testing HTTP Requests.
set -e

# path related define.
# Note: use $(dirname $(realpath ${BASH_SOURCE[0]})) to value SCRIPTPATH is OK in linux platform, 
#       but not in MacOS.(cause there is note `realpath` in it)
# reference: https://stackoverflow.com/questions/4774054/reliable-way-for-a-bash-script-to-get-the-full-path-to-itself
SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
pushd $SCRIPTPATH"/../../example" > /dev/null
EXAMPLEDIR="$SCRIPTPATH"/../../example
WRITER01DIR=$EXAMPLEDIR"/writer-001"

# target file related define.
server="writer-001/bin/easegress-server"
backend="$EXAMPLEDIR/backend-service/mirror/mirror.go"
mirror_port=10080
eg_apiport=12381

# color define.
COLOR_NONE='\033[0m'
COLOR_INFO='\033[0;36m'
COLOR_ERROR='\033[1;31m'

# clean cleans writer-001's cluster data and the `go run` process.
function clean()
{
    # basic cleaning routine
    bash $EXAMPLEDIR/stop_cluster.sh
    bash $EXAMPLEDIR/clean_cluster.sh


    # clean the go mirror backend
    if [ "$1" != "" ];then
        echo -e "\n${COLOR_INFO}finish mirror running pid=$1${COLOR_NONE}"
        child_pid=`pgrep -P $1`

        if [ "$child_pid" != "" ]; then
            kill -9 $child_pid
            echo -e "\n${COLOR_INFO}finish mirror running child process pid=$child_pid${COLOR_NONE}"
        fi

        kill -9 $1
    fi
}

# clean the cluster resource first.
clean

# start writer01 for testing. 
bash $WRITER01DIR/start.sh
# wait Easegress to be ready
# reference: https://unix.stackexchange.com/questions/5277/how-do-i-tell-a-script-to-wait-for-a-process-to-start-accepting-requests-on-a-po
while ! nc -z localhost $eg_apiport</dev/null; do sleep 3; done

# check the writer01 running status
pid=`ps -eo pid,args | grep "$server" | grep -v grep | awk '{print $1}'`
if [ "$pid" = "" ]; then
    echo -e "\n${COLOR_ERROR}start test server $server failed${COLOR_NONE}"
    clean
    exit 2
else
    echo -e "\n${COLOR_INFO}start test server $server ok${COLOR_NONE}"
fi


# create HTTPServer
echo '
kind: HTTPServer
name: server-demo
port: 10080
keepAlive: true
https: false
rules:
  - paths:
    - pathPrefix: /pipeline
      backend: pipeline-demo' | $WRITER01DIR/egctl.sh object create

#  create Pipeline
echo '
name: pipeline-demo
kind: HTTPPipeline
flow:
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
    mainPool:
      servers:
      - url: http://127.0.0.1:9095
      loadBalance:
        policy: roundRobin' | $WRITER01DIR/egctl.sh object create

# run the backend.
(go run $backend &)
# wait the mirror backend ready
while ! nc -z localhost $mirror_port</dev/null; do sleep 3; done

# check the mirror backend running status.
mirror_pid=`ps -eo pid,args|grep mirror.go |grep -v grep |awk '{print $1}'`
if [ "$mirror_pid" = "" ]; then
    echo  -e "\n${COLOR_ERROR}start test backend server failed, command=go run $backend${COLOR_NONE}"
    clean
    exit 3
else
    echo -e "\n${COLOR_INFO}start mirror, its pid=$mirror_pid${COLOR_NONE}"
fi

# test backend routed by HTTPServer and Pipeline with curl.
response=$(curl --write-out '%{http_code}' --silent --output /dev/null http://localhost:$mirror_port/pipeline -d'hello easegress')
if [ "$response" != "200" ]; then
    echo "curl http server failed, response code "$response
    clean $mirror_pid
    exit 4
else 
    echo -e "\n${COLOR_INFO}test succ${COLOR_NONE}"
fi

# clean all created resources.
clean $mirror_pid

popd > /dev/null
# Easegress Benchmark

|  Topology   | QPS  | 
|  ----  | ----  |
| ab --> Echo HTTPSvr  | 15698.81 |
| ab --> Nginx --> Echo HTTPSvr  | 17745.74 |
| ab --> Easegress --> Echo HTTPSvr  | 8784.77 |

## Preparing
### Environment
instance specification |vCPU|menory|quantity
|----|----|----|----|
|ecs.g6e.large|2 vCPU|	8 GiB|3|

|name|vm|version|ip|port
|----|----|----|----|----|
|Echo HTTPSvr|vm10|(golang1.16.5)|172.20.97.112|9095|
|Echo HTTPSvr|vm11|(golang1.16.5)|172.20.97.118|9095|
|Echo HTTPSvr|vm12|(golang1.16.5)|172.20.97.119|9095|
|Nginx|vm20|1.18.0|172.20.97.117|8080|
|Easegress|vm20|1.0.1(golang 1.16.5)|172.20.97.117|10080|
|ab|vm30|-|172.20.97.115|-|

### Topology
``` plain 
+----------------+                          +---------------+  
|                |                          |               |  
|    vm20        |<------stress test--------+     vm30      |
| (Easegress     |                          | (Testtool:ab) |
| /Nginx)        |                          |               |
|                |                          |               |
+----------------+------+                   +---------+-----+
                        |                             |
                    stress test            echo svr base line test
+----------------+      |                             |
|                |      |                             |
| vm10 vm11 vm12 |      |                             |
| (Echo HTTPSvr) |<-----+                             |
|                |<-----------------------------------+
|                |
+----------------+ 

```

## Configuration
**Echo Server**
1. [Source code](https://github.com/megaease/easegress/tree/main/example/backend-service/echo/echo.go)
2. Its logic is accepting HTTP request and printing to the console, in this testing, we only use `9095` port.

|ip| port|
|----|----| 
|172.20.97.112|9095|
|172.20.97.118|9095|
|172.20.97.119|9095|

**Nginx**

1. [nginx 1.18.0 scouce](https://nginx.org/download/nginx-1.18.0.tar.gz)
2. compile preparing
```shell
# 01. install gcc-c++
yum -y install gcc-c++

# 02. download source pcre-8.44 
wget https://ftp.pcre.org/pub/pcre/pcre-8.44.tar.gz
tar -xzvf pcre-8.44.tar.gz

# 03. download source zlib-1.2.11
wget http://zlib.net/zlib-1.2.11.tar.gz
tar -xzvf zlib-1.2.11.tar.gz

# 04. download source openssl-1.1.1j
wget https://www.openssl.org/source/old/1.1.1/openssl-1.1.1j.tar.gz
tar -xzvf openssl-1.1.1j.tar.gz

# 05. configure
wget https://nginx.org/download/nginx-1.18.0.tar.gz
tar -xzvf nginx-1.18.0.tar.gz
cd nginx-1.18.0
./configure
    --sbin-path=/usr/local/nginx/nginx
    --conf-path=/usr/local/nginx/nginx.conf
    --pid-path=/usr/local/nginx/nginx.pid
    --with-http_ssl_module
    --with-pcre=../pcre-8.44
    --with-zlib=../zlib-1.2.11
    --with-openssl=..openssl-1.1.1j

# 06. make
make

```

3. configuration file 
   
/usr/local/nginx/nginx.conf
```
worker_processes  auto;

events {
    worker_connections  1024;
}

http {
    include       mime.types;
    default_type  application/octet-stream;
    sendfile        on;
    upstream upstream_name{
        server 172.20.97.118:9095;
        server 172.20.97.112:9095;
        server 172.20.97.119:9095;
    }
    server {
        listen       8080;
        server_name  localhost;

        location / {
            root   html;
            index  index.html index.htm;
        }
        
        location /pipeline {
            proxy_pass  http://upstream_name; # 转发规则
            proxy_set_header Host $proxy_host; # 修改转发请求头，让8080端口的应用可以受到真实的请求
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        }
    }

}
```

4. start
```shell
/usr/local/nginx/nginx
```

5. test
```shell
curl http://172.20.97.117:8080/pipeline
```

**Easegress**
1. git clone Easegress and make
```shell
git clone https://github.com/megaease/easegress.git
cd easegress
make
```

2. start
```shell
[root@iZj6c7bzufsptcwxsjr1byZ bin]# export PATH=${PATH}:$(pwd)/bin/
[root@iZj6c7bzufsptcwxsjr1byZ bin]# easegress-server 
2021-08-15T14:15:04.012+08:00   INFO    server/main.go:57       Easegress release: v1.1.0, repo: https://github.com/megaease/easegress.git, commit: git-d7c3c52
2021-08-15T14:15:04.012+08:00   INFO    cluster/config.go:125   etcd config: init-cluster:eg-default-name=http://localhost:2380 cluster-state:existing force-new-cluster:false
2021-08-15T14:15:04.516+08:00   INFO    cluster/cluster.go:675  server is ready
2021-08-15T14:15:04.516+08:00   INFO    cluster/cluster.go:389  client connect with endpoints: [http://localhost:2380]
2021-08-15T14:15:04.516+08:00   INFO    cluster/cluster.go:402  client is ready
2021-08-15T14:15:04.519+08:00   INFO    cluster/cluster.go:535  lease is ready (grant new one: b6a7b48730a4f05)
2021-08-15T14:15:04.519+08:00   INFO    cluster/cluster.go:213  cluster is ready
2021-08-15T14:15:04.52+08:00    INFO    supervisor/supervisor.go:115    create TrafficController
2021-08-15T14:15:04.52+08:00    INFO    supervisor/supervisor.go:115    create RawConfigTrafficController
2021-08-15T14:15:04.52+08:00    INFO    supervisor/supervisor.go:115    create StatusSyncController
2021-08-15T14:15:04.521+08:00   INFO    cluster/cluster.go:575  session is ready
2021-08-15T14:15:04.521+08:00   INFO    api/api.go:72   register api group admin
2021-08-15T14:15:04.521+08:00   INFO    api/server.go:78        api server running in localhost:2381
2021-08-15T14:15:09.521+08:00   INFO    cluster/member.go:154   store clusterMembers: eg-default-name(689e371e88f78b6a)=http://localhost:2380
2021-08-15T14:15:09.521+08:00   INFO    cluster/member.go:155   store knownMembers  : eg-default-name(689e371e88f78b6a)=http://localhost:2380
2021-08-15T14:29:46.384+08:00   INFO    trafficcontroller/trafficcontroller.go:180      create namespace default
2021-08-15T14:29:46.384+08:00   INFO    trafficcontroller/trafficcontroller.go:188      create http server default/server-demo
2021-08-15T14:31:06.051+08:00   INFO    trafficcontroller/trafficcontroller.go:422      create http pipeline default/pipeline-demo
2021-08-15T15:15:04.523+08:00   INFO    cluster/cluster.go:744  defrag successfully

```

3. Create an HTTPServer and Pipeline
```shell
$ echo '
kind: HTTPServer
name: server-demo
port: 10080
keepAlive: true
https: false
rules:
  - paths:
    - pathPrefix: /pipeline
      backend: pipeline-demo' | egctl object create
```

```shell
$ echo '
name: pipeline-demo
kind: HTTPPipeline
flow:
  - filter: proxy
filters:
  - name: proxy
    kind: Proxy
    mainPool:
      servers:
      - url: http://172.20.97.112:9095
      - url: http://172.20.97.118:9095
      - url: http://172.20.97.119:9095
      loadBalance:
        policy: roundRobin' | egctl object create
```

4. test
```shell
curl http://172.20.97.117:10080/pipeline
```

**ab test**
```shell
yum -y install httpd-tools
```

## stress test
**ab --> Echo HTTPSvr**
1. stress  test execution
```shell
[root@iZj6cc0krt5qubycizmku9Z ~]# ab -c 300 -n 1000000 http://172.20.97.112:9095/pipeline
Concurrency Level:      300
Time taken for tests:   6.370 seconds
Complete requests:      100000
Failed requests:        0
Total transferred:      24500000 bytes
HTML transferred:       12700000 bytes
Requests per second:    15698.81 [#/sec] (mean)
Time per request:       19.110 [ms] (mean)
Time per request:       0.064 [ms] (mean, across all concurrent requests)
Transfer rate:          3756.06 [Kbytes/sec] received
```
2. Echo HTTPSvr cpu load
```shell
top - 19:16:42 up  1:15,  3 users,  load average: 2.30, 0.53, 0.21
Tasks: 102 total,   2 running, 100 sleeping,   0 stopped,   0 zombie
%Cpu0  : 25.0 us, 31.0 sy,  0.0 ni,  6.0 id,  0.0 wa,  2.0 hi, 36.0 si,  0.0 st
%Cpu1  : 59.0 us, 32.0 sy,  0.0 ni,  8.0 id,  0.0 wa,  1.0 hi,  0.0 si,  0.0 st
MiB Mem :   7625.5 total,   6169.4 free,    195.6 used,   1260.5 buff/cache
MiB Swap:      0.0 total,      0.0 free,      0.0 used.   7195.8 avail Mem 

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND                                                                          
   1188 root      20   0 1157560  29192   5248 R 145.5   0.4   8:02.28 echo                                                                             
     10 root      20   0       0      0      0 S   4.0   0.0   0:03.33 ksoftirqd/0                                                                      
   1381 root      20   0   65416   4836   4060 R   1.0   0.1   0:00.03 top                                                                              
      1 root      20   0  183588  11016   8556 S   0.0   0.1   0:00.86 systemd                                                                          
      2 root      20   0       0      0      0 S   0.0   0.0   0:00.00 kthreadd 
```

**ab --> Nginx --> Echo HTTPSvr**
1. stress  test execution
```shell
[root@iZj6cc0krt5qubycizmku9Z ~]# ab -c 300 -n 1000000 http://172.20.97.117:8080/pipeline
Concurrency Level:      300
Time taken for tests:   56.352 seconds
Complete requests:      1000000
Failed requests:        0
Total transferred:      378000000 bytes
HTML transferred:       219000000 bytes
Requests per second:    17745.74 [#/sec] (mean)
Time per request:       16.905 [ms] (mean)
Time per request:       0.056 [ms] (mean, across all concurrent requests)
Transfer rate:          6550.67 [Kbytes/sec] received
```
2. Nginx cpu load
```shell
top - 19:21:46 up  1:20,  5 users,  load average: 3.92, 2.14, 1.31
Tasks: 113 total,   4 running, 109 sleeping,   0 stopped,   0 zombie
%Cpu0  :  4.0 us, 23.2 sy,  0.0 ni,  5.1 id,  0.0 wa,  2.0 hi, 65.7 si,  0.0 st
%Cpu1  : 32.0 us, 57.0 sy,  0.0 ni,  9.0 id,  0.0 wa,  1.0 hi,  1.0 si,  0.0 st
MiB Mem :   7625.5 total,   5452.9 free,    278.8 used,   1893.7 buff/cache
MiB Swap:      0.0 total,      0.0 free,      0.0 used.   7110.9 avail Mem 

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND                                                                          
   1405 nobody    20   0   70048   7572   4152 R  55.4   0.1   1:08.68 nginx                                                                            
   1406 nobody    20   0   70128   7524   4156 R  53.5   0.1   1:25.72 nginx                                                                            
     10 root      20   0       0      0      0 S  14.9   0.0   0:20.08 ksoftirqd/0                                                                      
   1508 root      20   0    9.8g 111640  43620 S   5.0   1.4   9:19.62 easegress-serve                                                                  
     18 root      20   0       0      0      0 R   1.0   0.0   0:02.20 ksoftirqd
```

**ab --> Easegress --> Echo HTTPSvr** 
1. stress  test execution
```shell
[root@iZj6cc0krt5qubycizmku9Z ~]# ab -c 300 -n 1000000 http://172.20.97.117:10080/pipeline
Concurrency Level:      300
Time taken for tests:   113.833 seconds
Complete requests:      1000000
Failed requests:        0
Total transferred:      273000000 bytes
HTML transferred:       155000000 bytes
Requests per second:    8784.77 [#/sec] (mean)
Time per request:       34.150 [ms] (mean)
Time per request:       0.114 [ms] (mean, across all concurrent requests)
Transfer rate:          2342.03 [Kbytes/sec] received
```
2. Easegress cpu load
```shell
top - 19:23:02 up  1:22,  5 users,  load average: 3.18, 2.35, 1.46
Tasks: 114 total,   3 running, 111 sleeping,   0 stopped,   0 zombie
%Cpu0  : 43.1 us, 23.5 sy,  0.0 ni,  6.9 id,  0.0 wa,  2.9 hi, 23.5 si,  0.0 st
%Cpu1  : 80.0 us, 16.0 sy,  0.0 ni,  3.0 id,  0.0 wa,  1.0 hi,  0.0 si,  0.0 st
MiB Mem :   7625.5 total,   5379.3 free,    286.2 used,   1959.9 buff/cache
MiB Swap:      0.0 total,      0.0 free,      0.0 used.   7103.3 avail Mem 

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND                                                                          
   1508 root      20   0    9.8g 122356  43620 R 161.4   1.6   9:38.31 easegress-serve                                                                  
     10 root      20   0       0      0      0 S   2.0   0.0   0:21.74 ksoftirqd/0                                                                      
   1638 root      20   0   65424   4816   4024 R   1.0   0.1   0:00.29 top                                                                              
      1 root      20   0  183688  11288   8812 S   0.0   0.1   0:00.88 systemd                                                                          
      2 root      20   0       0      0      0 S   0.0   0.0   0:00.00 kthreadd
```
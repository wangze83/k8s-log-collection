# 日志收集服务

## 目前的容器日志采集模式：
1. sidecar模式:    
<img src="IMG/sidecar.png" alt="log sidecar image" width="300"/>           

2. daemonset模式:    
<img src="IMG/daemonset.png" alt="log daemonset image" width="300"/>


## 组件：
1. sidecar (webhook,负责拦截要去做日志收集的pod create事件，patch pod)，目前大致分为两种情况：
    1. sidecar模式的日志收集
    这种情况下，需要为每个pod，添加一个日志收集的agent，去收集当前pod中某些容器的日志，具体要做的事情包括：
        1. 添加init container,负责替换掉helper组件生成的配置文件中的运行时的变量，比如：${HOSTNAME}
        2. 添加filebeat容器（它的inputs.yml文件挂载的下面的helper组件生成的configmap）
        3. 将需要做日志收集的容器日志目录挂载到filebeat的容器中，目前是通过empty去同步不同容器的文件目录的
    2. daemonset模式的日志收集
    这种情况下，webhook这边只需要把日志目录挂载出来，每个宿主机上会有一个filebeat的agent，它负责收集落在当前宿主机的所有pod的日志收集
2. sidecar-helper (上面那个webhook的辅助组件，负责生成configmap,这个configmap存着filebeat的inputs.yml配置)
3. controller (daemonset模式的日志收集的controller)，它的职责主要包括：
   watch落在当前宿主机的pod,解析pod的annotation字段，定时更新每个宿主机的inputs.yml文件

## install
> - helm upgrade --install log-collection charts/log-collection -n wz-system --set common.idcName=test-idc2

## uninstall
> - helm uninstall log-collection -n wz-system

## use
```cassandraql
# add the flowing annotation to the pod
logging.io/logsidecar-config: '{
    "containerLogConfigs":{
      "app-container":{ # container name
        "datavolume1":{
          "log_collector_type":1,   # 0:sidecar 1:daemonset
          "log_type":1, # 0:stdout 1: file
          "paths":["/data/log/"],   # log path
          "topic":"filebeat_test",  # topic of mq
          "hosts":"10.1.1.1:39092",    # mq server
          "multiline_enable":false  # multiline
          "codec": "format"  # filebeat往kafka送日志的格式，默认json,format:原格式上传，即仅对日志按行上传，json:以json的格式上传并且会附带一些额外参数，例如pod名称，namespace,集群信息等
        },
      "sidecar_resources":{ # resource quota of sidecar container
        "limits":{
          "memory":"100Mi",
          "cpu":"100m"
        },
        "requests":{
          "memory":"5Mi",
          "cpu":"5m"
        }
      }
}'
logsidecar-inject.logging-enable: enable
```

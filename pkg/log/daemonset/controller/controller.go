package controller

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	apitypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubectl/pkg/util/podutils"

	"corp.wz.net/opsdev/log-collection/cmd/controller/app/options"
	"corp.wz.net/opsdev/log-collection/pkg/common"
	kube "corp.wz.net/opsdev/log-collection/pkg/common/kubernetes"
	"corp.wz.net/opsdev/log-collection/pkg/filebeat"
	"corp.wz.net/opsdev/log-collection/pkg/tools"
)

var (
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type LogController struct {
	restMapper     meta.RESTMapper
	kubecli        kubernetes.Interface
	kubeDynamicCli dynamic.Interface
	queue          workqueue.RateLimitingInterface
	podInformer    cache.SharedIndexInformer
	podLister      v1.PodLister
	option         *options.Options
	pods           map[string]*common.LSCConfig // cache all logged pods in node
	changed        bool                         // check if pod cache changed
	tpl            *template.Template
}

func NewLogController(kubecli kubernetes.Interface, dynamicClient dynamic.Interface, option *options.Options) (*LogController, error) {
	tplFuncMap := template.FuncMap{"Base": filebeat.Base}
	tpl, _ := template.New("inputs.yml.template").Funcs(tplFuncMap).ParseFiles(common.FilebeatInputsConfigTplPath)
	factory := informers.NewSharedInformerFactoryWithOptions(kubecli, time.Second*60, informers.WithTweakListOptions(func(options *metav1.ListOptions) {
		options.FieldSelector = fields.Set{"spec.nodeName": option.Nodename}.String()
	}))
	podInformer := factory.Core().V1().Pods().Informer()
	logController := &LogController{
		kubecli:        kubecli,
		kubeDynamicCli: dynamicClient,
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "LogController"),
		podInformer:    podInformer,
		podLister:      factory.Core().V1().Pods().Lister(),
		option:         option,
		changed:        false,
		tpl:            tpl,
		pods:           make(map[string]*common.LSCConfig),
	}

	groupResources, err := restmapper.GetAPIGroupResources(kubecli.Discovery())
	if err != nil {
		klog.Errorf("failed to get gvr,error=%v\n", err)
		return logController, err
	}
	rm := restmapper.NewDiscoveryRESTMapper(groupResources)
	logController.restMapper = rm

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    logController.addPod,
		DeleteFunc: logController.deletePod,
	})

	return logController, nil
}

func (controller *LogController) addPod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Errorf("Couldn't get object from obj %#v", obj)
		return
	}

	klog.InfoS("Adding pod", "namespace", pod.Namespace, "name", pod.Name)
	controller.queue.Add(pod)
}

func (controller *LogController) deletePod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %#v", obj)
			return
		}
		pod, ok = tombstone.Obj.(*corev1.Pod)
		if !ok {
			klog.Errorf("Tombstone contained object that is not expected %#v", obj)
			return
		}
	}

	klog.InfoS("Deleting pod", "namespace", pod.Namespace, "name", pod.Name)
	controller.queue.Add(pod)
}

func (controller *LogController) runWorker() {
	for controller.processNextWorkItem() {
	}
}

// INFO: 默认每分钟运行一次，捞出queue里所有pod，根据 input.template.yml 渲染出 input.yml
func (controller *LogController) processNextWorkItem() bool {
	pods := []*corev1.Pod{}
	length := controller.queue.Len()
	if length == 0 {
		return false
	}
	klog.Infof("[processNextWorkItem]%d queue items are processed", length)
	for i := 0; i < length; i++ {
		pod, shutdown := controller.queue.Get()
		if shutdown {
			break
		}
		pods = append(pods, pod.(*corev1.Pod))
		controller.queue.Done(pod.(*corev1.Pod))
	}
	controller.syncPod(pods)

	return true
}

func (controller *LogController) syncPod(pods []*corev1.Pod) {
	for _, pod := range pods {
		// skip non-log pod
		if filebeat.Skip(pod.Annotations) {
			klog.Info(pod.Name, " do not need collect log")
			continue
		}
		logConfig, err := filebeat.DecodeLogConfig(pod.Annotations[common.LscAnnotationName])
		if err != nil {
			klog.Errorf("failed to parse pod(%s) annotations.err:%w", pod.Name, err)
			continue
		}
		if !filebeat.IsCollectLog(*logConfig, common.DaemonsetMode) {
			klog.Info(pod.Name, " do not need collect log by daemonset")
			continue
		}
		if !filebeat.LogConfigVaild(logConfig) {
			klog.Errorln(pod.Name, "lack of necessary fileds.skip to handle.")
			continue
		}

		key, err := KeyFunc(pod)
		if err != nil {
			klog.Errorf("[syncPod]KeyFunc(%s) error %w", pod.Name, err)
			continue
		}

		// TODO: 背景，pod 被完全删除后，docker container logs 文件也会被删除
		//  对于正在删除中的 terminating pod，log-controller 不应该去删除该 pod 的 input
		//  应该是等 pod 被完全删除后，log-controller 再去删除 input.yml 中该 pod 的 input
		//  @see https://github.com/elastic/beats/pull/20084
		//  https://github.com/elastic/beats/issues/17396
		//  https://github.com/elastic/beats/pull/14259/
		localPod, err := controller.podLister.Pods(pod.Namespace).Get(pod.Name)
		if err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("unable to retrieve pod %s/%s from local store: %w", pod.Namespace, pod.Name, err)
				continue
			}
			delete(controller.pods, key)
			controller.changed = true
			klog.Infof("[syncPod]pod %s is deleted from pod cache.err=%w", key, err)
			continue
		}

		// INFO: 重新放入queue这个逻辑是对的，只有 running pod 才会放到 pod cache 里:
		//  对于刚刚创建的和删除的pod，都是 not ready pod
		//  对于 preStop pod，起初是 ready 的，最后是 not ready pod

		// TODO: 这里需要测试下对于 preStop pod，kubectl delete 时不会去更新 input.yml，等待 preStop 之后才回去更新
		//apiextensions.ConditionTrue
		if !podutils.IsPodReady(pod) {
			controller.queue.AddAfter(localPod, time.Second*5)
			continue
		}

		controller.pods[key] = logConfig
		controller.changed = true
		klog.Infof("[syncPod]pod %s is added into pod cache", key)
	}

	// sync filebeat input yaml if pod cache changed
	if !controller.changed {
		return
	}

	filebeatInputs := controller.getFilebeatInputs(controller.pods)
	if filebeatInputs == nil {
		klog.Error("filebeatInputs is null.")
		return
	}

	buffer := bytes.NewBufferString("")
	err := controller.tpl.Execute(buffer, &filebeatInputs)
	if err != nil {
		klog.Errorf("[syncPod]fail to render filebeat template err %w", err)
		return
	}

	err = os.WriteFile(common.FinalFilebeatInputsConfigPath, buffer.Bytes(), fs.ModePerm)
	if err != nil {
		klog.Errorf("[syncPod]fail to sync filebeat input yml err %w", err)
		return
	}

	klog.Infof("updated filebeat input.yml at %s", time.Now().String())
	controller.changed = false

}

func (controller *LogController) getFilebeatInputs(pods map[string]*common.LSCConfig) *common.FilebeatInputConfigs {
	var inputs []common.FilebeatInputsData
	for podKey, logConfig := range pods {
		namespace, name, err := cache.SplitMetaNamespaceKey(podKey)
		if err != nil {
			klog.Errorf("[getFilebeatInputs]SplitMetaNamespaceKey err %w", err)
			continue
		}
		pod, err := controller.podLister.Pods(namespace).Get(name)
		if err != nil {
			klog.Errorf("[getFilebeatInputs]get pods err %w", err)
			continue
		}

		for containerName, volumeLogConfig := range logConfig.ContainerLogConfigs {
			queryOrderSpecInfo := filebeat.CalculateHowToMount(volumeLogConfig)
			for _, volumePathConfig := range volumeLogConfig {
				//只处理daemonset模式的配置
				if volumePathConfig.LogCollectorType == common.SidecarMode {
					continue
				}
				// 检查kafka-topic是否存在
				err = common.CheckTopic(volumePathConfig.Hosts, volumePathConfig.Topic)
				if err != nil {
					klog.Errorf("topic err, podName: %s, podNamespace: %s, kafkaHost: %s, topic: %s, err message: %v\n",
						pod.Name,
						pod.Namespace,
						volumePathConfig.Hosts,
						volumePathConfig.Topic,
						err)
					continue
				}

				//区分 stdout 和 fileLog
				paths := []string{}
				if volumePathConfig.LogType == common.StdoutMode { // stdout
					klog.Infof("[std log] start to handle pod(%s) in ns(%s).\n", pod.Name, pod.Namespace)
					cur := controller.handleStdLog(pod, containerName)
					paths = append(paths, cur...)
				} else {
					klog.Infof("[file log] start to handle pod(%s) in ns(%s).\n", pod.Name, pod.Namespace)
					cur := controller.handleFileLog(pod.UID, containerName, volumePathConfig, queryOrderSpecInfo)
					paths = append(paths, cur...)
				}

				appName, owners := kube.GetOwner(pod, controller.kubeDynamicCli, controller.kubecli, pod.Namespace, controller.restMapper)
				kind := ""
				if len(owners) > 0 {
					kind = owners[0].Kind
				}
				prefix := ""
				// pathConfig.Codec == "wzFormat"时，日志格式为原wz模式
				// 匹配原Kafka日志收集格式前缀: [idc=**,app=**,pod=**,filename=**]
				if volumePathConfig.Codec == common.LogWZFormat {
					if len(paths) == 1 {
						prefix = fmt.Sprintf("\"[IDC=%s,app=%s,pod=%s,filename=%s] \"",
							os.Getenv("IDC"),
							pod.Labels["app"],
							pod.Name,
							filebeat.Base(paths[0]))
					} else {
						prefix = fmt.Sprintf("\"[IDC=%s,app=%s,pod=%s] \"",
							os.Getenv("IDC"),
							pod.Labels["app"],
							pod.Name)
					}
				}

				input := common.FilebeatInputsData{
					Hosts:           volumePathConfig.Hosts,
					Paths:           paths,
					Topic:           volumePathConfig.Topic,
					Codec:           volumePathConfig.Codec,
					MultilineEnable: volumePathConfig.MultilineEnable,
					CustomField: fmt.Sprintf("cluster=%s,%s=%s,pod=%s,ip=%s,namespace=%s,container_name=%s",
						os.Getenv("IDC"), kind, appName, pod.Name, pod.Status.PodIP, pod.Namespace, containerName),
					Prefix: prefix,
				}
				if volumePathConfig.MultilineEnable {
					input.MultilinePattern = common.MultilineConfig{
						MulPattern: volumePathConfig.MultilinePattern.MulPattern,
						MulNegate:  volumePathConfig.MultilinePattern.MulNegate,
						MulMatch:   volumePathConfig.MultilinePattern.MulMatch,
					}
				}
				inputs = append(inputs, input)
			}
		}

	}

	return &common.FilebeatInputConfigs{FBInputs: inputs}
}

func (controller *LogController) handleStdLog(pod *corev1.Pod, containerName string) []string {
	paths := make([]string, 0)

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Name != containerName { // skip containerName
			continue
		}

		if len(strings.Split(containerStatus.ContainerID, "//")) > 1 {
			containerID := strings.Split(containerStatus.ContainerID, "//")[1]
			//cri为docker的pod中容器stdout的内容存在宿主机的目录：/var/lib/docker/containers/<容器id>/<容器id>-json.log
			p := fmt.Sprintf("%s/%s/%s-json.log", controller.option.DockerRootPath, containerID, containerID)
			if _, err := os.Stat(p); os.IsNotExist(err) {
				//cri为containerd的pod中容器标准输出的内容在宿主机的目录：/var/log/pods/<namespace>_<podName>_<podUID>/<containerName>/0.log
				p = fmt.Sprintf("%s/%s_%s_%s/%s", controller.option.ContainerdRootPath, pod.Namespace, pod.Name, pod.UID, containerName)
				lastestUpdateFileName, err := tools.GetLastestUpdateFile(p)
				if err != nil {
					continue
				}

				p = filepath.Join(p, lastestUpdateFileName)
			}
			paths = append(paths, p)
		}
	}

	return paths
}

func (controller *LogController) handleFileLog(podUid apitypes.UID, containerName string, volumePathConfig common.VolumePathConfig, queryOrderSpecInfo filebeat.QueryOrderSpec) []string {
	paths := make([]string, 0)
	for _, logRelPath := range volumePathConfig.Paths {
		if logRelPath = strings.TrimSpace(logRelPath); logRelPath == "" {
			continue
		}

		//pod中emptydir在宿主机的目录：/var/lib/kubelet/pods/<PODID>/Volumes/Kubernetes.io~empty-dir/<VOLUME_NAME>
		parentPath := fmt.Sprintf("%s/%s/volumes/kubernetes.io~empty-dir/log-volume/logpath-%s", controller.option.KubeletRootPath, podUid, containerName)
		p := fmt.Sprintf("%s/%s", parentPath, filebeat.Base(logRelPath))
		// 这里是为了兼容老的日志处理逻辑
		if _, err := os.Stat(parentPath); os.IsNotExist(err) {
			subPath := filebeat.CalSubPath(queryOrderSpecInfo, logRelPath, containerName)
			p = fmt.Sprintf("%s/%s/volumes/kubernetes.io~empty-dir/log-volume/%s/%s", controller.option.KubeletRootPath, podUid, subPath, filebeat.Base(logRelPath))
		}

		paths = append(paths, p)
	}

	return paths
}

func (controller *LogController) RunUntil(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	klog.Info("start log controller")

	go controller.podInformer.Run(stopCh)
	shutdown := cache.WaitForCacheSync(stopCh, controller.podInformer.HasSynced)
	if !shutdown {
		klog.Errorf("can not sync pods in node %s", controller.option.Nodename)
		return nil
	}

	klog.Infof("cache has synced")
	for i := 0; i < threadiness; i++ {
		go wait.Until(controller.runWorker, controller.option.SyncPeriod, stopCh)
	}

	<-stopCh
	return nil
}

package configmapctrl

import (
	"context"
	"corp.wz.net/opsdev/log-collection/pkg/common"
	kube "corp.wz.net/opsdev/log-collection/pkg/common/kubernetes"
	"corp.wz.net/opsdev/log-collection/pkg/filebeat"
	"corp.wz.net/opsdev/log-collection/pkg/kubernets"
	"errors"
	"fmt"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listcorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"os"
	"time"
)

type ConfigmapController struct {
	restMapper        meta.RESTMapper
	kubeClient        kubernetes.Interface
	kubeDynamicClient dynamic.Interface
	queue             workqueue.RateLimitingInterface
	podLister         listcorev1.PodLister
	podInformer       cache.SharedIndexInformer
}

func New(kubeClient kubernetes.Interface, dynamicClient dynamic.Interface, period time.Duration) (*ConfigmapController, error) {
	factory := informers.NewSharedInformerFactory(kubeClient, period)
	controller := ConfigmapController{
		kubeClient:        kubeClient,
		kubeDynamicClient: dynamicClient,
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "log_sidecar_helper"),
		podInformer:       factory.Core().V1().Pods().Informer(),
		podLister:         factory.Core().V1().Pods().Lister(),
	}

	groupResources, err := restmapper.GetAPIGroupResources(kubeClient.Discovery())
	if err != nil {
		klog.Errorf("failed to get gvr,error=%v\n", err)
		return &controller, err
	}
	rm := restmapper.NewDiscoveryRESTMapper(groupResources)
	controller.restMapper = rm

	factory.Core().V1().Pods().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.add,
		UpdateFunc: controller.update,
	})

	return &controller, nil
}

func (c *ConfigmapController) Run(workers int, period time.Duration, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()

	go c.podInformer.Run(stopCh)

	if shutdown := cache.WaitForCacheSync(stopCh, c.podInformer.HasSynced); !shutdown {
		return errors.New("failed to sync deployment")
	}

	klog.Info("Starting the workers of the filebeat configmap controller")
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, period, stopCh)
	}

	defer func() {
		<-stopCh
	}()

	return nil
}

func (c *ConfigmapController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *ConfigmapController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncHandler(context.Background(), key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("failed to sync configmap of filebeat %q: %w", key, err))
	return true
}

func (c *ConfigmapController) syncHandler(ctx context.Context, key string) error {
	klog.Info("start handle key:", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("failed to split meta namespace cache key.cacheky:%s.err=%w", key, err)
	}

	pod, err := c.podLister.Pods(namespace).Get(name)
	if err != nil || pod == nil {
		return fmt.Errorf("failed to get pod %s in %s namespace", name, namespace)
	}
	if filebeat.Skip(pod.Annotations) {
		return nil
	}

	logconfig, err := filebeat.DecodeLogConfig(pod.Annotations[common.LscAnnotationName])
	if err != nil || logconfig == nil {
		return err
	}

	if !filebeat.IsCollectLog(*logconfig, common.SidecarMode) {
		return nil
	}

	//controller, err := kube.GetOuterMostController(c.kubeClient, pod, pod.Namespace)
	//if err != nil {
	//	return err
	//}

	appName, owners := kube.GetOwner(pod, c.kubeDynamicClient, c.kubeClient, pod.Namespace, c.restMapper)
	err = createConfigMap(ctx, c.kubeClient, pod, owners, appName)
	if err != nil {
		return err
	}

	return nil
}

func (c *ConfigmapController) add(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error("[add] failed to get configmap meta name err:", err)
		return
	}

	c.queue.Add(key)
}

func (c *ConfigmapController) update(old interface{}, new interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(new)
	if err != nil {
		klog.Error("[update] failed to get configmap meta name err:", err)
		return
	}

	c.queue.Add(key)
}

func (c *ConfigmapController) delete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error("[delete] failed to get configmap meta name err:", err)
		return
	}

	c.queue.Add(key)
}

//通过configmap挂载filebeat配置的相关模板，
//根据deployment的annotation生成最终要用配置，
//为sidecar模式收集日志的pod创建所需的configmap，
//这个configmap存着filebeat需要的配置，包括filebeat.yml和inputs.yml
func createConfigMap(ctx context.Context, kubecli kubernetes.Interface, pod *coreV1.Pod, owner []metav1.OwnerReference, appName string) error {
	lscConfig, err := filebeat.DecodeLogConfig(pod.Annotations[common.LscAnnotationName])
	if err != nil {
		return fmt.Errorf("failed to decode log config.err=%w", err)
	}

	kind := ""
	if len(owner) > 0 {
		kind = owner[0].Kind
	}

	i := common.InputsData{
		ContainerLogConfigs: lscConfig.ContainerLogConfigs,
		CustomField: fmt.Sprintf("cluster=%s,%s=%s,pod=${HOSTNAME},ip=${IP},namespace=%s",
			os.Getenv("IDC"),
			kind,
			appName,
			pod.Namespace,
		),
		Prefix: fmt.Sprintf("IDC=%s,app=%s,pod=%s",
			os.Getenv("IDC"),
			pod.Labels["app"],
			pod.Name),
	}

	inputsConf, err := filebeat.Parse([]common.InputsData{i})
	if err != nil {
		return fmt.Errorf("failed to parse inputs data.err=%w", err)
	}
	filebeatConf, err := os.ReadFile(common.FilebeatConfigTplPath)
	if err != nil {
		return err
	}

	cm := coreV1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "v1",
			APIVersion: "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            appName + "-log-sidecar-cm",
			Namespace:       pod.Namespace,
			Labels:          pod.Labels,
			OwnerReferences: owner,
		},
		Data: map[string]string{
			"inputs.yml.template": inputsConf,
			"filebeat.yml":        string(filebeatConf),
		},
	}

	return kubernets.CreateOrUpdateConfigMap(kubecli, ctx, &cm)
}

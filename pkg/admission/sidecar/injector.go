package sidecar

import (
	kube "corp.wz.net/opsdev/log-collection/pkg/common/kubernetes"
	"corp.wz.net/opsdev/log-collection/pkg/filebeat"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	_ "k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type Injector struct {
	KubeClient        kubernetes.Interface
	KubeDynamicClient dynamic.Interface
	RestMapper        meta.RESTMapper
}

func NewInjector(k8sclient kubernetes.Interface, dynamicClient dynamic.Interface, restMapper meta.RESTMapper) *Injector {
	return &Injector{k8sclient, dynamicClient, restMapper}
}

func (i *Injector) Mutate(pod *corev1.Pod, namespace string) {
	if filebeat.Skip(pod.Annotations) {
		return
	}

	//controller, err := kube.GetOuterMostController(i.kubecli, pod, namespace)
	//if err != nil {
	//	klog.Error(err)
	//	return
	//}
	//appName := controller.ObjectMeta.GetName()
	appName, _ := kube.GetOwner(pod, i.KubeDynamicClient, i.KubeClient, namespace, i.RestMapper)
	klog.Info("start handle pod:", pod.GenerateName)
	i.ensurePod(pod, appName)
}

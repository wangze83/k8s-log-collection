package kubernetes

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type Object struct {
	ObjectMeta metav1.Object
	TypeMeta   runtime.Object
}

//这个方法目前只处理k8s原生cotroller创建的pod
func GetOuterMostController(kubecli kubernetes.Interface, pod *corev1.Pod, namespace string) (Object, error) {
	if namespace == "" && pod.Namespace == "" {
		namespace = corev1.NamespaceDefault
	}

	for _, ownerReference := range pod.OwnerReferences {
		if ownerReference.Kind == "StatefulSet" {
			sts, err := kubecli.AppsV1().StatefulSets(namespace).Get(context.Background(), ownerReference.Name, metav1.GetOptions{})
			if err != nil {
				return Object{}, err
			}
			sts.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "StatefulSet",
			})
			return Object{
				ObjectMeta: sts,
				TypeMeta:   sts,
			}, err
		} else if ownerReference.Kind == "DaemonSet" {
			daemonset, err := kubecli.AppsV1().DaemonSets(namespace).Get(context.Background(), ownerReference.Name, metav1.GetOptions{})
			if err != nil {
				return Object{}, err
			}
			daemonset.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "DaemonSet",
			})
			return Object{
				ObjectMeta: daemonset,
				TypeMeta:   daemonset,
			}, err
		} else if ownerReference.Kind == "ReplicaSet" {
			rs, err := kubecli.AppsV1().ReplicaSets(namespace).Get(context.Background(), ownerReference.Name, metav1.GetOptions{})
			if err != nil {
				return Object{}, err
			}
			if len(rs.OwnerReferences) == 0 {
				rs.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "apps",
					Version: "v1",
					Kind:    "ReplicaSet",
				})
				return Object{
					ObjectMeta: rs,
					TypeMeta:   rs,
				}, err
			}
			exist := false
			var deploy *appsv1.Deployment
			for _, ownerRef := range rs.OwnerReferences {
				if ownerRef.Kind == "Deployment" {
					deploy, err = kubecli.AppsV1().Deployments(namespace).Get(context.Background(), ownerRef.Name, metav1.GetOptions{})
					if err != nil {
						continue
					}
					exist = true
				}
			}
			if exist {
				deploy.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "apps",
					Version: "v1",
					Kind:    "Deployment",
				})
				return Object{
					ObjectMeta: deploy,
					TypeMeta:   deploy,
				}, err
			}
			return Object{}, fmt.Errorf("pod(%s) create by rs,but deployment kind not find in the rs ownerRferences", pod.Name)
		} else if ownerReference.Kind == "Job" {
			job, err := kubecli.BatchV1().Jobs(namespace).Get(context.Background(), ownerReference.Name, metav1.GetOptions{})
			if err != nil {
				return Object{}, err
			}
			if len(job.OwnerReferences) == 0 {
				job.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "batch",
					Version: "v1",
					Kind:    "Job",
				})
				return Object{
					ObjectMeta: job,
					TypeMeta:   job,
				}, err
			}
			exist := false
			var cronJob *batchv1.CronJob
			for _, ownerRef := range job.OwnerReferences {
				if ownerRef.Kind == "CronJob" {
					cronJob, err = kubecli.BatchV1().CronJobs(namespace).Get(context.Background(), ownerRef.Name, metav1.GetOptions{})
					if err != nil {
						continue
					}
					exist = true
				}
			}
			if exist {
				cronJob.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "batch",
					Version: "v1",
					Kind:    "CronJob",
				})
				return Object{
					ObjectMeta: cronJob,
					TypeMeta:   cronJob,
				}, err
			}
			return Object{}, fmt.Errorf("pod(%s) create by job(%s),but cronjob kind not find in the ownerRferences of job", pod.Name, job.Name)
		}
	}

	return Object{}, fmt.Errorf("pod %s in %s have no ownerReferences", pod.Name, namespace)
}

//递归获取最外层controller
func GetOutestOwner(rm meta.RESTMapper, ns string, dyClient dynamic.Interface, kubeClient kubernetes.Interface, object *unstructured.Unstructured, result *[]metav1.OwnerReference) {
	owners := object.GetOwnerReferences()
	if owners == nil || len(owners) == 0 {
		ref := metav1.NewControllerRef(object, object.GroupVersionKind())
		*result = append(*result, *ref)
		return
	}

	for _, owner := range owners {
		if owner.Controller == nil {
			continue
		}
		apiversionList := strings.Split(owner.APIVersion, "/")
		var group, version string
		if len(apiversionList) == 2 {
			group, version = apiversionList[0], apiversionList[1]
		} else if len(apiversionList) == 1 {
			version = apiversionList[0]
		}
		gk := schema.GroupKind{
			Group: group,
			Kind:  owner.Kind,
		}

		mapping, err := rm.RESTMapping(gk, version)
		parentObj, err := dyClient.Resource(mapping.Resource).
			Namespace(ns).
			Get(context.Background(), owner.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("failed to get resouces(%s:%s)in ns(%s),error=%v", gk.String(), owner.Name, ns, err)
			return
		}

		GetOutestOwner(rm, ns, dyClient, kubeClient, parentObj, result)
	}
}

func GetOwner(pod *corev1.Pod, dynamicClient dynamic.Interface, kubeClient kubernetes.Interface, namespace string, restMapper meta.RESTMapper) (string, []metav1.OwnerReference) {
	if namespace == "" {
		namespace = corev1.NamespaceDefault
	}

	appName := ""
	owners := make([]metav1.OwnerReference, 0)

	if _, ok := pod.Labels["app"]; ok {
		appName = pod.Labels["app"]
	}

	unstructuredPod, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
	if err != nil {
		klog.Errorf("failed to convert pod to unstructed.error=%v", err)
		return appName, owners
	}

	GetOutestOwner(restMapper, namespace, dynamicClient, kubeClient, &unstructured.Unstructured{unstructuredPod}, &owners)
	if len(owners) > 0 {
		appName = owners[0].Name
	}

	return appName, owners
}

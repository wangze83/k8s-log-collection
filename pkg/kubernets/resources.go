package kubernets

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CreateOrUpdateConfigMap(kubecli kubernetes.Interface, ctx context.Context, configMap *corev1.ConfigMap) error {
	storedConfigMap, err := kubecli.CoreV1().ConfigMaps(configMap.Namespace).Get(ctx, configMap.Name, metav1.GetOptions{})
	if err != nil {
		// If no resource we need to create.
		if errors.IsNotFound(err) {
			_, err := kubecli.CoreV1().ConfigMaps(configMap.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
			return err
		}
		return err
	}

	configMap.ResourceVersion = storedConfigMap.ResourceVersion
	_, err = kubecli.CoreV1().ConfigMaps(configMap.Namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	return err
}

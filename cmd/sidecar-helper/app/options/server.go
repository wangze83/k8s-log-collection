package options

import (
	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
)

func NewCommand(stopChan <-chan struct{}) *cobra.Command {
	opts := Options{}
	cmd := &cobra.Command{
		Use:  "sidecar-helper",
		Long: "create cm for pod that collect logs by sidecar",
		RunE: func(c *cobra.Command, args []string) error {
			if err := start(opts, stopChan); err != nil {
				klog.Errorf("run command err: %w", err)
				return err
			}
			return nil
		},
	}
	opts.Flags(cmd)
	return cmd
}

func start(option Options, stopC <-chan struct{}) error {
	restConfig, err := newRestConfig(option.Kubeconfig)
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	controller, err := configmapctrl.New(kubeClient, dynamicClient, option.SyncPeriod)
	if err != nil {
		return err
	}

	return controller.Run(1, option.SyncPeriod, stopC)
}

func newRestConfig(kubeconfig string) (*rest.Config, error) {
	var config *rest.Config
	if _, err := os.Stat(kubeconfig); err == nil {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	return config, nil
}

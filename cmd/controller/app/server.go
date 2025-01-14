package app

import (
	"corp.wz.net/opsdev/log-collection/cmd/controller/app/options"
	kube "corp.wz.net/opsdev/log-collection/pkg/common/kubernetes"
	"corp.wz.net/opsdev/log-collection/pkg/log/daemonset/controller"
	"fmt"
	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

func NewLogDaemonsetCommand(stopCh <-chan struct{}) *cobra.Command {
	opts := options.NewOptions()
	cmd := &cobra.Command{
		Use:  "log-controller",
		Long: "log controller for controller type",
		RunE: func(c *cobra.Command, args []string) error {
			if err := runCommand(opts, stopCh); err != nil {
				klog.Errorf("run command err: %w", err)
				return err
			}
			return nil
		},
	}
	opts.Flags(cmd)

	return cmd
}

func runCommand(option *options.Options, stopCh <-chan struct{}) error {
	restConfig, err := kube.NewRestConfig(option.Kubeconfig)
	if err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("unable to construct lister client: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	s, err := controller.NewLogController(kubeClient, dynamicClient, option)
	if err != nil {
		return err
	}

	klog.Info("starting run server...")
	return s.RunUntil(1, stopCh)
}

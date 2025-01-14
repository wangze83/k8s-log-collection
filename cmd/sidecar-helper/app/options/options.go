package options

import (
	"github.com/spf13/cobra"
	"time"
)

type Options struct {
	// path to the kubeconfig used to connect to the Kubernetes API server
	Kubeconfig string
	Debug      bool
	SyncPeriod time.Duration
}

func (o *Options) Flags(cmd *cobra.Command) {
	flag := cmd.Flags()
	flag.BoolVar(&o.Debug, "debug", false, "debug for skip updating container cpuset")
	flag.StringVar(&o.Kubeconfig, "kubeconfig", "~/.kube/config", "The path to the kubeconfig used to connect to the Kubernetes API server and the Kubelets (defaults to in-cluster config)")
	flag.DurationVar(&o.SyncPeriod, "sync-period", 0, "requeue resource period.")
}

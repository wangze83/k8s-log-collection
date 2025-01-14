package options

import (
	"github.com/spf13/cobra"
	"os"
	"time"
)

type Options struct {
	// path to the kubeconfig used to connect to the Kubernetes API server
	Kubeconfig string

	Debug bool

	// current node name
	Nodename string

	SyncPeriod time.Duration

	DockerRootPath     string
	ContainerdRootPath string
	KubeletRootPath    string
}

func (o *Options) Flags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.BoolVar(&o.Debug, "debug", false, "debug for skip updating container cpuset")
	flags.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "The path to the kubeconfig used to connect to the Kubernetes API server and the Kubelets (defaults to in-cluster config)")
	flags.StringVar(&o.Nodename, "nodename", o.Nodename, "current node name")
	flags.DurationVar(&o.SyncPeriod, "sync-period", 0, "requeue resource period.")
	flags.StringVar(&o.DockerRootPath, "docker-root-path", "/data/docker/containers", "root path of the Docker runtime")
	flags.StringVar(&o.ContainerdRootPath, "containerd-root-path", "/data/containerd/containers", "root path of the containerd runtime")
	flags.StringVar(&o.KubeletRootPath, "kubelet-root-path", "/data/kubelet/pods", `root of the kubelet runtime.`)
}

func NewOptions() *Options {
	return &Options{
		Nodename: os.Getenv("NODENAME"),
	}
}

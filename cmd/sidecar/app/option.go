package app

import (
	"corp.wz.net/opsdev/log-collection/pkg/common"
	"github.com/spf13/pflag"
	"time"
)

type Options struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	CertFile     string
	KeyFile      string
	KubeConfig   string
}

func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&o.Port, "port", 8443, "port.")
	fs.DurationVar(&o.ReadTimeout, "readtimeout", 30*time.Second, "read time out.")
	fs.DurationVar(&o.WriteTimeout, "writetimeout", 30*time.Second, "write time out.")
	fs.StringVar(&o.CertFile, "tlscert", "/etc/logsidecar/tls.crt", "tls cert file")
	fs.StringVar(&o.KeyFile, "tlskey", "/etc/logsidecar/tls.key", "tls key file")
	fs.StringVar(&o.KubeConfig, "kubeconfig", "~/.kube/config", "kube config")
	fs.StringVar(&common.LogSidecarImage, "lsc-image", "elastic/filebeat:6.7.0", "filebeat image")
}

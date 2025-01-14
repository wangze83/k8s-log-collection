package main

import (
	"corp.wz.net/opsdev/log-collection/cmd/controller/app"
	"flag"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"
)

// go run . --debug=true --kubeconfig=`echo ~`/.kube/config --nodename=docker05.idc2.net
func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	cmd := app.NewLogDaemonsetCommand(genericapiserver.SetupSignalHandler())
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}

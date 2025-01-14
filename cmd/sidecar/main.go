/*
 *     Copyright 2021
 */

package main

import (
	"corp.wz.net/opsdev/log-collection/cmd/sidecar/app"
	"flag"
)

func main() {
	cmd := app.NewCommand()
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}

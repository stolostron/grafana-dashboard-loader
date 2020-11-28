// Copyright (c) 2020 Red Hat, Inc.

package main

import (
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/pflag"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/component-base/version"

	"github.com/open-cluster-management/grafana-dashboard-loader/pkg/controller"
)

func main() {

	rand.Seed(time.Now().UTC().UnixNano())

	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	logs.InitLogs()
	defer logs.FlushLogs()

	command := controllercmd.
		NewControllerCommandConfig("grafana-dashboard-loader", version.Get(), controller.RunGrafanaDashboardController).
		NewCommand()
	command.Use = "grafana-dashboard-loader"
	command.Short = "Start the grafana dashboard loader"

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

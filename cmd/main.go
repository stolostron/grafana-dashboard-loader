// Copyright (c) 2020 Red Hat, Inc.

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	"github.com/open-cluster-management/grafana-dashboard-loader/pkg/loader"
)

type config struct {
	kubeconfigPath string
}

func main() {

	cfg := config{}

	klogFlags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	klog.InitFlags(klogFlags)
	flagset := pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	flagset.AddGoFlagSet(klogFlags)

	//Kubeconfig flag
	flagset.StringVar(&cfg.kubeconfigPath, "kubeconfig-path", "",
		"Path to a kubeconfig file. If unset, in-cluster configuration will be used")

	dl := loader.NewDashboardLoader(getKubernetesClients(cfg))
	dl.WatchDashboardConfigMaps()

	// use a channel to synchronize the finalization for a graceful shutdown
	stop := make(chan struct{})
	defer close(stop)

	go dl.Run(stop)

	// use a channel to handle OS signals to terminate and gracefully shut
	// down processing
	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	signal.Notify(sigTerm, syscall.SIGINT)
	<-sigTerm

}

// getKubernetesClients retrieve the Kubernetes cluster client
func getKubernetesClients(cfg config) *kubernetes.Clientset {

	// create the config from the path
	config, err := clientcmd.BuildConfigFromFlags("", cfg.kubeconfigPath)
	if err != nil {
		log.Fatalf("getClusterConfig: %v-%v", err, cfg.kubeconfigPath)
	}

	// generate the client based off of the config
	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("kubernetes.NewForConfig: %v", err)
	}
	return kclient
}

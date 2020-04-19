package main

import (
	"flag"
	"os"
	"fmt"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"github.com/cloud-ark/kubediscovery/pkg/cmd/server"
	"github.com/cloud-ark/kubediscovery/pkg/discovery"
)

func main() {

	if len(os.Args) == 4 {
		kind := os.Args[1]
		instance := os.Args[2]
		namespace := os.Args[3]
		discovery.BuildCompositionTree(namespace)
		composition := discovery.TotalClusterCompositions.GetCompositions(kind, 
																		  instance, 
																		  namespace)
		fmt.Printf("%s\n", composition)
	} else {
		fmt.Printf("Running from within cluster.\n")
		stopCh := genericapiserver.SetupSignalHandler()
		options := server.NewDiscoveryServerOptions(os.Stdout, os.Stderr)
		cmd := server.NewCommandStartDiscoveryServer(options, stopCh)
		cmd.Flags().AddGoFlagSet(flag.CommandLine)
		if err := cmd.Execute(); err != nil {
			panic(err)
		}
	}
}

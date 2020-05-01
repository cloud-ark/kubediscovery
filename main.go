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
	if len(os.Args) == 5  || len(os.Args) == 3 {
		var kind, instance, namespace string
		// kubediscovery composition Moodle moodle1 default
		// kubediscovery connections Moodle moodle1 default
		// kubediscovery man Moodle
		commandType := os.Args[1]
		if commandType == "composition" {
			kind = os.Args[2]
			instance = os.Args[3]
			namespace = os.Args[4]
			discovery.BuildCompositionTree(namespace)
			composition := discovery.TotalClusterCompositions.GetCompositions(kind,
																			  instance,
																			  namespace)
			fmt.Printf("%s\n", composition)
		}
		if commandType == "connections" {
			kind = os.Args[2]
			instance = os.Args[3]
			namespace = os.Args[4]
			level := 0
			relatives := discovery.GetRelatives(level, kind, instance, namespace)
			for _, relative := range relatives {
				fmt.Printf("%s\n", relative)
			}
		}
		if commandType == "man" {
			kind := os.Args[2]
			fmt.Printf("TODO: Implement man endpoint: %s\n", kind)
		}
		if commandType != "composition" && commandType != "connections" && commandType != "man" {
			fmt.Printf("Unknown command specified:%s\n", commandType)
			fmt.Printf("Allowed values: [composition, connections, man]\n")
		}
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
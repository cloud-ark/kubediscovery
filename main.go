package main

import (
	"flag"
	"os"
	"fmt"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"github.com/cloud-ark/kubediscovery/pkg/cmd/server"
	"github.com/cloud-ark/kubediscovery/pkg/discovery"
	"github.com/cloud-ark/kubediscovery/pkg/apiserver"
)

func main() {
	if len(os.Args) == 7  || len(os.Args) == 5 || len(os.Args) == 3 {
		var kind, instance, namespace string
		// kubediscovery connections Moodle moodle1 default -o flat
		// kubediscovery composition Moodle moodle1 default
		// kubediscovery man Moodle 
		commandType := os.Args[1]
		if commandType == "composition" {
			kind = os.Args[2]
			instance = os.Args[3]
			namespace = os.Args[4]
			discovery.BuildCompositionTree(namespace)
			composition := discovery.TotalClusterCompositions.GetCompositionsString(kind,
																			  instance,
																			  namespace)
			fmt.Printf("%s\n", composition)
		}
		if commandType == "connections" {
			kind = os.Args[2]
			instance = os.Args[3]
			namespace = os.Args[4]
			format := "default"
			if len(os.Args) == 7 {
				format = os.Args[6]
			}
			level := 0
			connections := make([]discovery.Connection, 0)
			relationType := ""
			origkind := kind
			originstance := instance
			/*rootNode := discovery.Connection{
				Name: instance,
				Kind: kind,
				Namespace: namespace,
				Level: 0,
			}*/
			discovery.BuildCompositionTree(namespace)
			//discovery.TotalClusterConnections = discovery.AppendConnections(discovery.TotalClusterConnections, rootNode)
			connections = discovery.GetRelatives(connections, level, kind, instance, origkind, originstance, 
												 namespace, relationType)
			if len(discovery.TotalClusterConnections) > 0 {
				discovery.PrintRelatives(format, discovery.TotalClusterConnections)
			}
		}
		if commandType == "man" {
			kind := os.Args[2]
			manPage := apiserver.GetManPage(kind)
			fmt.Printf("%s\n", manPage)
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
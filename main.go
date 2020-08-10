package main

import (
//	"flag"
	"os"
	"fmt"
	"time"
//	genericapiserver "k8s.io/apiserver/pkg/server"
//	"github.com/cloud-ark/kubediscovery/pkg/cmd/server"
	"github.com/cloud-ark/kubediscovery/pkg/discovery"
	"github.com/cloud-ark/kubediscovery/pkg/apiserver"
)

func main() {
	//if len(os.Args) == 7  || len(os.Args) == 5 || len(os.Args) == 3 {
	if len(os.Args) >= 2 {
		var kind, instance, namespace string
		// kubediscovery connections Moodle moodle1 default -o flat
		// kubediscovery composition Moodle moodle1 default
		// kubediscovery man Moodle 
		commandType := os.Args[1]
		if commandType == "composition" {
			if len(os.Args) != 5 {
				panic("Not enough arguments: ./kubediscovery composition <kind> <instance> <namespace>")
			}
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
			if len(os.Args) < 5  {
				panic("Not enough arguments:./kubediscovery connections <kind> <instance> <namespace>")
			}
			kind = os.Args[2]
			instance = os.Args[3]
			namespace = os.Args[4]
			discovery.OutputFormat = "default"
			if len(os.Args) == 7 {
				discovery.OutputFormat = os.Args[6]
			}
			level := 0
			visited := make([]discovery.Connection, 0)
			relationType := ""
			discovery.OrigKind = kind
			discovery.OrigName = instance
			discovery.OrigNamespace = namespace
			discovery.OrigLevel = level
			discovery.BuildCompositionTree(namespace)
			root := discovery.Connection{
				Name: instance,
				Kind: kind,
				Namespace: namespace,
				Level: level,
				Peer: &discovery.Connection{
					Name: "",
					Kind: "",
					Namespace: "",
				},
			}
			discovery.TotalClusterConnections = discovery.AppendConnections(discovery.TotalClusterConnections, root)

			level = level + 1
			visited = discovery.GetRelatives(visited, level, kind, instance, discovery.OrigKind, discovery.OrigName, 
												 namespace, relationType)
			if len(discovery.TotalClusterConnections) > 0 {
				discovery.PrintRelatives(discovery.OutputFormat, discovery.TotalClusterConnections)
			}
		}
		if commandType == "man" {
			if len(os.Args) != 3 {
				panic("Not enough arguments:./kubediscovery man <kind>.")
			}
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
		fmt.Printf("Installing KubePlus paths.\n")
		go apiserver.InstallKubePlusPaths()
		fmt.Printf("After installing KubePlus paths.\n")
		// Run forever
		for {
			time.Sleep(60*time.Second)
		}
		/*stopCh := genericapiserver.SetupSignalHandler()
		options := server.NewDiscoveryServerOptions(os.Stdout, os.Stderr)
		cmd := server.NewCommandStartDiscoveryServer(options, stopCh)
		cmd.Flags().AddGoFlagSet(flag.CommandLine)
		if err := cmd.Execute(); err != nil {
			panic(err)
		}*/
	}
}
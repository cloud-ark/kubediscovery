package main

import (
//	"flag"
	"os"
	"fmt"
	"time"
	"strings"
//	genericapiserver "k8s.io/apiserver/pkg/server"
//	"github.com/cloud-ark/kubediscovery/pkg/cmd/server"
	"github.com/cloud-ark/kubediscovery/pkg/discovery"
	"github.com/cloud-ark/kubediscovery/pkg/apiserver"
	"runtime/pprof"
	"flag"
	"log"
)	

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	flag.Parse()
	//fmt.Printf("CPU Profile flag:%s\n", *cpuprofile)
	if *cpuprofile != "" {
		//fmt.Printf("Profile file provided\ng")
        f, err := os.Create(*cpuprofile)
        if err != nil {
            log.Fatal(err)
        }
        pprof.StartCPUProfile(f)
        defer pprof.StopCPUProfile()
    }
	//if len(os.Args) == 7  || len(os.Args) == 5 || len(os.Args) == 3 {
	if len(os.Args) >= 2 {
		var kind, instance, namespace string
		// kubediscovery connections Moodle moodle1 default -o flat
		// kubediscovery composition Moodle moodle1 default
		// kubediscovery man Moodle
		// change to os.Args[2] when collecting profiling data
		commandType := os.Args[1]
		if _, exists := discovery.ALLOWED_COMMANDS[strings.TrimSpace(commandType)]; !exists {
			fmt.Printf("Unknown command specified:%s\n", commandType)
			fmt.Printf("Allowed values:\n")
			for k, _ := range discovery.ALLOWED_COMMANDS {
				fmt.Printf("%s ", k)
			}
		}
		if commandType == "composition" {
			if len(os.Args) < 5 {
				panic("Not enough arguments: ./kubediscovery composition <kind> <instance> <namespace>")
			}
			if len(os.Args) == 5 {
				kind = os.Args[2]
				instance = os.Args[3]
				namespace = os.Args[4]
				discovery.BuildConfig("")
			}
			if len(os.Args) == 6 {
				kind = os.Args[2]
				instance = os.Args[3]
				namespace = os.Args[4]
				kubeconfig := os.Args[5]
				kubeconfigparts := strings.Split(kubeconfig, "=")
				kubeconfigpath := kubeconfigparts[1]
				//fmt.Printf("Kubeconfig Path:%s\n", kubeconfigpath)
				discovery.BuildConfig(kubeconfigpath)
			}
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
			// change to 3, 4, 5 when collecting profiling data
			kind = os.Args[2]
			instance = os.Args[3]
			namespace = os.Args[4]
			discovery.OriginalInputNamespace = namespace
			discovery.OriginalInputKind = kind
			discovery.OriginalInputInstance = instance
			discovery.OutputFormat = "default"

			/*
			if len(os.Args) == 7 {
				discovery.OutputFormat = os.Args[6]
			}
			if len(os.Args) == 8 {
				kubeconfig := os.Args[7]
				//fmt.Printf("Kubeconfig Path:%s\n", kubeconfig)
				kubeconfigparts := strings.Split(kubeconfig, "=")
				kubeconfigpath := kubeconfigparts[1]
				//fmt.Printf("Kubeconfig Path:%s\n", kubeconfigpath)
				discovery.BuildConfig(kubeconfigpath)
				discovery.OutputFormat = os.Args[6]
			} else {
				discovery.BuildConfig("")
			}*/

			discovery.RelsToIgnore = ""
			kubeconfigpath := ""
			for _, opt := range os.Args {
				//fmt.Printf("Opt:%s\n", opt)
				parts := strings.Split(opt, "=")
				if len(parts) == 2 {
					option := parts[0]
					optVal := parts[1]
					ignorefound := strings.EqualFold(option, "--ignore")
					if ignorefound {
						discovery.RelsToIgnore = optVal
					}
					opformatfound := strings.EqualFold(option, "--output")
					if opformatfound {
					    //parts = strings.Split(opt, "=")
						discovery.OutputFormat = optVal
					}
					kubeconfigfound := strings.EqualFold(option, "--kubeconfig")
					if kubeconfigfound {
						kubeconfigpath = optVal
						/*kubeconfig := opt
						kubeconfigparts := strings.Split(kubeconfig, "=")
						kubeconfigpath = kubeconfigparts[1]
						//fmt.Printf("Kubeconfig Path:%s\n", kubeconfigpath)*/			
					}
				}
			}
			//fmt.Printf("O/P format:%s\n", discovery.OutputFormat)
			//fmt.Printf("Kubeconfig path:%s\n", kubeconfigpath)
			//fmt.Printf("IgnoreList:%s\n", discovery.RelsToIgnore)
			discovery.BuildConfig(kubeconfigpath)

			_ = discovery.ReadKinds(kind)
			exists := discovery.CheckExistence(kind, instance, namespace)
			if exists {
				// Prefetching does not seem to improve performance.
				// In fact, it degrades performance by few milliseconds.
				// So turning pre-fetching off
				//discovery.FetchGVKs(namespace)
				level := 0
				visited := make([]discovery.Connection, 0)
				relationType := ""
				discovery.OrigKind = kind
				discovery.OrigName = instance
				discovery.OrigNamespace = namespace
				discovery.OrigLevel = level
				// Build the composition tree
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
			} else {
				fmt.Printf("Resource %s of kind %s in namespace %s does not exist.\n", instance, kind, namespace)
				os.Exit(1)
			}
		}
		if commandType == "man" {

			/*if len(os.Args) < 4 {
				panic("Not enough arguments:./kubediscovery man <kind> --kubeconfig=<Full path to kubeconfig>")
			}*/

			kind := os.Args[2]
			namespace := os.Args[3]
			kubeconfig1 := os.Args[4]
			parts := strings.Split(kubeconfig1, "=")

			if len(parts) == 2 && parts[1] != "" {
				trimmedKubeconfig := strings.TrimSpace(parts[1])
				discovery.BuildConfig(trimmedKubeconfig)
			} else {
				discovery.BuildConfig("")
			}

			manPage := apiserver.GetManPage(kind, namespace)
			fmt.Printf("%s\n", manPage)
		}
		if commandType == "networkmetrics" {

                        nodeName := os.Args[2]
                        kubeconfig1 := os.Args[3]
                        parts := strings.Split(kubeconfig1, "=")
                        trimmedKubeconfig := strings.TrimSpace(parts[1])
                        discovery.BuildConfig(trimmedKubeconfig)

                        cAdvisorMetrics := discovery.GetCAdvisorMetrics(nodeName)
                        fmt.Printf(cAdvisorMetrics)
		}
		if commandType == "podmetrics" {

                        nodeName := os.Args[2]
                        kubeconfig1 := os.Args[3]
                        parts := strings.Split(kubeconfig1, "=")
                        trimmedKubeconfig := strings.TrimSpace(parts[1])
                        discovery.BuildConfig(trimmedKubeconfig)

                        podMetrics := discovery.GetKubeletMetrics(nodeName)
                        //fmt.Printf("-----\n")
                        fmt.Printf(podMetrics)

		}
	} else {
		fmt.Printf("Running from within cluster.\n")
		fmt.Printf("Installing KubePlus paths.\n")
		discovery.BuildConfig("")
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

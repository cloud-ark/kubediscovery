package apiserver

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"github.com/emicklei/go-restful"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/cloud-ark/kubediscovery/pkg/discovery"
)

const GroupName = "kubeplus.cloudark.io"
const GroupVersion = "v1"

var (
	Scheme = runtime.NewScheme()
	Codecs = serializer.NewCodecFactory(Scheme)
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
    SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion)
	return nil
}

func init() {
	utilruntime.Must(AddToScheme(Scheme))

	// Setting VersionPriority is critical in the InstallAPIGroup call (done in New())
	utilruntime.Must(Scheme.SetVersionPriority(SchemeGroupVersion))

	// TODO(devdattakulkarni) -- Following comments coming from sample-apiserver.
	// Leaving them for now. 
	// we need to add the options to empty v1
	// TODO fix the server code to avoid this
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: GroupVersion})

	// TODO(devdattakulkarni) -- Following comments coming from sample-apiserver.
	// Leaving them for now.
	// TODO: keep the generic API server from wanting this
	unversioned := schema.GroupVersion{Group: "", Version: GroupVersion}
	Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)

	// Start collecting provenance
	go discovery.BuildCompositionTree()
}

type ExtraConfig struct {
	// Place you custom config here.
}

type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

// DiscoveryServer contains state for a Kubernetes cluster master/api server.
type DiscoveryServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

type CompletedConfig struct {
	// Embed a private pointer that cannot be instantiated outside of this package.
	*completedConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (cfg *Config) Complete() CompletedConfig {
	c := completedConfig{
		cfg.GenericConfig.Complete(),
		&cfg.ExtraConfig,
	}

	c.GenericConfig.Version = &version.Info{
		Major: "1",
		Minor: "0",
	}

	return CompletedConfig{&c}
}

// New returns a new instance of DiscoveryServer from the given config.
func (c completedConfig) New() (*DiscoveryServer, error) {
	genericServer, err := c.GenericConfig.New("kube discovery server", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	s := &DiscoveryServer{
		GenericAPIServer: genericServer,
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(GroupName, Scheme, metav1.ParameterCodec, Codecs)

	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	installCompositionWebService(s)

	installExplainDescribePaths(s)

	//installExplainAlternate(s)

	return s, nil
}

func installExplainAlternate(discoveryServer *DiscoveryServer) {

	path := "/apis/" + GroupName + "/" + GroupVersion
	
	explainPath := path + "/explain"
	fmt.Printf("Explain PATH:%s\n", explainPath)
	//ws1 := getWebService()
	ws1 := new(restful.WebService)
	ws1.Path(explainPath).
		Consumes(restful.MIME_JSON, restful.MIME_XML).
		Produces(restful.MIME_JSON, restful.MIME_XML)
	ws1.Route(ws1.GET(explainPath).To(handleExplain))
	discoveryServer.GenericAPIServer.Handler.GoRestfulContainer.Add(ws1)

}

func installExplainDescribePaths(discoveryServer *DiscoveryServer) {

	//path := "/apis/" + GroupName + "/" + GroupVersion + "/namespaces/" + discovery.Namespace

	path := "/apis/" + GroupName + "/" + GroupVersion
	
	explainPath := path + "/explain"
	fmt.Printf("Explain PATH:%s\n", explainPath)
	ws1 := getWebService()
	//ws1 := new(restful.WebService)
	ws1.Path(path).
		Consumes(restful.MIME_JSON, restful.MIME_XML).
		Produces(restful.MIME_JSON, restful.MIME_XML)

	ws1.Route(ws1.GET("/explain").To(handleExplain))
	//discoveryServer.GenericAPIServer.Handler.GoRestfulContainer.Add(ws1)

	//describePath := path + "/describe"
	//fmt.Printf("Describe PATH:%s\n", describePath)
	//ws2 := getWebService()
	//ws2 := new(restful.WebService)
	//ws2.Path(path).
	//	Consumes(restful.MIME_JSON, restful.MIME_XML).
	//	Produces(restful.MIME_JSON, restful.MIME_XML)
	ws1.Route(ws1.GET("/describe").To(handleDescribe))
	discoveryServer.GenericAPIServer.Handler.GoRestfulContainer.Add(ws1)
}

func handleExplain(request *restful.Request, response *restful.Response) {
	fmt.Println("Entering handleExplain")

	customResourceKind := request.QueryParameter("cr")
	openAPISpec := discovery.GetOpenAPISpec(customResourceKind)

	fmt.Println("OpenAPI Spec:%v", openAPISpec)

	response.Write([]byte(openAPISpec))

	fmt.Println("Exiting handleExplain")
}

func handleDescribe(request *restful.Request, response *restful.Response) {
	fmt.Println("Entering handleDescribe")

	customResourceKind := request.QueryParameter("cr")
	customResourceInstance := request.QueryParameter("instance")

	describeInfo := discovery.TotalClusterProvenance.GetProvenance(customResourceKind, customResourceInstance)
	fmt.Println("Provenance Info:%v", describeInfo)

	response.Write([]byte(describeInfo))

	fmt.Println("Exiting handleDescribe")
}

func installCompositionWebService(discoveryServer *DiscoveryServer) {
	for _, resourceKindPlural := range discovery.KindPluralMap {
		namespaceToUse := discovery.Namespace
		path := "/apis/" + GroupName + "/" + GroupVersion + "/namespaces/"
		path = path + namespaceToUse + "/" + strings.ToLower(resourceKindPlural)
		fmt.Println("WS PATH:" + path)
		ws := getWebService()
		ws.Path(path).
			Consumes(restful.MIME_JSON, restful.MIME_XML).
			Produces(restful.MIME_JSON, restful.MIME_XML)
		getPath := "/{resource-id}/compositions"
		ws.Route(ws.GET(getPath).To(getCompositions))
		discoveryServer.GenericAPIServer.Handler.GoRestfulContainer.Add(ws)
	}
}

func getWebService() *restful.WebService {
	ws := new(restful.WebService)
	ws.Path("/apis")
	ws.Consumes("*/*")
	ws.Produces(restful.MIME_JSON, restful.MIME_XML)
	ws.ApiVersion(GroupName)
	return ws
}

func getCompositions(request *restful.Request, response *restful.Response) {
	fmt.Println("Inside getCompositions")
	resourceName := request.PathParameter("resource-id")
	requestPath := request.Request.URL.Path
	fmt.Printf("Printing Composition\n")
	fmt.Printf("Resource Name:%s\n", resourceName)
	fmt.Printf("Request Path:%s\n", requestPath)
	//provenance.TotalClusterProvenance.PrintProvenance()

	// Path looks as follows:
	// /apis/kubediscovery.cloudark.io/v1/namespaces/default/deployments/dep1/compositions
	resourcePathSlice := strings.Split(requestPath, "/")
	resourceKind := resourcePathSlice[6] // Kind is 7th element in the slice
	fmt.Printf("Resource Kind:%s, Resource name:%s\n", resourceKind, resourceName)
	provenanceInfo := discovery.TotalClusterProvenance.GetProvenance(resourceKind, resourceName)
	fmt.Println("Provenance Info:%v", provenanceInfo)

	response.Write([]byte(provenanceInfo))
}

package compatibility

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

// These adapters model the compile-time boundary for a generated module index.
// They deliberately contain no business code or generator runtime behavior.
type httpModuleRegistration struct {
	name       string
	operations map[string]string
	register   func(*gin.RouterGroup)
}

type rpcModuleRegistration struct {
	permissions map[string]string
	register    func(grpc.ServiceRegistrar)
}

type gatewayModuleRegistration func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error

type sampleRPCService struct{}

func (sampleRPCService) Register(registrar grpc.ServiceRegistrar) {
	registrar.RegisterService(&grpc.ServiceDesc{ServiceName: "example.v1.ExampleService"}, struct{}{})
}

type sampleServiceRegistrar struct{ count int }

func (registrar *sampleServiceRegistrar) RegisterService(*grpc.ServiceDesc, any) {
	registrar.count++
}

func TestGeneratedModuleRegistryContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	modules := []httpModuleRegistration{{
		name:       "example",
		operations: map[string]string{"GET": "read"},
		register: func(group *gin.RouterGroup) {
			group.GET("", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		},
	}}
	router := gin.New()
	for _, module := range modules {
		if module.name == "" || module.register == nil || module.operations["GET"] == "" {
			t.Fatal("incomplete HTTP module registration")
		}
		module.register(router.Group("/v1/" + module.name))
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/example", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("HTTP module route status = %d", response.Code)
	}

	service := sampleRPCService{}
	rpcModule := rpcModuleRegistration{
		permissions: map[string]string{"/example.v1.ExampleService/Get": "read"},
		register:    service.Register,
	}
	registrar := new(sampleServiceRegistrar)
	rpcModule.register(registrar)
	if registrar.count != 1 || rpcModule.permissions["/example.v1.ExampleService/Get"] != "read" {
		t.Fatal("RPC registration or permission contract did not compose")
	}

	var gatewayRegister gatewayModuleRegistration = func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error {
		return nil
	}
	if gatewayRegister == nil {
		t.Fatal("Gateway registration callback is nil")
	}

	documents := []string{"api/openapi/zebra.yaml", "api/openapi/example.yaml"}
	sort.Strings(documents)
	if documents[0] != "api/openapi/example.yaml" || documents[1] != "api/openapi/zebra.yaml" {
		t.Fatal("module documentation inputs are not deterministic")
	}
}

package templates

import (
	"fmt"
	"strings"
)

// The module factory is generated once and then owned by the developer. Its
// stable dependency argument allows constructors to evolve independently of
// the regenerated registry.
func renderModuleFactory(project Project, options Options, name string) (File, error) {
	var source strings.Builder
	source.WriteString("// Module assembly is developer-owned; goseed does not overwrite this file.\npackage bootstrap\n\nimport (\n")
	fmt.Fprintf(&source, "business %q\n", project.Module+"/internal/apiserver/biz/"+name)
	if options.Database != "none" {
		fmt.Fprintf(&source, "store %q\n", project.Module+"/internal/apiserver/store/"+name)
	}
	http := options.Transport == "http" || options.Transport == "dual"
	rpc := options.Transport != "http"
	if http {
		fmt.Fprintf(&source, "httpHandler %q\n", project.Module+"/internal/apiserver/handler/http/"+name)
	}
	if rpc {
		fmt.Fprintf(&source, "rpcHandler %q\n", project.Module+"/internal/apiserver/handler/grpc/"+name)
	}
	if options.Transport == "gateway" {
		fmt.Fprintf(&source, "\"context\"\n gateway %q\n\"github.com/grpc-ecosystem/grpc-gateway/v2/runtime\"\n\"google.golang.org/grpc\"\n", project.Module+"/internal/pkg/gateway")
	}
	source.WriteString(")\n\ntype " + name + "Module struct {\n")
	if http {
		source.WriteString("HTTP *httpHandler.Handler\nOperations map[string]string\n")
	}
	if rpc {
		source.WriteString("RPC *rpcHandler.Handler\nPermissions map[string]string\n")
	}
	if options.Transport == "gateway" {
		source.WriteString("Gateway gateway.Registration\n")
	}
	goName := strings.ToUpper(name[:1]) + name[1:]
	fmt.Fprintf(&source, "}\n\nfunc new%sModule(dependencies moduleDependencies) (%sModule, error) {\n", goName, name)
	if options.Database == "none" {
		source.WriteString("biz := business.New()\n")
	} else {
		source.WriteString("biz := business.New(store.New(dependencies.Database))\n")
	}
	source.WriteString("return " + name + "Module{\n")
	if http {
		source.WriteString("HTTP: httpHandler.New(biz),\nOperations: map[string]string{},\n")
	}
	if rpc {
		source.WriteString("RPC: rpcHandler.New(biz),\nPermissions: rpcHandler.Permissions,\n")
	}
	if options.Transport == "gateway" {
		source.WriteString("Gateway: gateway.Registration{Queries:map[string][]string{},BodyMethods:map[string]bool{},Register:func(context.Context,*runtime.ServeMux,*grpc.ClientConn)error{return nil}},\n")
	}
	source.WriteString("}, nil\n}\n")
	formatted, err := formatGo(source.String())
	if err != nil {
		return File{}, err
	}
	return File{Path: "internal/apiserver/bootstrap/module_" + name + ".go", Content: formatted}, nil
}

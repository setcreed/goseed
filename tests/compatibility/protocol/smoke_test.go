package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type echoServer struct{ UnimplementedEchoServiceServer }

func (echoServer) Echo(_ context.Context, request *EchoRequest) (*EchoResponse, error) {
	return &EchoResponse{Message: request.Message}, nil
}

func TestGeneratedProtocol(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	RegisterEchoServiceServer(server, echoServer{})
	serveError := make(chan error, 1)
	go func() { serveError <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		if err := <-serveError; err != nil {
			t.Errorf("gRPC serve: %v", err)
		}
	})
	connection, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	response, err := NewEchoServiceClient(connection).Echo(ctx, &EchoRequest{Message: "grpc"})
	if err != nil || response.GetMessage() != "grpc" {
		t.Fatalf("gRPC: %v %v", response, err)
	}
	mux := runtime.NewServeMux()
	if err := RegisterEchoServiceHandler(ctx, mux, connection); err != nil {
		t.Fatal(err)
	}
	writer := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/goseed.compatibility.EchoService/Echo", bytes.NewBufferString(`{"message":"gateway"}`)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(writer, request)
	var body struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(writer.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if writer.Code != http.StatusOK || body.Message != "gateway" {
		t.Fatalf("Gateway: %d %s", writer.Code, writer.Body.String())
	}
	data, err := os.ReadFile("smoke.swagger.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Swagger string                     `json:"swagger"`
		Paths   map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Swagger != "2.0" || len(spec.Paths) != 1 {
		t.Fatalf("OpenAPI output: %s paths=%d", spec.Swagger, len(spec.Paths))
	}
}

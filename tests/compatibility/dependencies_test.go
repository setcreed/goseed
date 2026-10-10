package compatibility

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/redis/go-redis/v9"
	"github.com/samber/do/v2"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// This module proves version interoperability; it is not a business template.
func TestRuntimeDependencies(t *testing.T) {
	config := viper.New()
	config.SetConfigType("yaml")
	if err := config.ReadConfig(bytes.NewBufferString("server:\n  port: 8080\n")); err != nil {
		t.Fatal(err)
	}
	if config.GetInt("server.port") != 8080 {
		t.Fatal("configuration was not decoded")
	}
	logger := zap.NewNop()
	if err := logger.Sync(); err != nil {
		t.Fatal(err)
	}
	injector := do.New()
	do.ProvideValue(injector, logger)
	if got, err := do.Invoke[*zap.Logger](injector); err != nil || got != logger {
		t.Fatalf("constructor composition: %v", err)
	}
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusNoContent {
		t.Fatal(response.Code)
	}
	key := []byte("compatibility-test-key-not-a-runtime-secret")
	claims := jwt.RegisteredClaims{Subject: "alice", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	token, err := jwt.Parse(raw, func(*jwt.Token) (any, error) { return key, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		t.Fatalf("JWT round-trip: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: time.Millisecond, ReadTimeout: time.Millisecond, WriteTimeout: time.Millisecond, MaxRetries: -1})
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	connection, err := grpc.NewClient("passthrough:///127.0.0.1:1", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	mux := runtime.NewServeMux()
	if err := mux.HandlePath(http.MethodGet, "/healthz", func(w http.ResponseWriter, _ *http.Request, _ map[string]string) { w.WriteHeader(http.StatusNoContent) }); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil).WithContext(context.Background()))
	if response.Code != http.StatusNoContent {
		t.Fatal(response.Code)
	}
	if _, err := proto.Marshal(&emptypb.Empty{}); err != nil {
		t.Fatal(err)
	}
}

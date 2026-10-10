package integration

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func rejectCommand(t *testing.T, directory, binary string, overrides []string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = directory
	command.Env = append(environment(t), overrides...)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("invalid startup succeeded")
	}
	if ctx.Err() != nil {
		t.Fatal("invalid startup did not fail promptly")
	}
	if strings.Contains(string(output), "test-secret-private") {
		t.Fatal("startup secret leaked")
	}
}
func verifyStartupFailures(t *testing.T, directory, binary string) {
	t.Helper()
	rejectCommand(t, directory, binary, nil, "serve", "--config", "missing-config.yaml")
	rejectCommand(t, directory, binary, nil, "serve", "--config", "configs/app.example.yaml")
	unknown := filepath.Join(directory, "configs/unknown.yaml")
	if err := os.WriteFile(unknown, []byte("unknown: test-secret-private\n"), 0600); err != nil {
		t.Fatal(err)
	}
	rejectCommand(t, directory, binary, nil, "serve", "--config", unknown)
	rejectCommand(t, directory, binary, []string{"APP_DATABASE_PATH=" + filepath.Join(directory, "data/unmigrated.db")}, "serve")
	rejectCommand(t, directory, binary, []string{"APP_HTTP_TLS_CERT=absent.pem", "APP_HTTP_TLS_KEY=absent.key"}, "serve")
	rejectCommand(t, directory, binary, []string{"APP_JWT_SECRET=test-secret-private"}, "serve")
	production := []string{"APP_ENV=production", "APP_LOG_FORMAT=json"}
	rejectCommand(t, directory, binary, production, "dev", "init", "--config", filepath.Join(directory, "configs/production-init.yaml"))
	if _, err := os.Stat(filepath.Join(directory, "configs/production-init.yaml")); !os.IsNotExist(err) {
		t.Fatal("production init wrote configuration")
	}
	rejectCommand(t, directory, binary, production, "dev", "token", "--subject", "alice")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	rejectCommand(t, directory, binary, nil, "serve", "--address", listener.Addr().String())
}
func verifyTLS(t *testing.T, directory, binary string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "local-test"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, DNSNames: []string{"localhost"}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(directory, "test-cert.pem")
	keyPath := filepath.Join(directory, "test-key.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certPEM) {
		t.Fatal("test certificate trust")
	}
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}}
	defer client.CloseIdleConnections()
	process := startServiceWith(t, directory, binary, client, "https", []string{"APP_HTTP_TLS_CERT=" + certPath, "APP_HTTP_TLS_KEY=" + keyPath, "APP_HTTP_ALLOWED_ORIGINS=https://client.example"})
	untrusted := &http.Client{Timeout: time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: x509.NewCertPool(), MinVersion: tls.VersionTLS12}}}
	defer untrusted.CloseIdleConnections()
	if response, err := untrusted.Get(process.url + "/readyz"); err == nil {
		_ = response.Body.Close()
		t.Fatal("untrusted certificate accepted")
	}
	response, err := client.Get(process.url + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.TLS == nil || len(response.TLS.VerifiedChains) == 0 || response.TLS.Version < tls.VersionTLS12 {
		t.Fatal("TLS was not verified")
	}
	for _, test := range []struct {
		origin string
		status int
	}{{"https://client.example", 204}, {"https://forbidden.example", 403}} {
		req, err := http.NewRequest("OPTIONS", process.url+"/v1/examples", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", test.origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != test.status {
			t.Fatalf("CORS status %d want %d", response.StatusCode, test.status)
		}
		if test.status == 204 && response.Header.Get("Access-Control-Allow-Origin") != test.origin {
			t.Fatal("CORS exact origin missing")
		}
	}
	process.stop(t)
	logs, err := os.ReadFile(process.log)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(logs)), "\n") {
		if !json.Valid([]byte(line)) {
			t.Fatal("HTTP protocol errors bypassed JSON logging")
		}
	}
}

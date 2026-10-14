package integration

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/setcreed/goseed/internal/templates"
)

func TestSingleHostDeploymentProjectsCompileReadonly(t *testing.T) {
	for _, transport := range []string{"http", "grpc", "dual", "gateway"} {
		t.Run(transport, func(t *testing.T) {
			directory := renderDeploymentProject(t, templates.Options{Database: "none", Transport: transport, Deployment: "single-host"})
			verifyReadonly(t, directory)
			run(t, directory, "go", "test", "-mod=readonly", "-run", "^$", "./...")
		})
	}
	t.Run("postgres-dual-redis-casbin", func(t *testing.T) {
		directory := renderDeploymentProject(t, templates.Options{Database: "postgres", Redis: true, Authz: "casbin", Transport: "dual", Deployment: "single-host"})
		verifyReadonly(t, directory)
		run(t, directory, "go", "test", "-mod=readonly", "-run", "^$", "./...")
	})
}

func TestSingleHostSQLiteBackupRestore(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 is required for the generated SQLite backup drill")
	}
	directory := renderDeploymentProject(t, templates.Options{Database: "sqlite", Transport: "http", Deployment: "single-host"})
	source := filepath.Join(directory, "deploy/backup.sh")
	restore := filepath.Join(directory, "deploy/restore.sh")
	backupRoot := filepath.Join(directory, "backup-dir")
	databaseRoot := filepath.Join(directory, "database-dir")
	if err := os.MkdirAll(backupRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(databaseRoot, 0700); err != nil {
		t.Fatal(err)
	}
	database := filepath.Join(databaseRoot, "app.db")
	command := exec.Command("sqlite3", database, "PRAGMA journal_mode=WAL; CREATE TABLE entries(value TEXT NOT NULL); INSERT INTO entries VALUES ('before-backup');")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create WAL database: %v\n%s", err, output)
	}
	for _, replacement := range []struct{ old, new string }{
		{"/var/lib/demo/app.db", database},
		{"/var/backups/demo", backupRoot},
	} {
		for _, name := range []string{source, restore} {
			data, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			data = []byte(strings.ReplaceAll(string(data), replacement.old, replacement.new))
			if err := os.WriteFile(name, data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	output, err := exec.Command("sh", source).CombinedOutput()
	if err != nil {
		t.Fatalf("generated backup failed: %v\n%s", err, output)
	}
	backup := strings.TrimSpace(string(output))
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("generated snapshot is missing: %v", err)
	}
	command = exec.Command("sqlite3", database, "INSERT INTO entries VALUES ('after-backup');")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("mutate WAL database after snapshot: %v\n%s", err, output)
	}
	restored := filepath.Join(databaseRoot, "restored.db")
	output, err = exec.Command("sh", restore, backup, restored).CombinedOutput()
	if err != nil {
		t.Fatalf("generated restore failed: %v\n%s", err, output)
	}
	command = exec.Command("sqlite3", restored, "SELECT group_concat(value, ',') FROM entries;")
	output, err = command.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "before-backup" {
		t.Fatalf("restored snapshot contents = %q, error = %v", strings.TrimSpace(string(output)), err)
	}
	if _, err := exec.Command("sh", restore, backup, restored).CombinedOutput(); err == nil {
		t.Fatal("restore overwrote an existing destination")
	}
	damaged := filepath.Join(backupRoot, "damaged.sqlite")
	if err := os.WriteFile(damaged, []byte("not a SQLite database"), 0600); err != nil {
		t.Fatal(err)
	}
	damagedTarget := filepath.Join(databaseRoot, "damaged-restore.db")
	if output, err := exec.Command("sh", restore, damaged, damagedTarget).CombinedOutput(); err == nil {
		t.Fatalf("restore accepted damaged backup: %s", output)
	}
	if _, err := os.Stat(damagedTarget); !os.IsNotExist(err) {
		t.Fatalf("failed restore left a target file: %v", err)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("restore altered or removed source backup: %v", err)
	}
	if err := os.Remove(database); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("sh", source).CombinedOutput(); err == nil {
		t.Fatalf("backup accepted a missing database: %s", output)
	}
	if _, err := os.Stat(database); !os.IsNotExist(err) {
		t.Fatalf("backup created an empty source database: %v", err)
	}
}

func TestSingleHostExternalSQLBackupRestore(t *testing.T) {
	password := os.Getenv("GOSEED_TEST_PASSWORD")
	mysqlContainer := os.Getenv("GOSEED_TEST_MYSQL_CONTAINER")
	postgresContainer := os.Getenv("GOSEED_TEST_POSTGRES_CONTAINER")
	if password == "" || mysqlContainer == "" || postgresContainer == "" {
		t.Skip("run the isolated TLS database target for MySQL/PostgreSQL backup and restore")
	}
	var random [24]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	backupPassword := hex.EncodeToString(random[:])
	docker := func(args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, "docker", args...)
		command.Env = environment(t)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("docker %v: %v\n%s", args, err, output)
		}
		return output
	}
	mysql := func(args ...string) []byte {
		t.Helper()
		return docker(append([]string{"exec", "-e", "MYSQL_PWD=" + password, mysqlContainer}, args...)...)
	}
	postgres := func(args ...string) []byte {
		t.Helper()
		return docker(append([]string{"exec", postgresContainer}, args...)...)
	}
	directory := renderDeploymentProject(t, templates.Options{Database: "mysql", Deployment: "single-host"})
	mysqlCreate := "CREATE DATABASE demo; CREATE USER 'demo_backup'@'127.0.0.1' IDENTIFIED BY '" + backupPassword + "' REQUIRE SSL; GRANT ALL PRIVILEGES ON demo.* TO 'demo_backup'@'127.0.0.1'; GRANT SHOW_ROUTINE ON *.* TO 'demo_backup'@'127.0.0.1'; CREATE TABLE demo.entries(value VARCHAR(100) NOT NULL); INSERT INTO demo.entries VALUES ('mysql-backup-row');"
	mysql("mysql", "-uroot", "-e", mysqlCreate)
	mysqlConfig := "[client]\nhost=127.0.0.1\nport=3306\nuser=demo_backup\npassword=" + backupPassword + "\nssl-mode=VERIFY_IDENTITY\nssl-ca=/testtls/ca.crt\n"
	mysqlConfigPath := filepath.Join(directory, "backup-mysql.cnf")
	if err := os.WriteFile(mysqlConfigPath, []byte(mysqlConfig), 0600); err != nil {
		t.Fatal(err)
	}
	docker("exec", mysqlContainer, "mkdir", "-p", "/etc/demo", "/var/backups/demo")
	docker("cp", filepath.Join(directory, "deploy/backup.sh"), mysqlContainer+":/tmp/backup.sh")
	docker("cp", mysqlConfigPath, mysqlContainer+":/etc/demo/backup-mysql.cnf")
	mysqlDump := strings.TrimSpace(string(docker("exec", mysqlContainer, "sh", "/tmp/backup.sh")))
	mysql("mysql", "-uroot", "-e", "DROP DATABASE demo; CREATE DATABASE demo;")
	localMySQLDump := filepath.Join(directory, "backup.sql")
	docker("cp", mysqlContainer+":"+mysqlDump, localMySQLDump)
	mysqlRestore := exec.Command("docker", "exec", "-i", "-e", "MYSQL_PWD="+password, mysqlContainer, "mysql", "-uroot", "demo")
	mysqlRestore.Env = environment(t)
	dumpFile, err := os.Open(localMySQLDump)
	if err != nil {
		t.Fatal(err)
	}
	mysqlRestore.Stdin = dumpFile
	if output, err := mysqlRestore.CombinedOutput(); err != nil {
		_ = dumpFile.Close()
		t.Fatalf("restore generated MySQL dump: %v\n%s", err, output)
	}
	if err := dumpFile.Close(); err != nil {
		t.Fatal(err)
	}
	if result := strings.TrimSpace(string(mysql("mysql", "-uroot", "-Nse", "SELECT value FROM demo.entries"))); result != "mysql-backup-row" {
		t.Fatalf("restored MySQL row = %q", result)
	}

	postgresDirectory := renderDeploymentProject(t, templates.Options{Database: "postgres", Deployment: "single-host"})
	pgPass := "127.0.0.1:5432:demo:goseed_test:" + password + "\n"
	pgPassPath := filepath.Join(postgresDirectory, "pgpass")
	if err := os.WriteFile(pgPassPath, []byte(pgPass), 0600); err != nil {
		t.Fatal(err)
	}
	postgres("createdb", "-U", "goseed_test", "demo")
	postgres("psql", "-U", "goseed_test", "-d", "demo", "-c", "CREATE TABLE entries(value text NOT NULL); INSERT INTO entries VALUES ('postgres-backup-row');")
	docker("exec", postgresContainer, "mkdir", "-p", "/etc/demo", "/var/backups/demo")
	docker("cp", filepath.Join(postgresDirectory, "deploy/backup.sh"), postgresContainer+":/tmp/backup.sh")
	docker("cp", pgPassPath, postgresContainer+":/etc/demo/pgpass")
	postgresDump := strings.TrimSpace(string(docker("exec", "-e", "PGHOST=127.0.0.1", "-e", "PGPORT=5432", "-e", "PGUSER=goseed_test", "-e", "PGPASSFILE=/etc/demo/pgpass", "-e", "PGSSLROOTCERT=/testtls/ca.crt", postgresContainer, "sh", "/tmp/backup.sh")))
	postgres("dropdb", "-U", "goseed_test", "demo")
	postgres("createdb", "-U", "goseed_test", "demo")
	postgres("pg_restore", "-U", "goseed_test", "--exit-on-error", "--no-owner", "--dbname=demo", postgresDump)
	if result := strings.TrimSpace(string(postgres("psql", "-U", "goseed_test", "-d", "demo", "-Atc", "SELECT value FROM entries"))); result != "postgres-backup-row" {
		t.Fatalf("restored PostgreSQL row = %q", result)
	}
}

func TestSingleHostReleaseSwitchRecovery(t *testing.T) {
	directory := renderDeploymentProject(t, templates.Options{Database: "none", Deployment: "single-host"})
	releases := filepath.Join(directory, "releases")
	if err := os.MkdirAll(releases, 0700); err != nil {
		t.Fatal(err)
	}
	for version, value := range map[string]string{"v1": "healthy", "v2": "unhealthy"} {
		release := filepath.Join(releases, version)
		if err := os.Mkdir(release, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(release, "health"), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	current := filepath.Join(directory, "current")
	switchTo := func(version, suffix string) {
		t.Helper()
		candidate := filepath.Join(directory, "current."+suffix)
		if err := os.Symlink(filepath.Join(releases, version), candidate); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(candidate, current); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "schema-version"), []byte("2"), 0600); err != nil {
		t.Fatal(err)
	}
	switchTo("v1", "next")
	switchTo("v2", "next")
	active, err := os.Readlink(current)
	if err != nil {
		t.Fatal(err)
	}
	if value, err := os.ReadFile(filepath.Join(active, "health")); err != nil || string(value) != "unhealthy" {
		t.Fatalf("new release health = %q, error = %v", value, err)
	}
	switchTo("v1", "rollback")
	active, err = os.Readlink(current)
	if err != nil {
		t.Fatal(err)
	}
	if value, err := os.ReadFile(filepath.Join(active, "health")); err != nil || string(value) != "healthy" {
		t.Fatalf("recovered release health = %q, error = %v", value, err)
	}
	if schema, err := os.ReadFile(filepath.Join(directory, "schema-version")); err != nil || string(schema) != "2" {
		t.Fatalf("binary rollback changed schema state: %q, error = %v", schema, err)
	}
	readme, err := os.ReadFile(filepath.Join(directory, "deploy/README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "mv -Tf") || !strings.Contains(string(readme), "应用二进制回退不会回滚 schema") {
		t.Fatal("generated recovery guide omits atomic release switch or schema rollback boundary")
	}
}

func TestSingleHostNginxContainer(t *testing.T) {
	if os.Getenv("GOSEED_TEST_DOCKER") != "1" {
		t.Skip("run make template-deployment-nginx for pinned Nginx container verification")
	}
	directory := renderDeploymentProject(t, templates.Options{Database: "none", Transport: "dual", Deployment: "single-host"})
	generateDeploymentCertificates(t, directory)
	tlsDirectory := filepath.Join(directory, "deploy/tls")
	certificate, err := os.ReadFile(filepath.Join(tlsDirectory, "public.crt"))
	if err != nil {
		t.Fatal(err)
	}
	publicCA, err := os.ReadFile(filepath.Join(tlsDirectory, "public-ca.crt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tlsDirectory, "public-fullchain.pem"), append(append(certificate, '\n'), publicCA...), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tlsDirectory, "public-key.pem"), mustReadFile(t, filepath.Join(tlsDirectory, "public.key")), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tlsDirectory, "grpc-ca.pem"), mustReadFile(t, filepath.Join(tlsDirectory, "grpc-ca.crt")), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var lock struct {
		Image string `json:"single_host_test_proxy_image"`
	}
	lockFile, err := os.ReadFile(filepath.Join(root, "dependencies.lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(lockFile, &lock); err != nil || !strings.Contains(lock.Image, "@sha256:") {
		t.Fatalf("single-host Nginx test image is not digest-pinned: %q, error=%v", lock.Image, err)
	}
	check := func(success bool) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		arguments := []string{"run", "--rm", "--entrypoint", "nginx", "-v", filepath.Join(directory, "deploy/nginx") + ":/etc/nginx/conf.d:ro", "-v", tlsDirectory + ":/etc/demo/tls:ro", lock.Image, "-t"}
		command := exec.CommandContext(ctx, "docker", arguments...)
		command.Env = environment(t)
		output, err := command.CombinedOutput()
		if (err == nil) != success {
			t.Fatalf("docker %v: unexpected exit %v\n%s", arguments, err, output)
		}
		return output
	}
	if output := check(true); !strings.Contains(string(output), "test is successful") {
		t.Fatalf("Nginx did not validate generated HTTP/gRPC configurations: %s", output)
	}
	badCertificate := "server { listen 8444 ssl; ssl_certificate /missing/cert.pem; ssl_certificate_key /missing/key.pem; }"
	if err := os.WriteFile(filepath.Join(directory, "deploy/nginx/bad-cert.conf"), []byte(badCertificate), 0600); err != nil {
		t.Fatal(err)
	}
	if output := check(false); !strings.Contains(string(output), "/missing/cert.pem") {
		t.Fatalf("invalid certificate path failed for unexpected reason: %s", output)
	}
}

func mustReadFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSingleHostDeploymentContainer(t *testing.T) {
	if os.Getenv("GOSEED_TEST_DOCKER") != "1" {
		t.Skip("run make template-deployment for isolated Linux/systemd/Nginx verification")
	}
	directory := renderDeploymentProject(t, templates.Options{Database: "none", Transport: "dual", Deployment: "single-host"})
	verifyReadonly(t, directory)
	binary := filepath.Join(directory, "bin/apiserver")
	client := filepath.Join(directory, "bin/exampleclient")
	if err := os.MkdirAll(filepath.Dir(binary), 0750); err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct{ output, packagePath string }{{binary, "./cmd/apiserver"}, {client, "./cmd/exampleclient"}} {
		runWith(t, directory, []string{"GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0"}, "go", "build", "-mod=readonly", "-o", target.output, target.packagePath)
	}
	var secretBytes [32]byte
	if _, err := rand.Read(secretBytes[:]); err != nil {
		t.Fatal(err)
	}
	secret := base64.StdEncoding.EncodeToString(secretBytes[:])
	if err := os.WriteFile(filepath.Join(directory, "deploy/app.env"), []byte("APP_ENV=production\nAPP_HTTP_ADDRESS=127.0.0.1:8080\nAPP_GRPC_ADDRESS=127.0.0.1:9090\nAPP_GRPC_TLS_CERT=/etc/demo/tls/grpc.crt\nAPP_GRPC_TLS_KEY=/etc/demo/tls/grpc.key\nAPP_JWT_SECRET="+secret+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	generateDeploymentCertificates(t, directory)
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	name := "goseed-d11-systemd-" + hex.EncodeToString(random[:])
	image := "goseed-d11-systemd-test:local"
	docker := func(success bool, args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, "docker", args...)
		command.Env = environment(t)
		output, err := command.CombinedOutput()
		if (err == nil) != success {
			t.Fatalf("docker %v: unexpected exit %v\n%s", args, err, output)
		}
		return output
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, "docker", "rm", "-f", name)
		command.Env = environment(t)
		_ = command.Run()
	})
	docker(true, "run", "-d", "--privileged", "--cgroupns=host", "--tmpfs", "/run", "--tmpfs", "/run/lock", "-v", "/sys/fs/cgroup:/sys/fs/cgroup:rw", "--name", name, "--label", "org.goseed.d11-test="+name, image)
	waitSystemd(t, docker, name)
	docker(true, "exec", name, "systemctl", "stop", "nginx.service")
	docker(true, "exec", name, "sh", "-c", "useradd --system --user-group --no-create-home --shell /usr/sbin/nologin demo && mkdir -p /opt/demo/releases/v1/bin /etc/demo/tls /etc/nginx/conf.d && ln -s /opt/demo/releases/v1 /opt/demo/current")
	for _, file := range []struct{ source, target string }{
		{binary, name + ":/opt/demo/releases/v1/bin/apiserver"},
		{client, name + ":/opt/demo/releases/v1/bin/exampleclient"},
		{filepath.Join(directory, "configs/app.example.yaml"), name + ":/etc/demo/app.yaml"},
		{filepath.Join(directory, "deploy/app.env"), name + ":/etc/demo/app.env"},
		{filepath.Join(directory, "deploy/systemd/apiserver.service"), name + ":/etc/systemd/system/demo.service"},
		{filepath.Join(directory, "deploy/nginx/http.conf"), name + ":/etc/nginx/conf.d/demo-http.conf"},
		{filepath.Join(directory, "deploy/nginx/grpc.conf"), name + ":/etc/nginx/conf.d/demo-grpc.conf"},
	} {
		docker(true, "cp", file.source, file.target)
	}
	for _, file := range []string{"public.crt", "public.key", "public-ca.crt", "grpc.crt", "grpc.key", "grpc-ca.crt"} {
		docker(true, "cp", filepath.Join(directory, "deploy/tls", file), name+":/etc/demo/tls/"+file)
	}
	docker(true, "exec", name, "sh", "-c", "mv /etc/demo/tls/public.crt /etc/demo/tls/public-fullchain.pem && mv /etc/demo/tls/public.key /etc/demo/tls/public-key.pem && mv /etc/demo/tls/grpc-ca.crt /etc/demo/tls/grpc-ca.pem && chown -R root:demo /etc/demo && chmod 0640 /etc/demo/app.env /etc/demo/app.yaml /etc/demo/tls/*.key && chmod 0644 /etc/demo/tls/*.crt && chmod 0755 /opt/demo/releases/v1/bin/* && systemctl daemon-reload && systemctl enable --now demo.service")
	if result := string(docker(true, "exec", name, "systemctl", "show", "demo.service", "--property=User", "--property=ProtectSystem", "--property=NoNewPrivileges", "--property=StateDirectory")); !strings.Contains(result, "User=demo") || !strings.Contains(result, "ProtectSystem=strict") || !strings.Contains(result, "NoNewPrivileges=yes") || !strings.Contains(result, "StateDirectory=") {
		t.Fatalf("systemd service hardening differs from contract: %s", result)
	}
	docker(true, "exec", name, "nginx", "-t")
	docker(true, "exec", name, "systemctl", "enable", "--now", "nginx.service")
	ready := func() string {
		t.Helper()
		return string(docker(true, "exec", name, "curl", "--fail", "--silent", "--show-error", "--resolve", "api.example.invalid:443:127.0.0.1", "--cacert", "/etc/demo/tls/public-ca.crt", "-H", "X-Forwarded-For: 203.0.113.99", "https://api.example.invalid/healthz"))
	}
	if body := ready(); !strings.Contains(body, "ok") {
		t.Fatalf("HTTP request did not traverse Nginx to the service: %s", body)
	}
	if body := string(docker(true, "exec", name, "/opt/demo/current/bin/exampleclient", "--address", "127.0.0.1:443", "--server-name", "grpc.example.invalid", "--ca", "/etc/demo/tls/public-ca.crt", "--action", "health")); !strings.Contains(body, "SERVING") {
		t.Fatalf("gRPC health request did not traverse verified Nginx TLS upstream: %s", body)
	}
	docker(true, "exec", name, "systemctl", "stop", "demo.service")
	if status := strings.TrimSpace(string(docker(false, "exec", name, "systemctl", "is-active", "demo.service"))); status != "inactive" {
		t.Fatalf("systemd service state after SIGTERM stop = %q, want inactive", status)
	}
	docker(true, "exec", name, "systemctl", "start", "demo.service")
	if body := ready(); !strings.Contains(body, "ok") {
		t.Fatalf("service failed health check after restart: %s", body)
	}
	if err := os.WriteFile(filepath.Join(directory, "deploy/nginx/bad-cert.conf"), []byte("server { listen 8444 ssl; ssl_certificate /missing/cert.pem; ssl_certificate_key /missing/key.pem; }"), 0600); err != nil {
		t.Fatal(err)
	}
	docker(true, "cp", filepath.Join(directory, "deploy/nginx/bad-cert.conf"), name+":/etc/nginx/conf.d/bad-cert.conf")
	docker(false, "exec", name, "nginx", "-t")
}

func waitSystemd(t *testing.T, docker func(bool, ...string) []byte, name string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		output := docker(true, "exec", name, "sh", "-c", "systemctl is-system-running || true")
		if strings.TrimSpace(string(output)) == "running" || strings.TrimSpace(string(output)) == "degraded" {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatal("systemd did not reach running state")
}

func generateDeploymentCertificates(t *testing.T, directory string) {
	t.Helper()
	tlsDirectory := filepath.Join(directory, "deploy/tls")
	if err := os.MkdirAll(tlsDirectory, 0750); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(tlsDirectory, "openssl.cnf")
	if err := os.WriteFile(config, []byte("[req]\ndistinguished_name=dn\n[dn]\n[ca]\nbasicConstraints=critical,CA:true\nkeyUsage=critical,keyCertSign,cRLSign\n[public]\nbasicConstraints=critical,CA:false\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:api.example.invalid,DNS:grpc.example.invalid\n[grpc]\nbasicConstraints=critical,CA:false\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:grpc.internal\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run(t, directory, "openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "2", "-subj", "/CN=d11-test-ca", "-config", config, "-extensions", "ca", "-keyout", filepath.Join(tlsDirectory, "ca.key"), "-out", filepath.Join(tlsDirectory, "ca.crt"))
	for _, certificate := range []struct{ name, extension string }{{"public", "public"}, {"grpc", "grpc"}} {
		key := filepath.Join(tlsDirectory, certificate.name+".key")
		csr := filepath.Join(tlsDirectory, certificate.name+".csr")
		cert := filepath.Join(tlsDirectory, certificate.name+".crt")
		run(t, directory, "openssl", "req", "-new", "-newkey", "rsa:2048", "-nodes", "-subj", "/CN="+certificate.name, "-config", config, "-extensions", certificate.extension, "-keyout", key, "-out", csr)
		run(t, directory, "openssl", "x509", "-req", "-days", "2", "-in", csr, "-CA", filepath.Join(tlsDirectory, "ca.crt"), "-CAkey", filepath.Join(tlsDirectory, "ca.key"), "-CAcreateserial", "-copy_extensions", "copy", "-out", cert)
	}
	if err := os.Rename(filepath.Join(tlsDirectory, "ca.crt"), filepath.Join(tlsDirectory, "public-ca.crt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(tlsDirectory, "public-ca.crt"), filepath.Join(tlsDirectory, "grpc-ca.crt")); err != nil {
		t.Fatal(err)
	}
}

func renderDeploymentProject(t *testing.T, options templates.Options) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(root, ".tmp", "d11-deployment")
	createdBase := false
	if err := os.Mkdir(base, 0750); err == nil {
		createdBase = true
	} else if !os.IsExist(err) {
		t.Fatal(err)
	}
	if createdBase {
		t.Cleanup(func() { _ = os.Remove(base) })
	}
	directory, err := os.MkdirTemp(base, options.Transport+"-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	files, err := templates.Render(templates.Project{Name: "demo", Module: "example.com/demo"}, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		filename := filepath.Join(directory, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(filename), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, file.Content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}

"""Verify rendered SQL projects against fresh, TLS-enabled, isolated databases."""

import os
from pathlib import Path
import secrets
import shutil
import subprocess
import sys
import tempfile


def main():
    root = Path(__file__).resolve().parents[1]
    base = root / ".tmp/template-db"
    base.mkdir(parents=True, exist_ok=True)
    directory = Path(tempfile.mkdtemp(prefix="run-", dir=base))
    password = secrets.token_hex(24)
    env = {key: value for key, value in os.environ.items() if not key.startswith(("APP_", "GOSEED_TEST_"))}
    env.update(
        GOSEED_TEST_PASSWORD=password,
        GOSEED_TEMPLATE_DB="1",
        GOTOOLCHAIN="go1.26.4",
        GOWORK="off",
        GOCACHE=str(root / ".tmp/go-cache"),
        GOMODCACHE=str(root / ".tmp/mod-cache"),
        GOLANGCI_LINT_CACHE=str(root / ".tmp/lint-cache"),
        COMPOSE_PROGRESS="plain",
    )
    project = "goseed-template-db-" + secrets.token_hex(6)
    override = directory / "compose.tls.yaml"
    compose = ["docker", "compose", "-p", project, "-f", str(root / "tests/compatibility/compose.yaml"), "-f", str(override)]

    def run(arguments, capture=False, cwd=root, timeout=300):
        return subprocess.run(arguments, cwd=cwd, env=env, check=True, text=True, capture_output=capture, timeout=timeout)

    started = False
    try:
        configuration = directory / "openssl.cnf"
        configuration.write_text("""[req]
distinguished_name = dn
[dn]
[ca]
basicConstraints = critical,CA:true
keyUsage = critical,keyCertSign,cRLSign
[server]
basicConstraints = critical,CA:false
keyUsage = critical,digitalSignature,keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = IP:127.0.0.1
""")
        run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1", "-subj", "/CN=goseed-template-test-ca", "-config", str(configuration), "-extensions", "ca", "-keyout", str(directory / "ca.key"), "-out", str(directory / "ca.crt")], True)
        run(["openssl", "req", "-new", "-newkey", "rsa:2048", "-nodes", "-subj", "/CN=goseed-template-test-server", "-config", str(configuration), "-keyout", str(directory / "server.key"), "-out", str(directory / "server.csr")], True)
        run(["openssl", "x509", "-req", "-days", "1", "-in", str(directory / "server.csr"), "-CA", str(directory / "ca.crt"), "-CAkey", str(directory / "ca.key"), "-CAcreateserial", "-extfile", str(configuration), "-extensions", "server", "-out", str(directory / "server.crt")], True)
        for name in ("ca.key", "server.key"):
            (directory / name).chmod(0o600)
        # Public certificates are readable after each database drops root.
        for name in ("ca.crt", "server.crt"):
            (directory / name).chmod(0o644)
        services = ["services:"]
        for service, account in (("mysql", "mysql"), ("postgres", "postgres")):
            services.extend([
                f"  {service}:",
                "    volumes:",
                f"      - {directory}:/testtls:ro",
                "    entrypoint:",
                "      - /bin/sh",
                "      - -c",
                f'      - cp /testtls/server.key /tmp/goseed-server.key && chmod 600 /tmp/goseed-server.key && chown {account}:{account} /tmp/goseed-server.key && exec docker-entrypoint.sh "$$@"',
                "      - --",
            ])
            if service == "mysql":
                services.extend([
                    "    environment:",
                    '      MYSQL_ROOT_HOST: "%"',
                    "    command: [mysqld, --ssl-ca=/testtls/ca.crt, --ssl-cert=/testtls/server.crt, --ssl-key=/tmp/goseed-server.key]",
                ])
            else:
                services.append("    command: [postgres, -c, ssl=on, -c, ssl_cert_file=/testtls/server.crt, -c, ssl_key_file=/tmp/goseed-server.key]")
        override.write_text("\n".join(services) + "\n")
        print("Starting isolated TLS database project: " + project, flush=True)
        started = True
        run(compose + ["up", "-d", "--wait", "--wait-timeout", "180"], timeout=900)
        for service, port in (("mysql", "3306"), ("postgres", "5432")):
            address = run(compose + ["port", service, port], True).stdout.strip().rsplit(":", 1)[1]
            env["GOSEED_TEST_" + service.upper() + "_PORT"] = address
            container = run(compose + ["ps", "-q", service], True).stdout.strip()
            image = run(["docker", "inspect", "--format", "{{.Image}}", container], True).stdout.strip()
            digest = run(["docker", "image", "inspect", "--format", "{{json .RepoDigests}}", image], True).stdout.strip()
            print(service + " tested image: " + digest, flush=True)
        env["GOSEED_TEST_CA_FILE"] = str(directory / "ca.crt")
        # Verify the clean v1 baseline first, then explicitly upgrade the owned
        # application database to the Casbin v2 schema in a separate invocation.
        for test in ("TestExternalSQLProjects", "TestExternalCasbinProjects", "TestExternalGRPCProjects", "TestExternalDualProjects", "TestExternalGatewayProjects", "TestExternalMigrationVersionProjects", "TestExternalResourceCoreProjects"):
            run(["go", "test", "-mod=readonly", "-count=1", "-v", "-timeout=15m", "./tests/integration", "-run", "^" + test + "$"], timeout=960)
    finally:
        if started:
            cleanup = subprocess.run(compose + ["down", "--volumes", "--remove-orphans"], cwd=root, env=env)
            if cleanup.returncode:
                print("Cleanup failed; owned resources/config requiring cleanup: " + project + " " + str(directory), file=sys.stderr)
                raise SystemExit(cleanup.returncode)
        # Only the unique task directory minted by this invocation.
        shutil.rmtree(directory)


if __name__ == "__main__":
    main()

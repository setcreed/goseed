"""Verify actual Redis templates with an owned, password-protected TLS instance."""

import json
import os
from pathlib import Path
import secrets
import shutil
import socket
import subprocess
import sys
import tempfile


def main():
    root = Path(__file__).resolve().parents[1]
    base = root / ".tmp/template-redis"
    base.mkdir(parents=True, exist_ok=True)
    directory = Path(tempfile.mkdtemp(prefix="run-", dir=base))
    directory.chmod(0o755)  # Public certs are read after the container drops root.
    name = "goseed-template-redis-" + secrets.token_hex(6)
    with socket.socket() as reservation:
        reservation.bind(("127.0.0.1", 0))
        port = reservation.getsockname()[1]
    password = secrets.token_hex(24)
    image = json.loads((root / "dependencies.lock.json").read_text())["redis_image"]
    env = {k: v for k, v in os.environ.items() if not k.startswith(("APP_", "GOSEED_TEST_", "GOSEED_TEMPLATE_"))}
    env.update(GOTOOLCHAIN="local", GOWORK="off", GOCACHE=str(root / ".tmp/go-cache"), GOMODCACHE=str(root / ".tmp/mod-cache"), GOLANGCI_LINT_CACHE=str(root / ".tmp/lint-cache"))

    def run(args, capture=False, timeout=300):
        return subprocess.run(args, cwd=root, env=env, check=True, text=True, capture_output=capture, timeout=timeout)

    created = False
    try:
        config = directory / "openssl.cnf"
        config.write_text("""[req]
distinguished_name=dn
[dn]
[ca]
basicConstraints=critical,CA:true
keyUsage=critical,keyCertSign,cRLSign
[server]
basicConstraints=critical,CA:false
keyUsage=critical,digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth
subjectAltName=IP:127.0.0.1
""")
        run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1", "-subj", "/CN=goseed-redis-test-ca", "-config", str(config), "-extensions", "ca", "-keyout", str(directory / "ca.key"), "-out", str(directory / "ca.crt")], True)
        run(["openssl", "req", "-new", "-newkey", "rsa:2048", "-nodes", "-subj", "/CN=goseed-redis-test-server", "-config", str(config), "-keyout", str(directory / "server.key"), "-out", str(directory / "server.csr")], True)
        run(["openssl", "x509", "-req", "-days", "1", "-in", str(directory / "server.csr"), "-CA", str(directory / "ca.crt"), "-CAkey", str(directory / "ca.key"), "-CAcreateserial", "-extfile", str(config), "-extensions", "server", "-out", str(directory / "server.crt")], True)
        (directory / "password").write_text(password)
        (directory / "redis.conf").write_text('bind 0.0.0.0\nport 0\ntls-port 6379\ntls-cert-file /testtls/server.crt\ntls-key-file /tmp/test-server.key\ntls-ca-cert-file /testtls/ca.crt\ntls-auth-clients no\nsave ""\nappendonly no\nrequirepass ' + password + '\n')
        for file in directory.iterdir():
            file.chmod(0o600 if file.name in {"ca.key", "server.key", "password", "redis.conf"} else 0o644)
        print("Starting owned TLS Redis instance: " + name, flush=True)
        created = True
        # Persist the selected host port: Docker assigns a new ephemeral port after stop/start otherwise.
        run(["docker", "run", "-d", "--name", name, "--label", "org.goseed.redis=" + name, "-p", f"127.0.0.1:{port}:6379", "-v", str(directory) + ":/testtls:ro", "--entrypoint", "sh", image, "-c", "cp /testtls/server.key /tmp/test-server.key && cp /testtls/redis.conf /tmp/redis.conf && cp /testtls/password /tmp/test-password && chmod 600 /tmp/test-server.key /tmp/redis.conf /tmp/test-password && chown redis:redis /tmp/test-server.key /tmp/redis.conf /tmp/test-password && exec docker-entrypoint.sh redis-server /tmp/redis.conf"], True, 900)
        run(["docker", "exec", name, "sh", "-c", "for i in $(seq 1 100); do REDISCLI_AUTH=$(cat /tmp/test-password) redis-cli --tls --cacert /testtls/ca.crt -h 127.0.0.1 ping 2>/dev/null | grep -q PONG && exit 0; sleep .05; done; exit 1"], True)
        address = run(["docker", "port", name, "6379/tcp"], True).stdout.strip()
        print("Redis tested image: " + run(["docker", "image", "inspect", "--format", "{{json .RepoDigests}}", image], True).stdout.strip(), flush=True)
        env.update(GOSEED_TEMPLATE_REDIS="1", GOSEED_TEST_REDIS_CONTAINER=name, GOSEED_TEST_REDIS_ADDRESS=address, GOSEED_TEST_REDIS_PASSWORD=password, GOSEED_TEST_REDIS_CA_FILE=str(directory / "ca.crt"))
        run(["go", "test", "-mod=readonly", "-count=1", "-timeout=15m", "-v", "./tests/integration", "-run", "^TestReal(Redis|GRPC|Dual|Gateway)Projects$"], timeout=960)
    finally:
        if created:
            cleanup = subprocess.run(["docker", "rm", "-f", name], cwd=root, env=env, capture_output=True, text=True)
            if cleanup.returncode and "No such container" not in cleanup.stderr:
                print("Cleanup failed; owned resource/config: " + name + " " + str(directory), file=sys.stderr)
                raise SystemExit(cleanup.returncode)
        shutil.rmtree(directory)


if __name__ == "__main__":
    main()

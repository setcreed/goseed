"""Run database proofs in a unique Compose project; never use existing data."""

import os
from pathlib import Path
import secrets
import subprocess
import sys


def main():
    root = Path(__file__).resolve().parents[1]
    env = os.environ.copy()
    password = secrets.token_hex(24)
    env.update(
        GOSEED_TEST_PASSWORD=password,
        GOTOOLCHAIN="go1.26.4",
        GOWORK="off",
        GOCACHE=str(root / ".tmp/go-cache"),
        GOMODCACHE=str(root / ".tmp/mod-cache"),
        CGO_ENABLED="0",
        COMPOSE_PROGRESS="plain",
    )
    project = "goseed-compat-" + secrets.token_hex(6)
    compose = ["docker", "compose", "-p", project, "-f", str(root / "tests/compatibility/compose.yaml")]

    def run(arguments, capture=False, cwd=root, timeout=300):
        return subprocess.run(arguments, cwd=cwd, env=env, check=True, text=True, capture_output=capture, timeout=timeout)

    print("Starting isolated database project: " + project, flush=True)
    try:
        run(compose + ["up", "-d", "--wait", "--wait-timeout", "180"], timeout=900)
        mysql_port = run(compose + ["port", "mysql", "3306"], True).stdout.strip().rsplit(":", 1)[1]
        postgres_port = run(compose + ["port", "postgres", "5432"], True).stdout.strip().rsplit(":", 1)[1]
        env.update(
            GOSEED_REQUIRE_EXTERNAL="1",
            GOSEED_TEST_MYSQL_DSN=f"goseed_test:{password}@tcp(127.0.0.1:{mysql_port})/goseed_test?parseTime=true&charset=utf8mb4",
            GOSEED_TEST_POSTGRES_DSN=f"host=127.0.0.1 port={postgres_port} user=goseed_test password={password} dbname=goseed_test sslmode=disable",
        )
        run(["go", "test", "-mod=readonly", "-count=1", "-v", "-timeout=2m", "./..."], cwd=root / "tests/compatibility")
        for service in ("mysql", "postgres"):
            container = run(compose + ["ps", "-q", service], True).stdout.strip()
            image = run(["docker", "inspect", "--format", "{{.Image}}", container], True).stdout.strip()
            digest = run(["docker", "image", "inspect", "--format", "{{json .RepoDigests}}", image], True).stdout.strip()
            print(service + " tested image: " + digest, flush=True)
    finally:
        # Scope cleanup to this invocation's project and its fresh test volumes.
        cleanup = subprocess.run(compose + ["down", "--volumes", "--remove-orphans"], cwd=root, env=env)
        if cleanup.returncode:
            print("Cleanup failed; project requiring manual cleanup: " + project, file=sys.stderr)
            raise SystemExit(cleanup.returncode)


if __name__ == "__main__":
    main()

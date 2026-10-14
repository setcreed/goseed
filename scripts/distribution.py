#!/usr/bin/env python3
"""Build and verify local, versioned goseed binary archives."""

import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys
import tarfile
import tempfile


ROOT = Path(__file__).resolve().parent.parent
GO_VERSION = "1.26.4"
VERSION_PATTERN = re.compile(r"^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$")


def run(args, *, cwd=ROOT, env=None):
    return subprocess.run(args, cwd=cwd, env=env, check=True, text=True, capture_output=True)


def environment():
    env = os.environ.copy()
    env["GOTOOLCHAIN"] = "go" + GO_VERSION
    env["GOWORK"] = "off"
    env["GOCACHE"] = str(ROOT / ".tmp" / "go-cache")
    env["GOMODCACHE"] = str(ROOT / ".tmp" / "mod-cache")
    return env


def module_licenses(env):
    result = run(["go", "list", "-mod=readonly", "-deps", "-json", "./cmd/goseed"], env=env)
    decoder = json.JSONDecoder()
    text = result.stdout
    offset = 0
    modules = {}
    while offset < len(text):
        while offset < len(text) and text[offset].isspace():
            offset += 1
        if offset == len(text):
            break
        item, offset = decoder.raw_decode(text, offset)
        module = item.get("Module")
        if not module or not module.get("Dir") or not module.get("Version"):
            continue
        modules[(module["Path"], module["Version"])] = Path(module["Dir"])

    licenses = []
    for (module, version), directory in sorted(modules.items()):
        found = sorted(
            path for path in directory.iterdir()
            if path.is_file() and re.match(r"^(license|copying|notice)(?:[._ -].*)?$", path.name, re.IGNORECASE)
        )
        if not found:
            raise RuntimeError("no license/notice file found for build dependency " + module + "@" + version)
        for source in found:
            archive_path = "licenses/" + module + "@" + version + "/" + source.name
            licenses.append((archive_path, source.read_bytes()))
    return licenses


def source_constant(path, name, pattern):
    text = path.read_text(encoding="utf-8")
    match = re.search(pattern, text, re.MULTILINE)
    if not match:
        raise RuntimeError("cannot read source version constant " + name)
    return match.group(1)


def build(args):
    validate_version(args.version)

    env = environment()
    if not args.goos or not args.goarch:
        target = run(["go", "env", "GOOS", "GOARCH"], env=env).stdout.splitlines()
        goos = args.goos or target[0]
        goarch = args.goarch or target[1]
    else:
        goos, goarch = args.goos, args.goarch

    actual_go = run(["go", "version"], env=env).stdout.strip()
    if not re.search(r"\bgo" + re.escape(GO_VERSION) + r"\b", actual_go):
        raise RuntimeError("expected Go " + GO_VERSION + ", got " + actual_go)

    status = run(["git", "status", "--porcelain", "--untracked-files=all"]).stdout.strip()
    if status and not args.allow_dirty:
        raise RuntimeError("refusing to package a dirty working tree; commit or discard changes first")
    commit = run(["git", "rev-parse", "--verify", "HEAD"]).stdout.strip()
    if not re.fullmatch(r"[0-9a-f]{40,64}", commit):
        raise RuntimeError("cannot determine full source commit")

    out = Path(args.output).resolve()
    out.mkdir(parents=True, exist_ok=True)
    name = "goseed_{}_{}_{}.tar.gz".format(args.version, goos, goarch)
    archive = out / name
    folder = "goseed_{}_{}_{}".format(args.version, goos, goarch)
    with tempfile.TemporaryDirectory(prefix="goseed-dist-", dir=str(ROOT / ".tmp")) as temp:
        stage = Path(temp) / folder
        stage.mkdir()
        binary = stage / "goseed"
        run([
            "go", "build", "-mod=readonly", "-trimpath",
            "-ldflags", "-X main.version=" + args.version + " -X main.commit=" + commit,
            "-o", str(binary), "./cmd/goseed",
        ], env=env)

        licenses = module_licenses(env)
        license_records = []
        for relpath, data in licenses:
            target = stage / PurePosixPath(relpath)
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(data)
            license_records.append({"path": relpath, "sha256": hashlib.sha256(data).hexdigest()})

        (stage / "LICENSE").write_bytes((ROOT / "LICENSE").read_bytes())
        (stage / "README.md").write_bytes((ROOT / "README.md").read_bytes())
        binary_digest = hashlib.sha256(binary.read_bytes()).hexdigest()
        manifest = {
            "generator_version": args.version,
            "source_commit": commit,
            "source_dirty": bool(status),
            "go_version": GO_VERSION,
            "goos": goos,
            "goarch": goarch,
            "template_version": int(source_constant(ROOT / "internal/generator/generator.go", "TemplateVersion", r"const TemplateVersion = ([0-9]+)")),
            "resource_spec_version": int(source_constant(ROOT / "internal/generator/resource_spec.go", "ResourceSpecVersion", r"const ResourceSpecVersion = ([0-9]+)")),
            "binary": {"path": "goseed", "sha256": binary_digest},
            "third_party_licenses": license_records,
        }
        (stage / "manifest.json").write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")

        out.mkdir(parents=True, exist_ok=True)
        with archive.open("wb") as raw:
            with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0, compresslevel=9) as compressed:
                with tarfile.open(fileobj=compressed, mode="w") as tar:
                    for item in sorted(stage.rglob("*")):
                        if item.is_file():
                            rel = item.relative_to(stage).as_posix()
                            info = tar.gettarinfo(str(item), arcname=folder + "/" + rel)
                            info.uid = 0
                            info.gid = 0
                            info.uname = ""
                            info.gname = ""
                            info.mtime = 0
                            info.mode = 0o755 if item == binary else 0o644
                            with item.open("rb") as source:
                                tar.addfile(info, source)

    digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    archive.with_suffix(archive.suffix + ".sha256").write_text(digest + "  " + archive.name + "\n", encoding="ascii")
    print(str(archive))
    print("SHA-256 " + digest)


def validate_version(value):
    if not VERSION_PATTERN.fullmatch(value):
        raise RuntimeError("VERSION must be a semantic version beginning with v, for example v0.1.0")


def verify(path):
    archive = Path(path).resolve()
    checksum = archive.with_suffix(archive.suffix + ".sha256")
    expected, _, filename = checksum.read_text(encoding="ascii").strip().partition("  ")
    if filename != archive.name or not re.fullmatch(r"[0-9a-f]{64}", expected):
        raise RuntimeError("invalid archive checksum sidecar")
    actual = hashlib.sha256(archive.read_bytes()).hexdigest()
    if actual != expected:
        raise RuntimeError("archive SHA-256 mismatch")

    with tarfile.open(archive, "r:gz") as tar:
        files = {}
        roots = set()
        for member in tar.getmembers():
            pure = PurePosixPath(member.name)
            if pure.is_absolute() or ".." in pure.parts or not member.isfile() or member.name in files:
                raise RuntimeError("archive contains an unsafe or duplicate entry")
            roots.add(pure.parts[0])
            stream = tar.extractfile(member)
            if stream is None:
                raise RuntimeError("cannot read archive entry " + member.name)
            files[member.name] = stream.read()
        if len(roots) != 1:
            raise RuntimeError("archive must contain one top-level directory")
        root = next(iter(roots))
        manifest_path = root + "/manifest.json"
        if manifest_path not in files:
            raise RuntimeError("archive has no manifest.json")
        manifest = json.loads(files[manifest_path])
        binary_path = root + "/" + manifest["binary"]["path"]
        binary = files.get(binary_path)
        if binary is None or hashlib.sha256(binary).hexdigest() != manifest["binary"]["sha256"]:
            raise RuntimeError("binary SHA-256 mismatch")
        for entry in manifest["third_party_licenses"]:
            data = files.get(root + "/" + entry["path"])
            if data is None or hashlib.sha256(data).hexdigest() != entry["sha256"]:
                raise RuntimeError("third-party license checksum mismatch: " + entry["path"])
        if not (root + "/LICENSE" in files and root + "/README.md" in files):
            raise RuntimeError("archive is missing project documentation or license")
    print("Verified {} {} {} from {}".format(manifest["generator_version"], manifest["goos"], manifest["goarch"], manifest["source_commit"]))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    package = subparsers.add_parser("package", help="build a local binary archive")
    package.add_argument("--version", required=True)
    package.add_argument("--goos", default="")
    package.add_argument("--goarch", default="")
    package.add_argument("--output", required=True)
    package.add_argument("--allow-dirty", action="store_true", help="allow a local QA archive from modified sources")
    package.set_defaults(func=build)
    check = subparsers.add_parser("verify", help="verify an archive and its embedded manifest")
    check.add_argument("archive")
    check.set_defaults(func=lambda args: verify(args.archive))
    version = subparsers.add_parser("validate-version", help="validate an install version")
    version.add_argument("version")
    version.set_defaults(func=lambda args: validate_version(args.version))
    args = parser.parse_args()
    try:
        args.func(args)
    except (OSError, RuntimeError, subprocess.CalledProcessError, json.JSONDecodeError, tarfile.TarError) as exc:
        detail = exc.stderr.strip() if isinstance(exc, subprocess.CalledProcessError) and exc.stderr else str(exc)
        print("distribution: " + detail, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

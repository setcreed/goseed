"""Check pinned module/tool contracts and local documentation references."""

import ast
import json
from pathlib import Path
import re
import subprocess


def main():
    root = Path(__file__).resolve().parents[1]
    lock = json.loads((root / "dependencies.lock.json").read_text())
    for relative, section in ((".", "generator"), ("tests/compatibility", "runtime")):
        directory = root / relative
        manifest = (directory / "go.mod").read_text()
        actual = dict(re.findall(r"^\s*(?:require\s+)?(\S+)\s+(v\S+)", manifest, re.MULTILINE))
        expected_module = lock["module"] + ("/tests/compatibility" if section == "runtime" else "")
        if not re.search(r"^module " + re.escape(expected_module) + r"$", manifest, re.MULTILINE):
            raise SystemExit("Module path mismatch: " + relative)
        if not re.search(r"^go " + re.escape(lock["go"]) + r"$", manifest, re.MULTILINE):
            raise SystemExit("Go version mismatch: " + relative)
        for module, version in lock[section].items():
            if actual.get(module) != version:
                raise SystemExit("Pinned dependency mismatch: " + module)
    makefile = (root / "Makefile").read_text()
    compatibility_manifest = (root / "tests/compatibility/go.mod").read_text()
    for module, version in lock["sql_connectors"].items():
        if not re.search(r"^\s*" + re.escape(module) + r"\s+" + re.escape(version) + r"(?:\s|$)", compatibility_manifest, re.MULTILINE):
            raise SystemExit("SQL connector differs from compatibility baseline: " + module)
    lint_version = lock["tools"]["github.com/golangci/golangci-lint/v2/cmd/golangci-lint"]
    if "LINT_VERSION := " + lint_version not in makefile or "GOTOOLCHAIN := go" + lock["go"] not in makefile:
        raise SystemExit("Makefile tool version mismatch")
    compose = (root / "tests/compatibility/compose.yaml").read_text()
    for image in lock["database_images"].values():
        if "image: " + image not in compose:
            raise SystemExit("Database image version mismatch")
    files = list(root.glob("*.md")) + list((root / "docs").glob("*.md"))
    links = 0
    for path in files:
        content = path.read_text()
        for target in re.findall(r"\]\(([^)]+)\)", content):
            if "://" in target or target.startswith("#"):
                continue
            target = target.split("#", 1)[0]
            if not (path.parent / target).is_file():
                raise SystemExit("Broken local document link: " + str(path) + " -> " + target)
            links += 1
    for path in (root / "scripts").glob("*.py"):
        ast.parse(path.read_text(), filename=str(path))
    # Includes untracked delivery files, unlike git diff --check alone.
    delivery = subprocess.run(["git", "ls-files", "--cached", "--others", "--exclude-standard"], cwd=root, check=True, text=True, capture_output=True).stdout.splitlines()
    for relative in delivery:
        path = root / relative
        data = path.read_bytes()
        if data and not data.endswith(b"\n"):
            raise SystemExit("Missing final newline: " + relative)
        if any(line.endswith((b" ", b"\t")) for line in data.splitlines()):
            raise SystemExit("Trailing whitespace: " + relative)
    print(f"Module/tool contracts, {links} local links, Python syntax and {len(delivery)} delivery files passed")


if __name__ == "__main__":
    main()

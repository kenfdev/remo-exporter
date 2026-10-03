#!/usr/bin/env python3
"""Test command construction and publication guards without Docker or a registry."""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]


class PackagingTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.log = self.directory / "commands.jsonl"
        stub = f"""#!{sys.executable}
import json, os, sys
with open(os.environ['COMMAND_LOG'], 'a') as log:
    log.write(json.dumps({{
        'command': os.path.basename(sys.argv[0]),
        'args': sys.argv[1:],
        'cwd': os.getcwd(),
        'env': {{key: os.environ.get(key) for key in
                ['GOOS', 'GOARCH', 'GOARM', 'CGO_ENABLED']}},
    }}) + '\\n')
with open(os.environ['COMMAND_LOG']) as log:
    count = len(log.readlines())
if (sys.argv[1] == os.environ.get('FAIL_COMMAND')
        or str(count) == os.environ.get('FAIL_AT')):
    sys.exit(9)
"""
        for command in ("docker", "go"):
            path = self.directory / command
            path.write_text(stub)
            path.chmod(0o755)
        self.environment = {
            **os.environ,
            "PATH": f"{self.directory}{os.pathsep}{os.environ['PATH']}",
            "COMMAND_LOG": str(self.log),
        }
        for name in ("CI", "CIRCLECI", "GITHUB_ACTIONS", "FAIL_COMMAND", "FAIL_AT"):
            self.environment.pop(name, None)

    def run_script(self, path, *arguments, environment=None):
        return subprocess.run(
            ["sh", str(ROOT / path), *arguments],
            cwd=self.directory,
            env={**self.environment, **(environment or {})},
            text=True,
            capture_output=True,
            check=False,
        )

    def commands(self):
        if not self.log.exists():
            return []
        return [json.loads(line) for line in self.log.read_text().splitlines()]

    def test_legacy_and_invalid_publish_calls_never_invoke_docker(self):
        for arguments in (
            (), ("v1.2.3",), ("master-deadbeef",), ("branch-deadbeef",),
            ("--publish",), ("--publish", ""), ("--publish", "v"),
            ("--publish", "bad/tag"), ("--publish", "-bad"),
            ("--publish", "v1", "repo", "extra"),
        ):
            with self.subTest(arguments=arguments):
                result = self.run_script("packaging/docker/push-image.sh", *arguments)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(self.commands(), [])

    def test_ci_never_invokes_docker_even_with_publish_flag(self):
        for name in ("CI", "CIRCLECI", "GITHUB_ACTIONS"):
            for value in ("true", "false", "0"):
                with self.subTest(name=name, value=value):
                    result = self.run_script(
                        "packaging/docker/push-image.sh", "--publish", "v1.2.3",
                        environment={name: value},
                    )
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn("disabled in CI", result.stderr)
                    self.assertEqual(self.commands(), [])

    def test_legacy_circleci_test_step_stops_before_coverage_upload(self):
        result = self.run_script(
            "scripts/run-test-with-coverage.sh", environment={"CIRCLECI": "true"}
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Refusing the legacy CircleCI upload pipeline", result.stderr)
        self.assertEqual(self.commands(), [])

    def test_binary_builds_are_path_independent_and_keep_all_architectures(self):
        result = self.run_script("scripts/build-bin.sh", "-ldflags=-s -w")
        self.assertEqual(result.returncode, 0, result.stderr)
        commands = self.commands()
        self.assertEqual(len(commands), 3)
        for command, arch, suffix in zip(
            commands, ("amd64", "arm64", "arm"), ("amd64", "arm64", "armv7")
        ):
            self.assertEqual(command["command"], "go")
            self.assertEqual(command["cwd"], str(ROOT))
            self.assertEqual(command["env"], {
                "GOOS": "linux", "GOARCH": arch, "GOARM": "7", "CGO_ENABLED": "0",
            })
            self.assertEqual(command["args"], [
                "build", "-trimpath", "-ldflags=-s -w", "-o",
                f"dist/remo-exporter-linux-{suffix}", ".",
            ])

    def test_build_failure_stops_remaining_architectures(self):
        result = self.run_script("scripts/build-bin.sh", environment={"FAIL_COMMAND": "build"})
        self.assertEqual(result.returncode, 9)
        self.assertEqual(len(self.commands()), 1)

    def test_image_builds_select_matching_platforms_and_keep_dockerhub_names(self):
        result = self.run_script("packaging/docker/build-image.sh", "v1.2.3")
        self.assertEqual(result.returncode, 0, result.stderr)
        commands = self.commands()
        builds = commands[:3]
        self.assertEqual(len(commands), 6)
        for command, platform, binary, suffix in zip(
            builds,
            ("linux/amd64", "linux/arm/v7", "linux/arm64"),
            ("amd64", "armv7", "arm64"),
            ("", "-linux-arm32v7", "-linux-arm64v8"),
        ):
            self.assertEqual(command["cwd"], str(ROOT))
            self.assertEqual(command["args"], [
                "build", "--platform", platform,
                "--build-arg", f"EXPORTER_BINARY=remo-exporter-linux-{binary}",
                "--tag", f"kenfdev/remo-exporter{suffix}:1.2.3",
                "--file", "packaging/docker/Dockerfile", "dist",
            ])
        self.assertEqual([command["args"] for command in commands[3:]], [
            ["tag", f"kenfdev/remo-exporter{suffix}:1.2.3",
             f"kenfdev/remo-exporter{suffix}:latest"]
            for suffix in ("", "-linux-arm32v7", "-linux-arm64v8")
        ])

    def test_opted_in_manual_release_constructs_manifests_without_login(self):
        result = self.run_script("packaging/docker/push-image.sh", "--publish", "v1.2.3")
        self.assertEqual(result.returncode, 0, result.stderr)
        commands = [command["args"] for command in self.commands()]
        self.assertFalse(any(command[0] == "login" for command in commands))
        self.assertEqual(len(commands), 12)
        for index, tag in enumerate(("1.2.3", "latest")):
            group = commands[index * 6:(index + 1) * 6]
            self.assertEqual(group[:3], [
                ["push", f"kenfdev/remo-exporter{suffix}:{tag}"]
                for suffix in ("", "-linux-arm32v7", "-linux-arm64v8")
            ])
            self.assertEqual(group[3], [
                "manifest", "create", "--amend", f"kenfdev/remo-exporter:{tag}",
                f"kenfdev/remo-exporter:{tag}",
                f"kenfdev/remo-exporter-linux-arm32v7:{tag}",
                f"kenfdev/remo-exporter-linux-arm64v8:{tag}",
            ])
            self.assertEqual(group[4], [
                "manifest", "annotate", f"kenfdev/remo-exporter:{tag}",
                f"kenfdev/remo-exporter-linux-arm32v7:{tag}",
                "--os", "linux", "--arch", "arm", "--variant", "v7",
            ])
            self.assertEqual(group[5], [
                "manifest", "push", "--purge", f"kenfdev/remo-exporter:{tag}",
            ])

    def test_release_channels_only_promote_prefixed_stable_versions(self):
        for tag, promote in (
            ("v0.9.0-rc.1", False), ("0.9.0-rc.1", False),
            ("v1.2.3-alpha", False), ("1.2.3-beta.2", False),
            ("v1.2.3-0", False), ("v1.2.3-01a", False),
            ("v1.2.3-x-y.9", False), ("v0.0.0", True),
            ("v1.2.3", True), ("1.2.3", False),
        ):
            for script in ("build-image.sh", "push-image.sh"):
                with self.subTest(tag=tag, script=script):
                    self.log.unlink(missing_ok=True)
                    args = ("--publish", tag) if script == "push-image.sh" else (tag,)
                    result = self.run_script(f"packaging/docker/{script}", *args)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    commands = [entry["args"] for entry in self.commands()]
                    refs = [arg for command in commands for arg in command
                            if arg.startswith("kenfdev/")]
                    self.assertTrue(refs)
                    allowed = {tag.removeprefix("v")}
                    if promote:
                        allowed.add("latest")
                    self.assertEqual({ref.rsplit(":", 1)[1] for ref in refs}, allowed)
                    self.assertFalse(any("-dev:" in ref for ref in refs))
                    self.assertEqual(len(commands),
                                     (12 if promote else 6) if script == "push-image.sh"
                                     else (6 if promote else 3))

    def test_malformed_release_tags_fail_before_any_docker_command(self):
        for tag in ("v1", "1.2", "v01.2.3", "1.02.3", "v1.2.03",
                    "v1.2.3-", "1.2.3-rc..1", "v1.2.3-01", "1.2.3-rc.01",
                    "v1.2.3+build.1", "1.2.3+build.1", "vlatest",
                    "v1.2.3_rc1", "v1.2.3\n", "branch\nname", "v1.2.3\nbad", "v" + "1" * 129):
            for script in ("build-image.sh", "push-image.sh"):
                with self.subTest(tag=tag, script=script):
                    args = ("--publish", tag) if script == "push-image.sh" else (tag,)
                    result = self.run_script(f"packaging/docker/{script}", *args)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertEqual(self.commands(), [])

    def test_failed_version_publication_never_reaches_latest(self):
        for step in range(1, 7):
            with self.subTest(step=step):
                self.log.unlink(missing_ok=True)
                result = self.run_script(
                    "packaging/docker/push-image.sh", "--publish", "v1.2.3",
                    environment={"FAIL_AT": str(step)},
                )
                self.assertEqual(result.returncode, 9)
                self.assertEqual(len(self.commands()), step)
                self.assertNotIn(":latest", json.dumps(self.commands()))

    def test_development_build_aliases_are_preserved(self):
        for tag in ("master-test", "branch-test"):
            with self.subTest(tag=tag):
                self.log.unlink(missing_ok=True)
                result = self.run_script("packaging/docker/build-image.sh", tag)
                self.assertEqual(result.returncode, 0, result.stderr)
                commands = [entry["args"] for entry in self.commands()]
                self.assertEqual(len(commands), 7)
                self.assertEqual(commands[3:6], [
                    ["tag", f"kenfdev/remo-exporter{suffix}:{tag}",
                     f"kenfdev/remo-exporter{suffix}:master"]
                    for suffix in ("", "-linux-arm32v7", "-linux-arm64v8")
                ])
                self.assertEqual(commands[-1], [
                    "tag", f"kenfdev/remo-exporter:{tag}",
                    f"kenfdev/remo-exporter-dev:{tag}",
                ])

    def test_custom_repository_never_publishes_to_default_repository(self):
        result = self.run_script(
            "packaging/docker/push-image.sh", "--publish", "master-test", "example/exporter"
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        commands = [command["args"] for command in self.commands()]
        self.assertNotIn("kenfdev/", json.dumps(commands))
        self.assertEqual(commands[-1], ["push", "example/exporter-dev:master-test"])


if __name__ == "__main__":
    unittest.main()

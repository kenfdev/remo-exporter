# Manual releases

Only an authorized maintainer may publish from a non-CI environment. CI and
CircleCI publication remain disabled; `--publish` is always required. Building
images does not publish them. Do not put registry credentials or Nature Remo
secrets in commands, commits, or release notes.

## Channels

| Input to both image scripts | Docker version tag | Additional publication |
| --- | --- | --- |
| `v0.9.0-rc.1` or `0.9.0-rc.1` | `0.9.0-rc.1` | None |
| `0.9.0` | `0.9.0` | None |
| `v0.9.0` | `0.9.0` | `latest` |
| `master-<commit>` | unchanged | `master` and the existing `-dev` repository |

Release versions use SemVer, with an optional leading `v`. Numeric components
and numeric prerelease identifiers cannot have leading zeroes. Build metadata
(`+...`) is rejected because Docker tags cannot represent it. Malformed
release-like inputs (starting with `v` or a digit) fail before any Docker command.
Other Docker-compatible development tags retain their existing behavior; avoid
reserved aliases such as `latest` as input versions. Unprefixed stable versions
are intentionally version-only, allowing verification before promotion.

## Prepare and test a candidate

1. Start from a clean checkout of the intended commit. Record its full SHA and
   confirm that CI passed for that SHA with the supported Go versions. Run
   `python3 scripts/test-packaging.py` for publication regression tests; these
   mock Docker and never contact a registry.
2. Run `go mod verify`, `go vet ./...`, and `go test -race ./...`. Run
   `go mod tidy` and confirm no module changes. Follow the current CI workflow
   for the pinned vulnerability check.
3. Build all three binaries and images. These example commands build only:

   ```sh
   ./scripts/build-bin.sh
   ./packaging/docker/build-image.sh v0.9.0-rc.1
   ```

   Supported platforms are `linux/amd64`, `linux/arm64`, and `linux/arm/v7`.
   The builder needs native workers or preconfigured emulation; these scripts
   do not install it. The optional repository argument must match in both
   build and push scripts. The default is `kenfdev/remo-exporter`.
4. Run the fixture-based binary and container smoke checks on compatible hosts:

   ```sh
   python3 scripts/smoke-test.py dist/remo-exporter-linux-amd64
   python3 scripts/smoke-test.py --image kenfdev/remo-exporter:0.9.0-rc.1
   python3 scripts/smoke-test.py --image kenfdev/remo-exporter-linux-arm64v8:0.9.0-rc.1
   python3 scripts/smoke-test.py --image kenfdev/remo-exporter-linux-arm32v7:0.9.0-rc.1
   ```

   Run each image on its matching native host or a host with working emulation.
   Record build result and runtime result separately for each platform, marking
   runtime as **native**, **emulated**, or **unrun** (with reason). A cross-build
   is not a runtime test. The fixture uses no real token or device; report any
   actual hardware testing separately. Do not claim unrun architectures passed.

## Publish an RC and verify

After reviewing the artifacts and authenticating separately to the intended
registry, an authorized maintainer may run:

```sh
./packaging/docker/push-image.sh --publish v0.9.0-rc.1
```

This publishes the three versioned images and the version manifest only.
It neither tags nor publishes `latest`. Before and after publication, record the
remote `latest` digest to confirm it is unchanged. Inspect remote manifests:

```sh
docker buildx imagetools inspect kenfdev/remo-exporter:0.9.0-rc.1
docker manifest inspect kenfdev/remo-exporter:0.9.0-rc.1
```

Require exactly Linux amd64, arm64, and arm/v7 descriptors, and verify each child
digest against the intended image (the ARM images use the suffix repositories
shown above). Record the index digest and child digests. Pull and smoke-test the
published version on each available platform and record native/emulated/unrun
status again. A successful push alone is not manifest or runtime verification.

## Stable release and latest promotion

Build and test the chosen stable commit using an **unprefixed** version such as
`0.9.0`, then publish it with `--publish 0.9.0`. Verify its remote manifest and
runtime results as above. This leaves `latest` unchanged.

When latest promotion is authorized, tag the same verified local images:

```sh
for suffix in '' -linux-arm32v7 -linux-arm64v8; do
    docker tag "kenfdev/remo-exporter$suffix:0.9.0" "kenfdev/remo-exporter$suffix:latest"
done
./packaging/docker/push-image.sh --publish v0.9.0
```

The prefixed stable invocation republishes the version first, then `latest`.
Confirm the local version images still match the verified artifacts before
running it. Verify the remote version and latest indexes contain the same child
digests and platforms afterward. Do not use RC artifacts for stable promotion.

## Partial publication recovery

Publication is not atomic. A failure stops subsequent commands, but earlier
pushes remain visible. In particular, pushing amd64 to the base repository tag
replaces its index temporarily; a failure can leave a version or even `latest`
with only amd64. Failure in the version phase prevents latest promotion.

Stop and inspect remote tags/digests to establish which stage completed. Keep
the tested local images and recorded prior latest digests. Resolve the failed
registry operation before retrying; do not rebuild a different artifact under
the same version. Remove the affected **local** manifest cache with
`docker manifest rm REPOSITORY:TAG` if it exists, because `--amend` may retain
stale entries. This does not delete remote images.

For an interrupted RC or version-only publication, rerun its original command
with the same verified images, then verify the complete remote manifest. For an
interrupted stable promotion, ensure both version and latest local tags still
point to the intended images and rerun the prefixed stable command. If recovery
requires restoring the previous latest, use the recorded prior image digests
and explicit maintainer authorization. Never substitute RC images. Recheck all
three descriptors, their digests, and available runtimes after recovery; do not
announce a complete release while publication is partial.

## GitHub release

After image verification, an authorized maintainer separately creates the Git
tag at the recorded source SHA and a GitHub release manually. Mark RC releases
as prereleases. Include changes, source SHA, immutable image digests, supported
platforms, the native/emulated/unrun test matrix, and known limitations. Neither
the packaging scripts nor CI creates tags or GitHub releases automatically.

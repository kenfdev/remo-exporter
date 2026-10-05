#!/usr/bin/env python3
"""One-artifact RC publication exception; the general CI guard stays intact."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import urllib.error
import urllib.parse
import urllib.request
import zipfile

REPO = "kenfdev/remo-exporter"
SOURCE = "a19d6f1b9051f09bd91e6b3e008e9588a631fc05"
BUILD_WORKFLOW = "1fb7fff4ef742152be19e8144569b3e7fc3e9530"
RUN = 37191841933
ARTIFACT = 11299541675
ZIP_SHA = "7dd770ac28c959cf91002c989c1e0529c8608a70568e93754480f4e7f1111b24"
ZIP_SIZE = 36078459
VERSION = "0.9.0-rc.1"
TAG = "v" + VERSION
BASE = "alpine@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6"
TARGETS = (
    (REPO, "amd64", "amd64", ""),
    (REPO + "-linux-arm64v8", "arm64", "arm64", ""),
    (REPO + "-linux-arm32v7", "armv7", "arm", "v7"),
)
WORK = Path("rc-publication")
ACCEPT = ", ".join((
    "application/vnd.oci.image.index.v1+json",
    "application/vnd.docker.distribution.manifest.list.v2+json",
    "application/vnd.oci.image.manifest.v1+json",
    "application/vnd.docker.distribution.manifest.v2+json",
))


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


class SafeRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        redirected = super().redirect_request(req, fp, code, msg, headers, newurl)
        if urllib.parse.urlsplit(req.full_url).netloc != urllib.parse.urlsplit(newurl).netloc:
            redirected.remove_header("Authorization")
        return redirected


OPENER = urllib.request.build_opener(SafeRedirect())


def request(url, headers=None, missing_ok=False):
    try:
        with OPENER.open(urllib.request.Request(url, headers=headers or {}), timeout=60) as response:
            return response.read()
    except urllib.error.HTTPError as error:
        if missing_ok and error.code == 404:
            return None
        raise RuntimeError(f"HTTP {error.code} reading {urllib.parse.urlsplit(url).netloc}; stopping") from None


def github(path, missing_ok=False):
    return request(f"https://api.github.com/repos/{REPO}/{path}", {
        "Authorization": "Bearer " + os.environ["GH_TOKEN"],
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
    }, missing_ok)


def docker(*args):
    return subprocess.check_output(["docker", *args], text=True)


def registry(repo, reference, missing_ok=False):
    query = urllib.parse.urlencode({"service": "registry.docker.io", "scope": f"repository:{repo}:pull"})
    token = json.loads(request("https://auth.docker.io/token?" + query))["token"]
    body = request(f"https://registry-1.docker.io/v2/{repo}/manifests/{reference}",
                   {"Authorization": "Bearer " + token, "Accept": ACCEPT}, missing_ok)
    if body is None:
        return None
    return {"digest": "sha256:" + hashlib.sha256(body).hexdigest(), "document": json.loads(body)}


def latest():
    return {repo: (result["digest"] if (result := registry(repo, "latest", True)) else None)
            for repo, *_ in TARGETS}


def absent():
    require(github("git/ref/tags/" + TAG, True) is None, "Git tag exists; do not overwrite")
    require(github("releases/tags/" + TAG, True) is None, "GitHub release exists; do not overwrite")
    for repo, *_ in TARGETS:
        require(registry(repo, VERSION, True) is None, f"RC tag already exists: {repo}; inspect partial publication")


def validate_provenance(directory):
    lines = (directory / "provenance.txt").read_text().splitlines()
    for entry in (f"source_sha={SOURCE}", f"workflow_sha={BUILD_WORKFLOW}",
                  f"base_image={BASE}", f"rc={VERSION}"):
        require(lines.count(entry) == 1, "Artifact provenance mismatch: " + entry.split("=")[0])
    results = (directory / "results.tsv").read_text().splitlines()
    for platform, mode in (("linux/amd64", "native"), ("linux/arm64", "emulated"), ("linux/arm/v7", "emulated")):
        require(f"{platform}\tpass\t{mode} pass" in results, "Missing successful runtime evidence: " + platform)
    require((directory / "latest-before.json").read_bytes() == (directory / "latest-after.json").read_bytes(),
            "Preparation changed latest")


def verify_archive(directory):
    image_file = directory / "rc-images.tar.gz"
    fields = (directory / "archive-sha256.txt").read_text().split()
    require(len(fields) == 2 and fields[1] == "evidence/rc-images.tar.gz", "Unexpected archive checksum entry")
    require(hashlib.sha256(image_file.read_bytes()).hexdigest() == fields[0], "Image archive checksum mismatch")
    expected = {}
    for repo, binary, arch, variant in TARGETS:
        records = json.loads((directory / f"image-{binary}.json").read_text())
        require(len(records) == 1, "Unexpected image inspection count")
        record = records[0]
        require(record["Os"] == "linux" and record["Architecture"] == arch, "Image architecture mismatch")
        require(not variant or record.get("Variant") == variant, "Image variant mismatch")
        require(record["Config"]["User"] == "exporter", "Unexpected runtime user")
        require(record["RepoTags"] == [repo + ":" + VERSION], "Unexpected saved image tags")
        expected[repo + ":" + VERSION] = record
    with tarfile.open(image_file, "r:gz") as archive:
        manifest = json.load(archive.extractfile("manifest.json"))
        require(len(manifest) == 3, "Expected exactly three saved images")
        found = set()
        for item in manifest:
            tags = item.get("RepoTags", [])
            require(len(tags) == 1 and tags[0] in expected and tags[0] not in found, "Unexpected/duplicate tar image tag")
            found.add(tags[0])
            config = archive.extractfile(item["Config"]).read()
            require("sha256:" + hashlib.sha256(config).hexdigest() == expected[tags[0]]["Id"], "Saved image ID mismatch")
        require(found == set(expected), "Missing saved images")
    return expected


def prepare():
    WORK.mkdir()
    run = json.loads(github(f"actions/runs/{RUN}"))
    require(run["head_sha"] == BUILD_WORKFLOW and run["conclusion"] == "success" and run["status"] == "completed",
            "Preparation run identity/status mismatch")
    metadata = json.loads(github(f"actions/artifacts/{ARTIFACT}"))
    require(metadata["id"] == ARTIFACT and not metadata["expired"], "Artifact missing or expired")
    require(metadata["workflow_run"]["id"] == RUN and metadata["workflow_run"]["head_sha"] == BUILD_WORKFLOW,
            "Artifact run mismatch")
    require(metadata["digest"] == "sha256:" + ZIP_SHA and metadata["size_in_bytes"] == ZIP_SIZE,
            "Artifact metadata digest/size mismatch")
    absent()
    data = github(f"actions/artifacts/{ARTIFACT}/zip")
    require(len(data) == ZIP_SIZE and hashlib.sha256(data).hexdigest() == ZIP_SHA, "Artifact ZIP checksum mismatch")
    zip_path = WORK / "artifact.zip"
    zip_path.write_bytes(data)
    directory = WORK / "evidence"
    directory.mkdir()
    with zipfile.ZipFile(zip_path) as archive:
        for entry in archive.infolist():
            require(Path(entry.filename).name == entry.filename and not entry.is_dir(), "Unexpected ZIP path")
            require(entry.filename not in (".", ".."), "Unsafe ZIP path")
        archive.extractall(directory)
    validate_provenance(directory)
    expected = verify_archive(directory)
    print(docker("load", "--input", str(directory / "rc-images.tar.gz")))
    for tag, record in expected.items():
        loaded = json.loads(docker("image", "inspect", tag))[0]
        for key in ("Id", "Os", "Architecture", "Config", "RootFS"):
            require(loaded[key] == record[key], "Loaded image differs: " + key)
    (WORK / "expected.json").write_text(json.dumps(expected))
    (WORK / "latest.json").write_text(json.dumps(latest()))
    print("PASS: exact artifact, provenance, checksums, image IDs, platforms, runtime evidence, and conflicts")


def verify_published(expected, before):
    children = {}
    for repo, _, arch, variant in TARGETS:
        # The base tag is now the index; match each child directly by its digest.
        if repo != REPO:
            image = registry(repo, VERSION)
            require(image["document"]["config"]["digest"] == expected[repo + ":" + VERSION]["Id"],
                    "Published ARM image differs from verified artifact")
            children[arch] = image["digest"]
    index = registry(REPO, VERSION)
    descriptors = index["document"].get("manifests", [])
    require(len(descriptors) == 3, "Expected exactly three manifest descriptors")
    seen = set()
    for descriptor in descriptors:
        platform = descriptor["platform"]
        arch = platform["architecture"]
        require(platform["os"] == "linux" and arch in ("amd64", "arm64", "arm") and arch not in seen,
                "Unexpected/duplicate platform")
        seen.add(arch)
        require(arch != "arm" or platform.get("variant") == "v7", "ARM variant must be v7")
        child = registry(REPO, descriptor["digest"])
        require(child["digest"] == descriptor["digest"], "Child digest mismatch")
        repo = next(repo for repo, _, target_arch, _ in TARGETS if target_arch == arch)
        require(child["document"]["config"]["digest"] == expected[repo + ":" + VERSION]["Id"],
                "Manifest child differs from verified artifact")
        require(arch not in children or children[arch] == child["digest"], "ARM repository/manifest mismatch")
    require(latest() == before, "Latest changed; stop before GitHub release")
    return index


def publish():
    expected = json.loads((WORK / "expected.json").read_text())
    before = json.loads((WORK / "latest.json").read_text())
    require(latest() == before, "Latest changed since preparation; stopping")
    absent()  # Recheck immediately before the first registry write.
    for repo, *_ in TARGETS:
        require(json.loads(docker("image", "inspect", repo + ":" + VERSION))[0]["Id"] == expected[repo + ":" + VERSION]["Id"],
                "Local image changed before publication")
    for repo, *_ in TARGETS:
        print(docker("push", repo + ":" + VERSION))
    tag = REPO + ":" + VERSION
    print(docker("manifest", "create", tag, *(repo + ":" + VERSION for repo, *_ in TARGETS)))
    print(docker("manifest", "annotate", tag, REPO + "-linux-arm32v7:" + VERSION,
                 "--os", "linux", "--arch", "arm", "--variant", "v7"))
    print(docker("manifest", "push", "--purge", tag))
    index = verify_published(expected, before)
    report = {"source": SOURCE, "artifact": ARTIFACT, "index": index, "latest_before_after": before}
    (WORK / "published.json").write_text(json.dumps(report, indent=2))
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write("index_digest=" + index["digest"] + "\n")
    with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as output:
        output.write("Verified RC publication; latest unchanged.\n```json\n" + json.dumps(report, indent=2) + "\n```\n")


def release():
    digest = os.environ["VERIFIED_INDEX_DIGEST"]
    require(registry(REPO, VERSION)["digest"] == digest, "RC manifest changed before release")
    require(github("git/ref/tags/" + TAG, True) is None, "Git tag exists; manual review required")
    require(github("releases/tags/" + TAG, True) is None, "Release exists; manual review required")
    notes = f"""RC from source `{SOURCE}`, using verified preparation run {RUN} / artifact {ARTIFACT}.

Adds optional device-online metrics and fractional temperature offsets; fixes concurrent cache access and malformed-response handling; bounds upstream HTTP requests; modernizes dependencies and packaging; prevents RC tags promoting latest.

Image: `{REPO}:{VERSION}`\nImmutable index: `{REPO}@{digest}`

Linux amd64: native fixture smoke passed. Linux arm64 and ARMv7: emulated fixture smoke passed. Native ARM hardware and the real Nature API are untested. Go checks, race/vet, packaging tests, and vulnerability scan passed. Docker latest was verified unchanged.
"""
    Path("rc-release-notes.txt").write_text(notes)
    subprocess.run(["gh", "api", f"repos/{REPO}/git/refs", "-f", "ref=refs/tags/" + TAG, "-f", "sha=" + SOURCE], check=True)
    subprocess.run(["gh", "release", "create", TAG, "--repo", REPO, "--verify-tag", "--prerelease", "--latest=false",
                    "--title", TAG, "--notes-file", "rc-release-notes.txt"], check=True)
    ref = json.loads(github("git/ref/tags/" + TAG))
    result = json.loads(github("releases/tags/" + TAG))
    require(ref["object"]["sha"] == SOURCE and result["prerelease"] and not result["draft"], "Release verification failed")
    print(result["html_url"])


def main():
    require(os.environ.get("GITHUB_REPOSITORY") == REPO and os.environ.get("GITHUB_EVENT_NAME") == "workflow_dispatch"
            and os.environ.get("GITHUB_REF") == "refs/heads/master" and os.environ.get("APPROVED_RC") == TAG,
            "This exception requires the approved manual RC dispatch on master")
    require(len(sys.argv) == 2 and sys.argv[1] in ("prepare", "publish", "release"), "Invalid phase")
    {"prepare": prepare, "publish": publish, "release": release}[sys.argv[1]]()


if __name__ == "__main__":
    main()

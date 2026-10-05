#!/usr/bin/env python3
"""Publication safety tests. Every Docker, registry, and GitHub call is mocked."""
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest
from unittest.mock import patch
import urllib.error
import urllib.request

spec = importlib.util.spec_from_file_location('publication', Path(__file__).with_name('publish.py'))
p = importlib.util.module_from_spec(spec)
spec.loader.exec_module(p)


class PublicationTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.expected = {}
        self.members = {}
        manifest = []
        for repo, binary, arch, variant in p.TARGETS:
            tag = repo + ':' + p.VERSION
            config = json.dumps({'architecture': arch}).encode()
            digest = hashlib.sha256(config).hexdigest()
            record = {'Id': 'sha256:' + digest, 'Os': 'linux', 'Architecture': arch,
                      'Variant': variant, 'Config': {'User': 'exporter'}, 'RepoTags': [tag]}
            self.expected[tag] = record
            (self.directory / f'image-{binary}.json').write_text(json.dumps([record]))
            self.members[digest + '.json'] = config
            manifest.append({'RepoTags': [tag], 'Config': digest + '.json', 'Layers': []})
        self.manifest = manifest
        self.save_archive()

    def save_archive(self):
        with tarfile.open(self.directory / 'rc-images.tar.gz', 'w:gz') as archive:
            members = {**self.members, 'manifest.json': json.dumps(self.manifest).encode()}
            for name, body in members.items():
                info = tarfile.TarInfo(name)
                info.size = len(body)
                archive.addfile(info, io.BytesIO(body))
        digest = hashlib.sha256((self.directory / 'rc-images.tar.gz').read_bytes()).hexdigest()
        (self.directory / 'archive-sha256.txt').write_text(digest + '  evidence/rc-images.tar.gz\n')

    def test_exact_archive_accepts_all_three_images(self):
        self.assertEqual(p.verify_archive(self.directory), self.expected)

    def test_archive_corruption_fails_closed(self):
        with (self.directory / 'rc-images.tar.gz').open('ab') as out:
            out.write(b'corrupt')
        with self.assertRaisesRegex(RuntimeError, 'checksum'):
            p.verify_archive(self.directory)

    def test_unapproved_saved_tag_rejected(self):
        self.manifest[0]['RepoTags'] = [p.REPO + ':latest']
        self.save_archive()
        with self.assertRaisesRegex(RuntimeError, 'tag'):
            p.verify_archive(self.directory)

    def test_saved_config_tampering_rejected(self):
        self.members[self.manifest[0]['Config']] = b'{}'
        self.save_archive()
        with self.assertRaisesRegex(RuntimeError, 'ID mismatch'):
            p.verify_archive(self.directory)

    def test_provenance_and_runtime_are_required(self):
        lines = [f'source_sha={p.SOURCE}', f'workflow_sha={p.BUILD_WORKFLOW}',
                 f'base_image={p.BASE}', f'rc={p.VERSION}']
        (self.directory / 'provenance.txt').write_text('\n'.join(lines))
        (self.directory / 'results.tsv').write_text('linux/amd64\tpass\tnative pass\nlinux/arm64\tpass\temulated pass\nlinux/arm/v7\tpass\temulated pass\n')
        for suffix in ('before', 'after'):
            (self.directory / f'latest-{suffix}.json').write_text('{}')
        p.validate_provenance(self.directory)
        (self.directory / 'provenance.txt').write_text('\n'.join(lines).replace(p.SOURCE, p.BUILD_WORKFLOW))
        with self.assertRaisesRegex(RuntimeError, 'provenance'):
            p.validate_provenance(self.directory)

    def test_http_denial_is_not_treated_as_missing(self):
        with patch.object(p.OPENER, 'open', side_effect=urllib.error.HTTPError('https://example.com', 403, 'Forbidden', {}, None)):
            with self.assertRaisesRegex(RuntimeError, 'HTTP 403'):
                p.request('https://example.com', missing_ok=True)

    def test_cross_host_redirect_drops_authorization(self):
        request = urllib.request.Request('https://api.github.com/example', headers={'Authorization': 'Bearer fake'})
        redirected = p.SafeRedirect().redirect_request(request, None, 302, '', {}, 'https://artifact.example/archive')
        self.assertIsNone(redirected.get_header('Authorization'))

    def test_existing_rc_prevents_any_write(self):
        with patch.object(p, 'github', return_value=None), patch.object(p, 'registry', return_value={}), patch.object(p, 'docker') as docker:
            with self.assertRaisesRegex(RuntimeError, 'already exists'):
                p.absent()
            docker.assert_not_called()

    def test_publish_only_fixed_version_destinations(self):
        (self.directory / 'expected.json').write_text(json.dumps(self.expected))
        (self.directory / 'latest.json').write_text('{}')
        commands = []
        def docker(*args):
            commands.append(args)
            if args[:2] == ('image', 'inspect'):
                return json.dumps([self.expected[args[2]]])
            return ''
        env = {'GITHUB_OUTPUT': str(self.directory / 'out'), 'GITHUB_STEP_SUMMARY': str(self.directory / 'summary')}
        with patch.object(p, 'WORK', self.directory), patch.object(p, 'latest', return_value={}), patch.object(p, 'absent'), patch.object(p, 'docker', side_effect=docker), patch.object(p, 'verify_published', return_value={'digest': 'sha256:test'}), patch.dict(os.environ, env):
            p.publish()
        writes = [args for args in commands if args[0] != 'image']
        self.assertEqual(len(writes), 6)
        self.assertEqual(writes[:3], [('push', repo + ':' + p.VERSION) for repo, *_ in p.TARGETS])
        self.assertFalse(any(':latest' in arg for args in writes for arg in args))
        self.assertEqual(writes[-1], ('manifest', 'push', '--purge', p.REPO + ':' + p.VERSION))

    def test_push_failure_stops_manifest_and_further_pushes(self):
        (self.directory / 'expected.json').write_text(json.dumps(self.expected))
        (self.directory / 'latest.json').write_text('{}')
        commands = []
        def docker(*args):
            commands.append(args)
            if args[:2] == ('image', 'inspect'):
                return json.dumps([self.expected[args[2]]])
            raise subprocess.CalledProcessError(1, 'docker')
        with patch.object(p, 'WORK', self.directory), patch.object(p, 'latest', return_value={}), patch.object(p, 'absent'), patch.object(p, 'docker', side_effect=docker):
            with self.assertRaises(subprocess.CalledProcessError):
                p.publish()
        self.assertEqual(len(commands), 4)
        self.assertEqual(commands[-1][0], 'push')

    def remote_fixture(self):
        records = {}
        descriptors = []
        for repo, _, arch, variant in p.TARGETS:
            digest = 'sha256:' + arch
            platform = {'os': 'linux', 'architecture': arch}
            if variant:
                platform['variant'] = variant
            descriptors.append({'digest': digest, 'platform': platform})
            child = {'digest': digest, 'document': {'config': {'digest': self.expected[repo + ':' + p.VERSION]['Id']}}}
            records[(repo, p.VERSION)] = child
            records[(p.REPO, digest)] = child
        records[(p.REPO, p.VERSION)] = {'digest': 'sha256:index', 'document': {'manifests': descriptors}}
        return records

    def test_remote_manifest_matches_verified_images(self):
        records = self.remote_fixture()
        with patch.object(p, 'registry', side_effect=lambda repo, ref: records[(repo, ref)]), patch.object(p, 'latest', return_value={}):
            self.assertEqual(p.verify_published(self.expected, {})['digest'], 'sha256:index')

    def test_remote_wrong_image_or_variant_or_latest_rejected(self):
        for failure in ('image', 'variant', 'latest'):
            with self.subTest(failure=failure):
                records = self.remote_fixture()
                if failure == 'image':
                    records[(p.REPO, 'sha256:amd64')]['document']['config']['digest'] = 'sha256:wrong'
                if failure == 'variant':
                    records[(p.REPO, p.VERSION)]['document']['manifests'][2]['platform']['variant'] = 'v6'
                with patch.object(p, 'registry', side_effect=lambda repo, ref: records[(repo, ref)]), patch.object(p, 'latest', return_value={'changed': True} if failure == 'latest' else {}):
                    with self.assertRaises(RuntimeError):
                        p.verify_published(self.expected, {})

    def test_release_conflict_never_creates_tag(self):
        with patch.dict(os.environ, {'VERIFIED_INDEX_DIGEST': 'sha256:index'}), patch.object(p, 'registry', return_value={'digest': 'sha256:index'}), patch.object(p, 'github', return_value=b'{}'), patch.object(p.subprocess, 'run') as run:
            with self.assertRaisesRegex(RuntimeError, 'tag exists'):
                p.release()
            run.assert_not_called()

    def test_manual_dispatch_guard(self):
        with patch.dict(os.environ, {}, clear=True):
            with self.assertRaisesRegex(RuntimeError, 'manual RC dispatch'):
                p.main()


if __name__ == '__main__':
    unittest.main()

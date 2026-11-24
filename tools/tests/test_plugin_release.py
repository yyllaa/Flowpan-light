import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import zipfile

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('plugin_release', ROOT / 'tools/plugin_release.py')
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseChecks(unittest.TestCase):
    def test_interaction_menus_require_semantic_contract(self):
        manifest = {'definition': {'interaction': {'menus': [{'id': 'start', 'label': '开始', 'command': 'work'}]}}}
        self.assertEqual(release.required_features(manifest)['interaction.menus'], 1)
        self.assertNotIn('interaction.menus', release.required_features({'definition': {'interaction': {'commands': []}}}))

    def setUp(self):
        self.base = release.baseline()
        self.manifest = dict(id='fixture', version='1.0.0', runtime=dict(kind='wasm'),
                             permissions=['data.read'], release=dict(notes='Fixture release'),
                             definition=dict(id='fixture', version='1.0.0', permissions=['data.read']),
                             requires=dict(api_version='1', features={'manifest.requirements': 1}, tools=['kv.get']))
        self.report = dict(tools=['kv.get'], dynamic_calls=[], unknown_tools=[])

    def errors(self):
        return release.validate_manifest(self.manifest, self.base['capabilities'], self.report)

    def test_undeclared_call_blocks_release_even_when_permission_exists(self):
        self.manifest['requires']['tools'] = []
        self.assertIn('undeclared source tool: kv.get', self.errors())

    def test_missing_page_file_picker_and_continuation_semantics(self):
        self.manifest['definition'].update(pages=[dict(id='p')],
            actions=[dict(continuable=True)], fields=[dict(extensions={'flowpan.files': {}})])
        errors = self.errors()
        for name in ('ui.pages', 'ui.file-picker', 'jobs.continuation'):
            self.assertIn('missing required feature: ' + name, errors)

    def test_missing_required_tool_blocks_but_optional_tool_allows_degradation(self):
        self.manifest['requires']['tools'].append('future.query')
        self.assertIn('baseline lacks required tool: future.query', self.errors())
        self.manifest['requires']['tools'].remove('future.query')
        self.manifest['optional'] = dict(tools=['future.query'])
        self.report['tools'].append('future.query')
        self.report['unknown_tools'].append('future.query')
        self.assertEqual([], self.errors())

    def test_sdk_and_baseline_drift_are_rejected(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            for folder in ('app/pluginapi', 'sdk/pluginwasm'):
                (root / folder).mkdir(parents=True)
                (root / folder / 'fixture.go').write_text('package fixture\n')
            base = copy.deepcopy(self.base)
            base['sdk_sha256'] = release.sdk_digest(release.sdk_files(root))
            release.sdk_lock(root, root / 'SDK.lock.json', base)
            self.assertEqual(base['sdk_sha256'], release.check_sdk_lock(root, base)['sdk_sha256'])
            (root / 'sdk/pluginwasm/fixture.go').write_text('package changed\n')
            with self.assertRaisesRegex(ValueError, 'SDK differs'):
                release.check_sdk_lock(root, base)

    def test_storage_fields_and_invalid_feature_versions_are_checked(self):
        self.manifest['definition']['storage'] = dict(fields=[dict(options_source='versions')])
        self.assertIn('missing required feature: actions.dynamic-options', self.errors())
        self.manifest['requires']['features']['manifest.requirements'] = '1'
        self.assertIn('invalid required feature version: manifest.requirements', self.errors())

    def test_nested_modules_are_discovered_without_output_or_vendor(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            for folder in ('plugins/music', 'sdk/separate', 'plugins/vendor/ignored', 'output/fixture'):
                p = root / folder
                p.mkdir(parents=True)
                (p / 'go.mod').write_text('module fixture\n')
            self.assertEqual(['plugins/music', 'sdk/separate'], [p.relative_to(root).as_posix() for p in release.discover_modules(root)])

    def test_catalog_requires_runtime_receipt_and_uses_final_zip_manifest(self):
        with tempfile.TemporaryDirectory() as temp:
            folder = Path(temp)
            package = folder / 'plugin.zip'
            with zipfile.ZipFile(package, 'w') as z:
                z.writestr('manifest.json', json.dumps(self.manifest))
            verified = dict(schema_version=1, plugin_id='fixture', version='1.0.0',
                            package_sha256=release.sha(package), baseline=self.base['id'], image_id=self.base['image_id'],
                            sdk_sha256=self.base['sdk_sha256'], static_checked=True, runtime_verified=False)
            release.write_json(str(package) + '.verified.json', verified)
            args = type('Args', (), dict(package=package, baseline=release.BASELINE,
                url='https://example.com/plugin.zip', commit='b' * 40, out=folder / 'index.json'))()
            with self.assertRaisesRegex(ValueError, 'workflow verification'):
                release.catalog(args)
            verified['runtime_verified'] = True
            release.write_json(str(package) + '.verified.json', verified)
            release.catalog(args)
            entry = release.read_json(args.out)['plugins'][0]
            self.assertEqual('b' * 40, entry['package']['commit_sha'])
            entry.pop('package')
            self.assertEqual(self.manifest, entry)
            args.commit = None
            with self.assertRaisesRegex(ValueError, 'commit must be a full SHA'):
                release.catalog(args)
            args.commit = 'not-a-commit'
            with self.assertRaisesRegex(ValueError, 'commit must be a full SHA'):
                release.catalog(args)
            args.commit = 'b' * 40
            package.write_bytes(package.read_bytes() + b'changed')
            with self.assertRaisesRegex(ValueError, 'does not match'):
                release.catalog(args)

    def test_failed_workflow_never_creates_verified_receipt(self):
        with tempfile.TemporaryDirectory() as temp:
            package = Path(temp) / 'candidate.zip'
            package.write_bytes(b'candidate')
            release.write_json(str(package) + '.build.json', dict(package_sha256=release.sha(package),
                image_id=self.base['image_id'], sdk_sha256=self.base['sdk_sha256'], static_checked=True))
            release.write_json(str(package) + '.verified.json', dict(runtime_verified=True))
            args = type('Args', (), dict(package=package, baseline=release.BASELINE,
                workflow=[sys.executable, '-c', 'raise SystemExit(7)']))()
            with self.assertRaises(subprocess.CalledProcessError):
                release.verify(args)
            self.assertFalse(Path(str(package) + '.verified.json').exists())

    def test_every_export_mode_declares_calls_and_pins_sdk(self):
        modes = [[], ['--schedule'], ['--page'], ['--files'], ['--storage'], ['--interaction'],
                 ['--recognition'], ['--emby'], ['--capability-provider'], ['--capability-consumer']]
        with tempfile.TemporaryDirectory(prefix='flowpan-kit-test-') as temp:
            for i, mode in enumerate(modes):
                with self.subTest(mode=mode):
                    target = Path(temp) / str(i)
                    command = [sys.executable, str(ROOT / 'tools/plugin-kit.py'), '--out', str(target), '--id', 'fixture-' + str(i), *mode]
                    subprocess.run(command, cwd=ROOT, check=True, stdout=subprocess.PIPE)
                    manifest, report = release.check_plugin(target)
                    self.assertEqual(set(report['tools']), set(manifest['requires']['tools']))
                    self.assertEqual(self.base['sdk_sha256'], release.check_sdk_lock(target, self.base)['sdk_sha256'])
                    self.assertTrue((target / 'tools/plugin_release.py').is_file())
                    sdk = ROOT / 'sdk/plugins'
                    for name in ('COMPATIBILITY.md', 'CAPABILITIES.md', 'RELEASE.md', 'INTEGRATION.md', 'FILE-PICKER.md'):
                        self.assertEqual((sdk / name).read_bytes(), (target / name).read_bytes(), name)
                    if mode == ['--storage']:
                        self.assertIn('actions.dynamic-options', manifest['requires']['features'])
                    if mode == ['--page']:
                        self.assertEqual((sdk / 'web/assets/plugin-page-sdk.js').read_bytes(),
                                         (target / 'ui/flowpan-page-sdk.js').read_bytes())
                        self.assertEqual(manifest['definition']['pages'][0]['id'], 'home')
                        self.assertEqual([action['id'] for action in manifest['definition']['actions']], ['run'])
                        self.assertNotIn('media.process', manifest['permissions'])
                        self.assertIn('FlowpanPage.loadUI()', (target / 'ui/app.js').read_text(encoding='utf-8'))
                        # Use the exported checker itself outside the parent repository.
                        subprocess.run([sys.executable, str(target / 'tools/plugin_release.py'),
                            'check', '--plugin', str(target)], cwd=target, check=True, stdout=subprocess.PIPE)


if __name__ == '__main__':
    unittest.main()

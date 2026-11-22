"""Plugin release checks against an immutable host; no host source is built."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
SDK_ROOT = ROOT / 'sdk/plugins' if (ROOT / 'sdk/plugins').is_dir() else ROOT
BASELINE = SDK_ROOT / 'plugins/host-baseline.json'


def read_json(path):
    return json.loads(Path(path).read_text(encoding='utf-8-sig'))


def write_json(path, data):
    Path(path).write_text(json.dumps(data, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def run(argv, **kwargs):
    result = subprocess.run([str(v) for v in argv], check=True, **kwargs)
    return result


def sdk_files(root=SDK_ROOT):
    return {p.relative_to(root).as_posix(): sha(p)
            for folder in ('app/pluginapi', 'sdk/pluginwasm')
            for p in sorted((root / folder).glob('*.go')) if not p.name.endswith('_test.go')}


def sdk_digest(sources):
    return hashlib.sha256(json.dumps(sources, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def baseline(path=BASELINE):
    data = read_json(path)
    if data.get('schema_version') != 1 or not re.fullmatch(r'sha256:[0-9a-f]{64}', data.get('image_id', '')):
        raise ValueError('baseline must pin an immutable Docker image ID')
    if data.get('id') != data['capabilities']['contract']['host_version']:
        raise ValueError('baseline version differs from captured host contract')
    return data


def sdk_lock(root=SDK_ROOT, target=None, base=None):
    base = base or baseline()
    sources = sdk_files(root)
    if sdk_digest(sources) != base['sdk_sha256']:
        raise ValueError('SDK differs from the pinned release baseline; validate and version a new SDK baseline')
    data = dict(schema_version=1, baseline=base['id'], image_id=base['image_id'],
                sdk_sha256=base['sdk_sha256'], files=sources)
    if target:
        write_json(target, data)
    return data


def check_sdk_lock(root=SDK_ROOT, base=None):
    base = base or baseline()
    lock = root / 'SDK.lock.json'
    current = sdk_lock(root, base=base)
    if lock.is_file() and read_json(lock) != current:
        raise ValueError('SDK.lock.json does not match the exported SDK or baseline')
    return current


def source_report(directory, baseline_path=BASELINE, root=SDK_ROOT, go=None):
    source = directory / 'plugin' if (directory / 'plugin/main.go').is_file() else directory
    argv = [go or os.environ.get('FLOWPAN_GO', 'go'), 'run', str(root / 'tools/plugin-scan/main.go'),
            '-plugin', str(source), '-sdk', str(root / 'sdk/pluginwasm'), '-catalog', str(baseline_path)]
    return json.loads(run(argv, cwd=root, stdout=subprocess.PIPE, encoding='utf-8').stdout)


def required_features(manifest):
    definition = manifest.get('definition') or {}
    features = {'manifest.requirements': 1}
    if definition.get('pages'):
        features['ui.pages'] = 1
    if definition.get('provides') or definition.get('uses'):
        features['plugins.capabilities'] = 1
    if (definition.get('interaction') or {}).get('menus'):
        features['interaction.menus'] = 1
    if any(a.get('continuable') for a in definition.get('actions', [])):
        features['jobs.continuation'] = 1
    fields = list(definition.get('fields', []))
    fields += [f for a in definition.get('actions', []) for f in a.get('input_fields', [])]
    storage = definition.get('storage') or {}
    fields += storage.get('fields', [])
    fields += [f for group in (storage.get('task_fields') or {}).values() for f in group]
    if any(f.get('options_source') for f in fields):
        features['actions.dynamic-options'] = 1
    if any('flowpan.files' in (f.get('extensions') or {}) for f in fields):
        features['ui.file-picker'] = 1
    if 'files.publish' in manifest.get('requires', {}).get('tools', []):
        features['files.verified-publish'] = 1
    return features


def validate_manifest(manifest, capabilities, report):
    """Lint known calls; dynamic dispatch is explicitly reported, never certified."""
    if manifest.get('runtime', {}).get('kind') != 'wasm':
        return []
    errors = []
    definition = manifest.get('definition') or {}
    required = manifest.get('requires') or {}
    optional = manifest.get('optional') or {}
    tools = {t['name']: t for t in capabilities['tools']}
    contract = capabilities['contract']
    declared = set(required.get('tools', [])) | set(optional.get('tools', []))
    permissions = set(manifest.get('permissions', []))
    if required.get('api_version') != contract['api_version']:
        errors.append('requires.api_version must declare the baseline API version')
    for key in ('id', 'version'):
        if manifest.get(key) != definition.get(key):
            errors.append('definition.' + key + ' differs from manifest')
    if permissions != set(definition.get('permissions', [])):
        errors.append('definition permissions differ from manifest')
    if not re.fullmatch(r'v?\d+(?:\.\d+)*(?:-[\w.-]+)?(?:\+[\w.-]+)?', manifest.get('version', '')):
        errors.append('release version must support numeric comparison')
    if not manifest.get('release', {}).get('notes', '').strip():
        errors.append('release.notes is required')
    for name, version in required_features(manifest).items():
        actual = required.get('features', {}).get(name, 0)
        if not isinstance(actual, int) or isinstance(actual, bool) or actual < version:
            errors.append('missing required feature: ' + name)
    for name in report['unknown_tools']:
        if name not in optional.get('tools', []):
            errors.append('unknown SDK tool: ' + name)
    for name in report['tools']:
        if name not in declared:
            errors.append('undeclared source tool: ' + name)
    for name in sorted(declared):
        spec = tools.get(name)
        if spec is None:
            if name in required.get('tools', []):
                errors.append('baseline lacks required tool: ' + name)
            continue
        if spec['permission'] and spec['permission'] not in permissions:
            errors.append('missing permission for ' + name + ': ' + spec['permission'])
    for name, version in required.get('features', {}).items():
        if not isinstance(version, int) or isinstance(version, bool) or version < 1:
            errors.append('invalid required feature version: ' + name)
        elif contract['features'].get(name, 0) < version:
            errors.append('baseline lacks required feature: ' + name)
    for name, value in required.get('limits', {}).items():
        if contract['limits'].get(name, 0) < value:
            errors.append('baseline limit is insufficient: ' + name)
    return errors


def check_plugin(directory, baseline_path=BASELINE, go=None):
    directory = Path(directory).resolve()
    base = baseline(baseline_path)
    manifest = read_json(directory / 'manifest.json')
    if manifest['runtime']['kind'] != 'wasm':
        return manifest, dict(tools=[], dynamic_calls=[], unknown_tools=[])
    report = source_report(directory, baseline_path, go=go)
    errors = validate_manifest(manifest, base['capabilities'], report)
    if errors:
        raise ValueError(manifest['id'] + ': ' + '; '.join(errors))
    return manifest, report


def discover_modules(root=ROOT):
    return sorted({p.parent for folder in ('plugins', 'sdk') for p in (root / folder).rglob('go.mod')
                   if not any(part in {'vendor', 'node_modules', 'testdata', 'dist'} for part in p.relative_to(root).parts)})


class Host:
    def __init__(self, base, stage, executable=None, executable_sha256=None):
        self.base, self.stage = base, Path(stage).resolve()
        self.executable = Path(executable).resolve() if executable else None
        if self.executable:
            if not executable_sha256 or sha(self.executable) != executable_sha256:
                raise ValueError('--host requires its exact --host-sha256')
        captured = json.loads(self.call(['plugin', 'capabilities']).stdout)
        if captured != base['capabilities']:
            raise ValueError('host capability snapshot differs from the pinned baseline')

    def call(self, args):
        if self.executable:
            mapped = [str(self.stage / a.removeprefix('/release/')) if a.startswith('/release/') else a for a in args]
            argv = [self.executable, *mapped]
        else:
            argv = ['docker', 'run', '--rm', '--pull=never', '--network', 'none', '--platform', self.base['platform'],
                    '--mount', 'type=bind,source=' + str(self.stage) + ',target=/release',
                    '--entrypoint', 'flowpan', self.base['image_id'], *args]
        return run(argv, stdout=subprocess.PIPE, encoding='utf-8')


def build(args):
    directory, output = Path(args.plugin).resolve(), Path(args.out).resolve()
    if output.exists():
        raise ValueError('release output already exists; use a new release directory')
    base = baseline(args.baseline)
    lock = check_sdk_lock(base=base)
    manifest, report = check_plugin(directory, args.baseline, args.go)
    if manifest['runtime']['kind'] != 'wasm':
        raise ValueError('only external WASM plugins are built here')
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='plugin-candidate-', dir=output.parent) as tmp:
        stage = Path(tmp)
        shutil.copy2(directory / 'manifest.json', stage / 'manifest.json')
        if (directory / 'ui').is_dir():
            shutil.copytree(directory / 'ui', stage / 'ui')
        module = Path(manifest['runtime']['module'])
        if module.is_absolute() or '..' in module.parts:
            raise ValueError('module path must stay inside the package')
        (stage / module).parent.mkdir(parents=True, exist_ok=True)
        source = directory / 'plugin' if (directory / 'plugin/main.go').is_file() else directory
        env = dict(os.environ, GOOS='wasip1', GOARCH='wasm', CGO_ENABLED='0')
        run([args.go, 'build', '-trimpath', '-buildvcs=false', '-ldflags=-s -w', '-o', stage / module, '.'], cwd=source, env=env)
        host = Host(base, stage, args.host, args.host_sha256)
        host.call(['plugin', 'pack', '--release', '--dir', '/release', '--out', '/release/candidate.zip'])
        checked = json.loads(host.call(['plugin', 'check', '--release', '/release/candidate.zip']).stdout)
        if checked.get('executed') is not False or not checked['compatibility']['compatible']:
            raise ValueError('unexpected static package check result')
        with zipfile.ZipFile(stage / 'candidate.zip') as archive:
            if json.loads(archive.read('manifest.json'))['definition'] != manifest['definition']:
                raise ValueError('packed definition differs from source')
            if archive.testzip() is not None:
                raise ValueError('invalid ZIP')
        shutil.copyfile(stage / 'candidate.zip', output)
    receipt = dict(schema_version=1, plugin_id=manifest['id'], version=manifest['version'],
                   package_sha256=sha(output), baseline=base['id'], image_id=base['image_id'],
                   sdk_sha256=lock['sdk_sha256'], static_checked=True, runtime_verified=False,
                   source_calls=report)
    if args.host:
        receipt['host_sha256'] = args.host_sha256
    write_json(str(output) + '.build.json', receipt)
    print('PLUGIN_CANDIDATE_OK id=' + manifest['id'] + ' version=' + manifest['version'] + ' host_changes=0 runtime_verified=false')


def verify(args):
    package = Path(args.package).resolve()
    # A failed rerun must invalidate any earlier successful receipt.
    Path(str(package) + '.verified.json').unlink(missing_ok=True)
    receipt = read_json(str(package) + '.build.json')
    base = baseline(args.baseline)
    if (receipt['package_sha256'] != sha(package) or receipt['image_id'] != base['image_id']
            or receipt.get('sdk_sha256') != base['sdk_sha256'] or not receipt.get('static_checked')):
        raise ValueError('candidate changed or belongs to a different baseline')
    command = args.workflow
    if command and command[0] == '--':
        command = command[1:]
    if not command:
        raise ValueError('a real workflow test command is required; pack/check is not runtime verification')
    env = dict(os.environ, FLOWPAN_PLUGIN_ZIP=str(package), FLOWPAN_IMAGE_ID=base['image_id'],
               FLOWPAN_BASELINE=str(Path(args.baseline).resolve()))
    before = sha(package)
    run(command, cwd=ROOT, env=env)
    if sha(package) != before:
        raise ValueError('workflow changed the candidate package')
    receipt.update(runtime_verified=True, workflow_command=command)
    write_json(str(package) + '.verified.json', receipt)
    print('PLUGIN_WORKFLOW_OK sha256=' + before + ' image=' + base['image_id'])


def catalog(args):
    package = Path(args.package).resolve()
    base = baseline(args.baseline)
    receipt = read_json(str(package) + '.verified.json')
    if not receipt.get('runtime_verified') or not receipt.get('static_checked'):
        raise ValueError('workflow verification is required before catalog publication')
    if receipt['package_sha256'] != sha(package) or receipt['image_id'] != base['image_id'] or receipt['sdk_sha256'] != base['sdk_sha256']:
        raise ValueError('verification does not match this package, SDK and baseline')
    if not args.url.startswith('https://'):
        raise ValueError('catalog URL must use HTTPS')
    with zipfile.ZipFile(package) as archive:
        manifest = json.loads(archive.read('manifest.json'))
    if (manifest['id'], manifest['version']) != (receipt['plugin_id'], receipt['version']):
        raise ValueError('verification identity differs from package')
    manifest['package'] = dict(url=args.url, sha256=sha(package))
    if not isinstance(args.commit, str) or not re.fullmatch(r'[0-9a-f]{40}', args.commit):
        raise ValueError('commit must be a full SHA')
    manifest['package']['commit_sha'] = args.commit
    write_json(args.out, dict(schema_version=1, name='Flowpan verified plugins', plugins=[manifest]))
    print('PLUGIN_CATALOG_OK source=final_zip checksum=PASS')


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='command', required=True)
    for name in ('check', 'build', 'verify', 'catalog'):
        cmd = sub.add_parser(name)
        cmd.add_argument('--baseline', type=Path, default=BASELINE)
        if name in ('check', 'build'):
            cmd.add_argument('--plugin', type=Path, required=name == 'build')
            cmd.add_argument('--go', default=os.environ.get('FLOWPAN_GO', 'go'))
        else:
            cmd.add_argument('--package', required=True, type=Path)
        if name == 'check':
            cmd.add_argument('--all', action='store_true')
        if name == 'build':
            cmd.add_argument('--out', required=True, type=Path)
            cmd.add_argument('--host')
            cmd.add_argument('--host-sha256')
        if name == 'verify':
            cmd.add_argument('workflow', nargs=argparse.REMAINDER)
        if name == 'catalog':
            cmd.add_argument('--url', required=True)
            cmd.add_argument('--commit', required=True, help='full 40-character lowercase source commit SHA')
            cmd.add_argument('--out', required=True, type=Path)
    cmd = sub.add_parser('test-modules')
    cmd.add_argument('--go', default=os.environ.get('FLOWPAN_GO', 'go'))
    args = parser.parse_args(argv)
    if args.command == 'check':
        base = baseline(args.baseline)
        check_sdk_lock(base=base)
        paths = sorted((ROOT / 'plugins').rglob('manifest.json')) if args.all else [args.plugin]
        if not paths or any(p is None for p in paths):
            parser.error('select --plugin or --all')
        for path in paths:
            directory = path.parent if path.name == 'manifest.json' else path
            m, r = check_plugin(directory, args.baseline, args.go)
            print('PLUGIN_DECLARATIONS_OK id=' + m['id'] + ' calls=' + str(len(r['tools'])) + ' dynamic_review=' + str(len(r['dynamic_calls'])))
    elif args.command == 'test-modules':
        for module in discover_modules():
            print('PLUGIN_MODULE_TEST ' + module.relative_to(ROOT).as_posix(), flush=True)
            for action in ('build', 'vet', 'test'):
                run([args.go, action, './...'], cwd=module)
    else:
        globals()[args.command](args)


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, KeyError, subprocess.CalledProcessError, zipfile.BadZipFile) as exc:
        print('PLUGIN_RELEASE_REJECTED: ' + str(exc), file=sys.stderr)
        sys.exit(1)

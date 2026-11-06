"""Build the Dian plugin and standalone SDK from this repository."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import zipfile

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--out', type=Path, default=Path('dist/plugins'))
p.add_argument('--go', default='go')
p.add_argument('--host', help='optional Flowpan executable for package validation')
a = p.parse_args()
root = Path(__file__).resolve().parents[1]
out = a.out.resolve()
out.mkdir(parents=True, exist_ok=True)
sdk = root / 'sdk/plugins'
dian = root / 'plugins/dian'
manifest = json.loads((dian / 'manifest.json').read_text('utf-8'))
plugin = out / (manifest['id'] + '-' + manifest['version'] + '.zip')
sdkzip = out / 'Flowpan-Plugin-SDK.zip'
if plugin.exists() or sdkzip.exists():
    p.error('release already exists; choose a new output directory')


def entry(archive, name, content):
    info = zipfile.ZipInfo(name, (2026, 11, 6, 0, 0, 0))
    info.compress_type = zipfile.ZIP_DEFLATED
    info.external_attr = 0o100644 << 16
    archive.writestr(info, content)


env = os.environ.copy()
env.update(GOOS='wasip1', GOARCH='wasm', CGO_ENABLED='0', GOWORK='off')
with tempfile.TemporaryDirectory(prefix='flowpan-plugin-build-') as temporary:
    wasm = Path(temporary) / 'plugin.wasm'
    subprocess.run([a.go, 'build', '-trimpath', '-buildvcs=false', '-ldflags=-s -w', '-o', str(wasm), '.'], cwd=dian, env=env, check=True)
    module = wasm.read_bytes()
    if module[:8] != b'\0asm\x01\0\0\0':
        raise RuntimeError('invalid WASM output')
    with zipfile.ZipFile(plugin, 'w') as archive:
        entry(archive, 'manifest.json', json.dumps(manifest, ensure_ascii=False, indent=2).encode())
        entry(archive, manifest['runtime']['module'], module)
with zipfile.ZipFile(sdkzip, 'w') as archive:
    for path in sorted(sdk.rglob('*')):
        if path.is_file() and 'bin' not in path.relative_to(sdk).parts and '__pycache__' not in path.parts and path.name != 'plugin.zip':
            entry(archive, path.relative_to(sdk).as_posix(), path.read_bytes())
for path in (plugin, sdkzip):
    with zipfile.ZipFile(path) as archive:
        if archive.testzip() is not None:
            raise RuntimeError('ZIP verification failed')
    digest = hashlib.sha256(path.read_bytes()).hexdigest()
    (out / (path.name + '.sha256')).write_text(digest + '  ' + path.name + '\n', encoding='utf-8')
    print('BUILT', path.name, 'SHA256=' + digest)
if a.host:
    subprocess.run([str(Path(a.host).resolve()), 'plugin', 'check', str(plugin)], check=True)

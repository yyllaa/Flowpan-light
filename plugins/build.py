"""Build independently managed Flowpan WASI plugins and the public SDK."""
from pathlib import Path
import argparse
import hashlib
import json
import os
import subprocess
import tempfile
import zipfile

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--out', type=Path, default=Path('dist/plugins'))
p.add_argument('--go', default='go')
p.add_argument('--host', help='optional Flowpan executable for package validation')
p.add_argument('--plugin', action='append', help='plugin directory name; repeat to select several')
a = p.parse_args()
root = Path(__file__).resolve().parents[1]
out = a.out.resolve()
out.mkdir(parents=True, exist_ok=True)
sdk = root / 'sdk/plugins'
available = {d.name: d for d in (root/'plugins').iterdir() if d.is_dir() and (d/'manifest.json').is_file() and (d/'go.mod').is_file()}
selected = a.plugin or sorted(available)
if len(selected) != len(set(selected)) or any(name not in available for name in selected):
    p.error('select unique plugin directory names: ' + ', '.join(sorted(available)))

def entry(archive, name, content):
    info = zipfile.ZipInfo(name, (2026, 11, 13, 0, 0, 0))
    info.compress_type = zipfile.ZIP_DEFLATED
    info.external_attr = 0o100644 << 16
    archive.writestr(info, content)

artifacts = []
env = os.environ.copy()
env.update(GOOS='wasip1', GOARCH='wasm', CGO_ENABLED='0', GOWORK='off')
for name in selected:
    folder = available[name]
    manifest = json.loads((folder/'manifest.json').read_text(encoding='utf-8'))
    destination = out / (manifest['id']+'-'+manifest['version']+'.zip')
    if destination.exists():
        p.error('release already exists; select a new output directory: '+destination.name)
    with tempfile.TemporaryDirectory(prefix='flowpan-plugin-build-') as temporary:
        wasm = Path(temporary)/'plugin.wasm'
        subprocess.run([a.go,'build','-trimpath','-buildvcs=false','-ldflags=-s -w','-o',str(wasm),'.'],cwd=folder,env=env,check=True)
        module = wasm.read_bytes()
        if module[:8] != b'\0asm\x01\0\0\0':
            raise RuntimeError('invalid WASM output')
        with zipfile.ZipFile(destination,'w') as archive:
            entry(archive,'manifest.json',json.dumps(manifest,ensure_ascii=False,indent=2).encode('utf-8'))
            entry(archive,manifest['runtime']['module'],module)
    artifacts.append(destination)
    if a.host:
        subprocess.run([str(Path(a.host).resolve()),'plugin','check',str(destination)],check=True)

sdkzip = out/'Flowpan-Plugin-SDK.zip'
if sdkzip.exists():
    p.error('SDK release already exists; select a new output directory')
with zipfile.ZipFile(sdkzip,'w') as archive:
    for path in sorted(sdk.rglob('*')):
        relative = path.relative_to(sdk)
        if path.is_file() and 'bin' not in relative.parts and '__pycache__' not in relative.parts and path.name != 'plugin.zip':
            entry(archive,relative.as_posix(),path.read_bytes())
artifacts.append(sdkzip)
for path in artifacts:
    with zipfile.ZipFile(path) as archive:
        if archive.testzip() is not None:
            raise RuntimeError('ZIP verification failed')
    digest = hashlib.sha256(path.read_bytes()).hexdigest()
    (out/(path.name+'.sha256')).write_text(digest+'  '+path.name+'\n',encoding='utf-8')
    print('BUILT '+path.name+' SHA256='+digest)

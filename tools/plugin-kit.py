"""Export a standalone Go/WASI plugin project using the current source SDK."""
import argparse
import json
from pathlib import Path
import re
import shutil

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--out', required=True)
parser.add_argument('--id', default='external-example')
parser.add_argument('--name', default='外部插件示例')
args = parser.parse_args()
if not re.fullmatch(r'[a-z0-9][a-z0-9._-]{0,63}', args.id):
    parser.error('invalid plugin ID')
if args.id in {'market', 'tasks', 'contributions', 'resources', 'browser', 'capabilities'}:
    parser.error('reserved plugin ID')
root = Path(__file__).resolve().parents[1] / 'sdk/plugins'
target = Path(args.out).resolve()
target.mkdir(parents=True, exist_ok=False)
for relative in ('app/pluginapi', 'sdk/pluginwasm'):
    destination = target / relative
    destination.mkdir(parents=True)
    for source in sorted((root / relative).glob('*.go')):
        if relative == 'app/pluginapi' and source.name.endswith('_test.go'):
            continue
        shutil.copyfile(source, destination / source.name)
(target / 'plugin').mkdir()
shutil.copyfile(root / 'plugin/main.go', target / 'plugin/main.go')
(target / 'go.mod').write_text('module flowpan\n\ngo 1.25.0\n', encoding='utf-8')
permissions = ['host.read', 'task.progress', 'log.write', 'data.read', 'data.write']
definition = dict(schema_version=2, api_version='1', id=args.id, version='1.0.0', name=args.name, runtime='wasm', config_version=1, permissions=permissions,
                  fields=[dict(key='message', label='消息', type='text')],
                  actions=[dict(id='run', label='运行', kind='run', requires_enabled=True, cancellable=True)])
manifest = dict(schema_version=1, id=args.id, version='1.0.0', name=args.name, description='外部插件接入示例', category='示例', config_schema_version=1, data_schema_version=1,
                runtime=dict(kind='wasm', module='bin/plugin.wasm'), package=dict(local=True), permissions=permissions, definition=definition)
(target / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2)+'\n', encoding='utf-8')
(target / 'build.py').write_text('''import argparse, os, subprocess
from pathlib import Path
p=argparse.ArgumentParser(); p.add_argument('--host',required=True); p.add_argument('--go',default='go'); a=p.parse_args()
root=Path(__file__).resolve().parent; host=str(Path(a.host).resolve()); out=root/'plugin.zip'
if out.exists(): p.error('plugin.zip already exists; choose a new release directory or rename the previous archive')
(root/'bin').mkdir(exist_ok=True)
env=os.environ.copy(); env.update(GOOS='wasip1',GOARCH='wasm',CGO_ENABLED='0')
subprocess.run([a.go,'build','-trimpath','-buildvcs=false','-ldflags=-s -w','-o',str(root/'bin/plugin.wasm'),'./plugin'],cwd=root,env=env,check=True)
subprocess.run([host,'plugin','pack','--dir',str(root),'--out',str(out)],cwd=root,check=True)
subprocess.run([host,'plugin','check',str(out)],cwd=root,check=True)
''', encoding='utf-8')
(target / 'README.md').write_text('''# Flowpan 外部插件开发包

此目录可独立开发，SDK 与合同来自导出时的项目源码，仅依赖 Go 标准库。

1. 编辑 `plugin/main.go` 和 `manifest.json`，同步两层 ID、版本及权限。
2. 运行 `go test ./sdk/pluginwasm`。
3. 运行 `python build.py --host /实际路径/flowpan --go go`，Windows 使用 flowpan.exe。
4. 将生成的 plugin.zip 在「辅助插件 → 插件管理 → 已安装」导入，启用后运行。

生成的 ZIP 仅包含清单和 WASM，SDK 源码不会进入运行包。构建与校验不执行插件。
调用浏览器、FFmpeg、转存和整理时使用 SDK 的宿主工具，账号及目录授权在项目中管理。
SDK 的 Serve 分开处理 run/health/migrate。扩展数据结构时填写自己的 Migrate 处理函数。
外部插件不依赖项目管理员登录信息，不直接操作云盘，也不需要独立服务镜像。
''', encoding='utf-8')
print('PLUGIN_KIT_OK id='+args.id+' sdk=standalone output='+str(target))

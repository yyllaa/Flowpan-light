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
parser.add_argument('--provider-id', default='customdrive', help='native drive ID for --storage')
parser.add_argument('--storage', action='store_true', help='export a native storage provider adapter')
parser.add_argument('--schedule', action='store_true', help='include an opt-in daily schedule registration')
parser.add_argument('--proxy', choices=('direct', 'project', 'inherit'), default='direct', help='declare direct access or require the project HTTP proxy')
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
shutil.copyfile(root / ('plugins/examples/storage-http/main.go' if args.storage else 'plugins/examples/starter/main.go'), target / 'plugin/main.go')
(target / 'go.mod').write_text('module flowpan\n\ngo 1.25.0\n', encoding='utf-8')
permissions = ['host.read', 'task.progress', 'log.write', 'data.read', 'data.write']
definition = dict(schema_version=2, api_version='1', id=args.id, version='1.0.0', name=args.name, runtime='wasm', config_version=1, permissions=permissions, network=dict(proxy=args.proxy),
                  fields=[dict(key='message', label='消息', type='text')],
                  actions=[dict(id='run', label='运行', kind='run', requires_enabled=True, cancellable=True)])
if args.storage:
    if not re.fullmatch(r'[a-z][a-z0-9_-]{0,63}', args.provider_id) or args.provider_id in {'p115','p139'}:
        parser.error('invalid storage provider ID')
    if args.schedule:
        parser.error('--storage and --schedule use different starter handlers')
    permissions[:] = ['storage.provider', 'http.fetch', 'log.write']
    definition.update(fields=[], actions=[], http_hosts=['storage.example.com'], storage=dict(id=args.provider_id, name=args.name, login_methods=['token'], fields=[dict(key='endpoint',label='接口地址',type='url',required=True,hint='HTTPS 地址，需要同步声明 http_hosts'),dict(key='token',label='Token',type='secret',required=True)], capabilities=['list','stat','read','link','upload','mkdir','rename','delete','copy','move','refresh']))
if args.storage:
    definition['storage']['fields'].append(dict(key='client_version',label='客户端版本',type='select',default='v1',group='连接设置',options_source='client_versions',hint='驱动返回支持的接口版本；按目标网盘调整。'))
    definition['storage']['task_fields'] = dict(
        upload=[dict(key='parallel',label='上传并发数',type='number',default=0,group='网盘附加设置',hint='0 使用驱动默认值。')],
        strm=[dict(key='channel',label='访问线路',type='select',default='auto',group='网盘附加设置',options=[dict(value='auto',label='自动线路'),dict(value='cdn',label='加速线路')])])
if args.schedule:
    permissions.append('schedule')
    definition['fields'].extend([dict(key='scheduled', label='启用定时', type='boolean'), dict(key='cron', label='定时规则', type='cron', hint='0 0 8 * * *', visible_when=dict(field='scheduled', equals=True), required=True)])
    definition['schedules'] = [dict(id='daily', label='每日运行', action='run', cron_field='cron', enabled_field='scheduled')]
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

`definition.network.proxy` 声明网络策略：direct 为直连，project 必须使用项目设置中的 HTTP 代理，inherit 跟随项目设置。代理地址由宿主管理。导出时可用 --proxy project 选择需要代理的插件。

1. 编辑 `plugin/main.go` 和 `manifest.json`，同步两层 ID、版本及权限。
2. 运行 `go test ./sdk/pluginwasm`。
3. 运行 `python build.py --host /实际路径/flowpan --go go`，Windows 使用 flowpan.exe。
4. 将生成的 plugin.zip 在「辅助插件 → 插件管理 → 已安装」导入，启用后运行。

生成的 ZIP 仅包含清单和 WASM，SDK 源码不会进入运行包。构建与校验不执行插件。
调用浏览器、FFmpeg、转存和整理时使用 SDK 的宿主工具，账号及目录授权在项目中管理。
使用 --storage 导出网盘驱动模板：安装启用后在项目账号设置、文件浏览、STRM、上传、整理、挂载和有声书中使用。
网盘模板通过独立的 Storage 处理函数接收账号上下文；按目标网盘改写协议与 http_hosts。模板的 HTTPS JSON 接口只是示例，默认没有配置任何账号。
SDK 的 Serve 分开处理 run/health/migrate/storage。扩展数据结构时填写自己的 Migrate 处理函数。
外部插件不依赖项目管理员登录信息，也不需要独立服务镜像。资源插件通过项目任务操作网盘；网盘驱动插件实现自身声明的文件协议。
''', encoding='utf-8')
print('PLUGIN_KIT_OK id='+args.id+' sdk=standalone output='+str(target))

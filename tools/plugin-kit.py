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
parser.add_argument('--version', default='1.0.0', help='numeric release version')
parser.add_argument('--provider-id', default='customdrive', help='native drive ID for --storage')
parser.add_argument('--storage', action='store_true', help='export a native storage provider adapter')
parser.add_argument('--schedule', action='store_true', help='include an opt-in daily schedule registration')
parser.add_argument('--page', action='store_true', help='include a navigation page with its own frontend and a media example')
parser.add_argument('--files', action='store_true', help='include cloud staging, media processing, publication and recovery actions')
parser.add_argument('--recognition', action='store_true', help='export a recognition provider contract scaffold')
parser.add_argument('--emby', action='store_true', help='export a read-only Emby host contract scaffold')
parser.add_argument('--interaction', action='store_true', help='include bot commands, conversation replies, sessions, buttons and attachments')
parser.add_argument('--proxy', choices=('direct', 'project', 'inherit'), default='direct', help='declare direct access or require the project HTTP proxy')
args = parser.parse_args()
if not re.fullmatch(r'[a-z0-9][a-z0-9._-]{0,63}', args.id):
    parser.error('invalid plugin ID')
if args.id in {'market', 'tasks', 'contributions', 'resources', 'browser', 'capabilities', 'navigation'}:
    parser.error('reserved plugin ID')
if args.recognition and (args.emby or args.storage or args.page or args.files or args.interaction or args.schedule):
    parser.error('--recognition exports a standalone provider contract scaffold')
if args.emby and (args.storage or args.page or args.files or args.interaction or args.schedule):
    parser.error('--emby exports a standalone host contract scaffold')
if args.storage and args.page:
    parser.error('--page exports a service template; storage plugins can declare definition.pages manually')
if args.files and (args.storage or args.page):
    parser.error('--files exports a resource workflow; use it separately from --storage or --page')
if args.interaction and (args.storage or args.page or args.files or args.schedule):
    parser.error('--interaction exports a standalone interaction scaffold; other modes can declare the contract manually')
if not re.fullmatch(r'v?[0-9]+(?:\.[0-9]+)*(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?(?:\+[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?', args.version):
    parser.error('invalid release version')
repository = Path(__file__).resolve().parents[1]
root = repository / "sdk/plugins"
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
shutil.copyfile(root / ('plugins/examples/storage-http/main.go' if args.storage else 'plugins/examples/independent-page/main.go' if args.page else 'plugins/examples/starter/main.go'), target / 'plugin/main.go')
(target / 'go.mod').write_text('module flowpan\n\ngo 1.25.0\n', encoding='utf-8')
permissions = ['host.read', 'task.progress', 'log.write', 'data.read', 'data.write']
definition = dict(schema_version=2, api_version='1', id=args.id, version=args.version, name=args.name, runtime='wasm', config_version=1, permissions=permissions, network=dict(proxy=args.proxy),
                  fields=[dict(key='message', label='消息', type='text')],
                  actions=[dict(id='run', label='运行', kind='run', requires_enabled=True, cancellable=True)])
if args.recognition:
    shutil.copyfile(root / 'plugins/examples/recognition/main.go', target / 'plugin/main.go')
    definition['fields'] = []
    definition['actions'] = []
    definition['recognition'] = dict(categories=[dict(id='special',name='专用分类')], providers=[dict(id='catalog', name=args.name, extensions=['.mkv','.mp4','.strm'], keywords=['sample-key'], patterns=[r'(?i)\bNX-[0-9]+\b'], category='special')])
    shutil.copyfile(repository / 'docs/plugin-recognition.md', target / 'RECOGNITION.md')
if args.emby:
    shutil.copyfile(root / 'plugins/examples/emby-tools/main.go', target / 'plugin/main.go')
    permissions.extend(['emby.read'])
    definition['fields'] = []
    definition['actions'][0]['input_fields'] = [dict(key='instance_id', label='Emby 实例 ID', type='text'), dict(key='item_id', label='媒体条目 ID', type='text')]
    shutil.copyfile(repository / 'docs/plugin-emby.md', target / 'EMBY.md')
if args.page:
    shutil.copytree(root / 'plugins/examples/independent-page/ui', target / 'ui')
    shutil.copyfile(root / 'web/assets/plugin-page-sdk.js', target / 'ui/flowpan-page-sdk.js')
    permissions.extend(['media.read', 'media.process'])
    definition['pages'] = [dict(id='music', label=args.name, icon='file', description='音乐标签与封装工具', navigation=True, entry='ui/index.html')]
    definition['actions'].extend([
        dict(id='remux', label='音乐封装', kind='run', placement='tools', requires_enabled=True,
             input_fields=[dict(key=key,label=label,type='text',required=key in ('key','path','format'))
                           for key,label in [('key','任务键'),('path','文件路径'),('format','格式'),('title','标题'),('artist','艺术家'),('album','专辑')]]),
        dict(id='status', label='媒体进度', kind='probe', placement='tools', requires_enabled=True,
             input_fields=[dict(key='id',label='媒体任务 ID',type='text',required=True)])])
if args.interaction:
    shutil.copyfile(root / 'plugins/examples/interaction/main.go', target / 'plugin/main.go')
    permissions.extend(['interaction.receive','interaction.read','interaction.reply','interaction.session','interaction.attachments','artifacts.read','artifacts.write'])
    definition['interaction'] = dict(commands=[dict(name='interact',description='开始插件交互',action='run')],message_action='run',attachment_action='message')
    definition['actions'].extend([dict(id='message',label='接收输入',kind='run',requires_enabled=True),dict(id='done',label='结束交互',kind='run',requires_enabled=True)])
if args.files:
    shutil.copyfile(root / 'plugins/examples/file-pipeline/main.go', target / 'plugin/main.go')
    permissions.extend(['files.read', 'files.publish', 'media.read', 'media.process'])
    labels = dict(key='任务键', account='账号 ID', id='文件或任务 ID', parent_id='目标目录 ID', name='文件名称', file_task='原始暂存任务 ID', media_task='媒体任务 ID', format='输出格式', title='标题', artist='艺术家', album='专辑', conflict='同名处理')
    def action(action_id, label, kind, keys, optional=(), confirm=False):
        fields = [dict(key=key, label=labels[key], type='text', required=key not in optional) for key in keys]
        for field in fields:
            if field['key'] == 'conflict':
                field.update(type='select', default='keep_both', options=[dict(value='keep_both',label='另存副本'),dict(value='skip',label='同名跳过'),dict(value='replace',label='备份后替换')])
        value = dict(id=action_id, label=label, kind=kind, requires_enabled=True, cancellable=True, input_fields=fields)
        if confirm:
            value.update(danger=True, confirmation='确认回写或恢复所选云盘文件？')
        return value
    definition['actions'] = [
        action('run','检查能力','probe',['account']),
        action('stage','暂存云盘文件','run',['key','account','id']),
        action('probe','读取音乐标签','probe',['file_task']),
        action('process','写入音乐标签','run',['key','file_task','format','title','artist','album'],('title','artist','album')),
        action('publish','回写云盘','run',['key','account','parent_id','name','media_task','conflict','file_task'],('file_task',),True),
        action('status','文件任务进度','probe',['id']),
        action('media-status','媒体任务进度','probe',['id']),
        action('cancel','取消文件任务','run',['id']),
        action('restore','恢复原文件','run',['id','key'],confirm=True),
        action('release-file','释放暂存文件','run',['id']),
        action('release-media','释放媒体结果','run',['id']),
    ]
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
manifest = dict(schema_version=1, id=args.id, version=args.version, name=args.name, description='外部插件接入示例', category='示例', config_schema_version=1, data_schema_version=1,
                runtime=dict(kind='wasm', module='bin/plugin.wasm'), package=dict(local=True), permissions=permissions, definition=definition)
manifest['release'] = dict(title=args.name+' '+args.version, notes='首次发布。')
(target / 'CHANGELOG.md').write_text('# 更新日志\n\n## '+args.version+'\n\n首次发布。\n', encoding='utf-8')
(target / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2)+'\n', encoding='utf-8')
(target / 'build.py').write_text('''import argparse, os, subprocess
from pathlib import Path
p=argparse.ArgumentParser(); p.add_argument('--host',required=True); p.add_argument('--go',default='go'); a=p.parse_args()
root=Path(__file__).resolve().parent; host=str(Path(a.host).resolve()); out=root/'plugin.zip'
if out.exists(): p.error('plugin.zip already exists; choose a new release directory or rename the previous archive')
(root/'bin').mkdir(exist_ok=True)
env=os.environ.copy(); env.update(GOOS='wasip1',GOARCH='wasm',CGO_ENABLED='0')
subprocess.run([a.go,'build','-trimpath','-buildvcs=false','-ldflags=-s -w','-o',str(root/'bin/plugin.wasm'),'./plugin'],cwd=root,env=env,check=True)
subprocess.run([host,'plugin','pack','--release','--dir',str(root),'--out',str(out)],cwd=root,check=True)
subprocess.run([host,'plugin','check','--release',str(out)],cwd=root,check=True)
''', encoding='utf-8')
(target / 'README.md').write_text('''# Flowpan 外部插件开发包

此目录可独立开发，SDK 与合同来自导出时的项目源码，仅依赖 Go 标准库。

`definition.network.proxy` 声明网络策略：direct 为直连，project 必须使用项目设置中的 HTTP 代理，inherit 跟随项目设置。代理地址由宿主管理。导出时可用 --proxy project 选择需要代理的插件。

1. 编辑 `plugin/main.go` 和 `manifest.json`，同步两层 ID、版本及权限，并填写 release.notes 与 CHANGELOG.md。
2. 运行 `go test ./sdk/pluginwasm`。
3. 运行 `python build.py --host /实际路径/flowpan --go go`，Windows 使用 flowpan.exe。
4. 将生成的 plugin.zip 在「辅助插件 → 插件管理 → 已安装」导入，启用后运行。

发布版本使用数字版本号，支持 v 前缀、预发布及日期版本；新版本同时更新清单的两层 version、release.notes 与 CHANGELOG.md。build.py 自动运行 pack --release 和 check --release，缺少更新日志或版本不可比较时终止发布；普通 check 继续兼容旧包。
release 可声明 title、notes、published_at（RFC3339）和 url（HTTPS），notes 随安装包保存，本地导入和任意发布站点均可阅读。旧 GitHub 包继续自动定位 Release。
HTTP、SDK 和服务请求由宿主自动计入仪表盘；网盘提供者与账号从运行上下文绑定，不需要新增仪表盘页面或为品牌注册计数。
网盘返回 pluginapi.StorageError 时，SDK 自动保留业务码和 HTTP 状态，FUSE 使用实际提供者标签；只读与可写功能按 storage.capabilities 自动适配。

生成的 ZIP 包含清单、WASM 和可选的 ui/ 静态前端；宿主私有 SDK 不会进入运行包。构建与校验不执行插件。
使用 --page 导出独立页面示例，包含可分别维护的 HTML、CSS、ES 模块和页面桥接 SDK。
页面通过 FlowpanPage.connect() 调用自己声明的动作和任务；凭据与媒体工具保留在 WASM/宿主一侧。
音乐示例使用 media.submit 的 remux 操作保留编码并写入常用标签，输入路径必须处于授权目录，输出进入插件私有媒体目录。
使用 --files 导出云盘文件处理示例：files.stage → media.probe / media.submit(file_task) → files.publish，包含进度、取消、释放和恢复动作。每次动作只提交或查询一步，大文件通过宿主传输。
files.capabilities 返回账号支持的冲突策略。replace 为备份后替换，非原子操作；原文件保留为备份，可用 files.restore 恢复。attention 表示回执待核验，同一任务键不会自动重复写入。此示例填写标签，不包含音乐网站的匹配协议；FFmpeg/FFprobe 仍从项目依赖下载入口安装。
调用浏览器、FFmpeg、转存和整理时使用 SDK 的宿主工具，账号及目录授权在项目中管理。
SDK 同时包含通用运行基础：PutArtifact、ServiceUpload、CompleteAI、IdentifyMedia、PreviewOrganize/ApplyOrganize、CreateSTRM/InspectSTRM、CreateCAS、下载器与依赖、ShareOutput。先调用 Capabilities 核对实际宿主；按方法声明权限。独立整理只使用插件传入的文件清单，不修改项目原生整理或 STRM 配置。files.publish 的 media_task/artifact/file_task 三选一；使用返回的 result.id 定位实际输出。
使用 --emby 导出 Emby 实例与条目查询的只读宿主合同骨架；授权在插件管理中选择实例。EMBY.md 包含分页、元数据、图片、回执、事件和外部 API 合同。
使用 --interaction 导出通用机器人交互骨架。启用后发送 /plugins 查看入口；/plugin 插件ID 命令 始终定位当前插件，唯一命令也可直接用 /命令。SDK 提供 InteractionContext、ReplyInteraction、WaitInteraction、EndInteraction、StageInteractionAttachment、SendInteractionFile。附件为插件私有产物，可交给 media.probe/submit 的 artifact 输入；Telegram 使用原生按钮，企业微信使用 /reply 回调入口。会话按通道、用户、聊天和话题隔离，危险动作由宿主再次确认。合同见 docs/plugin-interaction.md。
使用 --storage 导出网盘驱动模板：安装启用后在项目账号设置、文件浏览、STRM、上传、整理、挂载和有声书中使用。
网盘模板通过独立的 Storage 处理函数接收账号上下文；按目标网盘改写协议与 http_hosts。模板的 HTTPS JSON 接口只是示例，默认没有配置任何账号。
使用 --recognition 导出新识别来源的合同骨架；实现 Recognition 处理函数，通过 RecognitionReply 返回身份或待确认状态。RECOGNITION.md 说明关键词自动接入原整理链路与分类目录绑定。
SDK 的 Serve 分开处理 run/health/migrate/storage/recognition。扩展数据结构时填写自己的 Migrate 处理函数。
外部插件不依赖项目管理员登录信息，也不需要独立服务镜像。资源插件通过项目任务操作网盘；网盘驱动插件实现自身声明的文件协议。
''', encoding='utf-8')
print('PLUGIN_KIT_OK id='+args.id+' sdk=standalone output='+str(target))

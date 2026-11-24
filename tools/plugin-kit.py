"""Export a standalone Go/WASI plugin project using the current source SDK."""
import argparse
import json
from pathlib import Path
import re
import shutil
from plugin_release import baseline, required_features, sdk_lock, source_report, write_json

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--out', required=True)
parser.add_argument('--baseline', type=Path, default=Path(__file__).resolve().parents[1]/'sdk/plugins/plugins/host-baseline.json')
parser.add_argument('--id', default='external-example')
parser.add_argument('--name', default='外部插件示例')
parser.add_argument('--version', default='1.0.0', help='numeric release version')
parser.add_argument('--provider-id', default='customdrive', help='native drive ID for --storage')
parser.add_argument('--storage', action='store_true', help='export a native storage provider adapter')
parser.add_argument('--schedule', action='store_true', help='include an opt-in daily schedule registration')
parser.add_argument('--page', action='store_true', help='include a navigation page with its own frontend')
parser.add_argument('--files', action='store_true', help='include cloud staging, media processing, publication and recovery actions')
parser.add_argument('--recognition', action='store_true', help='export a recognition provider contract scaffold')
parser.add_argument('--emby', action='store_true', help='export a read-only Emby host contract scaffold')
parser.add_argument('--interaction', action='store_true', help='include bot commands, conversation replies, sessions, buttons and attachments')
parser.add_argument('--proxy', choices=('direct', 'project', 'inherit'), default='direct', help='declare direct access or require the project HTTP proxy')
parser.add_argument('--capability-provider', action='store_true', help='export a versioned plugin capability provider')
parser.add_argument('--capability-consumer', action='store_true', help='export a plugin that calls a declared capability')
parser.add_argument('--interface', default='example.text.format', help='namespaced capability interface')
parser.add_argument('--interface-version', type=int, default=1, help='capability contract version, independent of package version')
args = parser.parse_args()
if args.capability_provider or args.capability_consumer:
    if args.capability_provider and args.capability_consumer or any((args.storage,args.page,args.files,args.recognition,args.emby,args.interaction,args.schedule)):
        parser.error('capability modes export independent scaffolds')
    if not re.fullmatch(r'[a-z0-9][a-z0-9._-]{0,63}', args.interface) or not 1 <= args.interface_version <= 1000000:
        parser.error('invalid capability interface or version')
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
root = Path(__file__).resolve().parents[1]
sdk = root / 'sdk/plugins'
base = baseline(args.baseline)
sdk_lock(sdk, base=base)
target = Path(args.out).resolve()
target.mkdir(parents=True, exist_ok=False)
for relative in ('app/pluginapi', 'sdk/pluginwasm'):
    destination = target / relative
    destination.mkdir(parents=True)
    for source in sorted((sdk / relative).glob('*.go')):
        if relative == 'app/pluginapi' and source.name.endswith('_test.go'):
            continue
        shutil.copyfile(source, destination / source.name)
for name in ('COMPATIBILITY.md', 'CAPABILITIES.md', 'RELEASE.md', 'INTEGRATION.md', 'FILE-PICKER.md'):
    shutil.copyfile(sdk / name, target / name)
(target / 'tools/plugin-scan').mkdir(parents=True)
for name in ('plugin_release.py', 'plugin-scan/main.go', 'plugin-scan/main_test.go'):
    shutil.copyfile(root / 'tools' / name, target / 'tools' / name)
(target / 'plugins').mkdir()
write_json(target / 'plugins/host-baseline.json', base)
sdk_lock(target, target / 'SDK.lock.json', base)
(target / 'plugin').mkdir()
shutil.copyfile(root / ('plugins/examples/storage-http/main.go' if args.storage else 'plugins/examples/starter/main.go'), target / 'plugin/main.go')
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
    shutil.copyfile(root / 'docs/plugin-recognition.md', target / 'RECOGNITION.md')
if args.emby:
    shutil.copyfile(root / 'plugins/examples/emby-tools/main.go', target / 'plugin/main.go')
    permissions.extend(['emby.read'])
    definition['fields'] = []
    definition['actions'][0]['input_fields'] = [dict(key='instance_id', label='Emby 实例 ID', type='text'), dict(key='item_id', label='媒体条目 ID', type='text')]
    shutil.copyfile(root / 'docs/plugin-emby.md', target / 'EMBY.md')
if args.page:
    shutil.copytree(root / 'plugins/examples/starter-page/ui', target / 'ui')
    shutil.copyfile(sdk / 'web/assets/plugin-page-sdk.js', target / 'ui/flowpan-page-sdk.js')
    definition['pages'] = [dict(id='home', label=args.name, icon='file', description='插件页面', navigation=True, entry='ui/index.html')]
if args.interaction:
    shutil.copyfile(root / 'plugins/examples/interaction/main.go', target / 'plugin/main.go')
    permissions.extend(['interaction.receive','interaction.read','interaction.reply','interaction.session','interaction.attachments','artifacts.read','artifacts.write'])
    definition['interaction'] = dict(commands=[dict(name='interact',description='开始插件交互',action='run')],menus=[dict(id='start',label='开始交互',command='interact',channels=['wecom','telegram'])],message_action='run',attachment_action='message')
    shutil.copyfile(root / 'docs/plugin-interaction.md', target / 'INTERACTION.md')
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
    definition.update(fields=[], actions=[], http_hosts=['storage.example.com'], storage=dict(id=args.provider_id, name=args.name, endpoint_field='endpoint', login_methods=['token'], fields=[dict(key='endpoint',label='接口地址',type='url',required=True,hint='当前账号的 HTTP/HTTPS 服务地址；访问限于该地址和路径前缀'),dict(key='token',label='Token',type='secret',required=True)], capabilities=['list','stat','read','link','upload','mkdir','rename','delete','copy','move','refresh']))
if args.storage:
    definition['storage']['fields'].append(dict(key='client_version',label='客户端版本',type='select',default='v1',group='连接设置',options_source='client_versions',hint='驱动返回支持的接口版本；按目标网盘调整。'))
    definition['storage']['task_fields'] = dict(
        upload=[dict(key='parallel',label='上传并发数',type='number',default=0,group='网盘附加设置',hint='0 使用驱动默认值。')],
        strm=[dict(key='channel',label='访问线路',type='select',default='auto',group='网盘附加设置',options=[dict(value='auto',label='自动线路'),dict(value='cdn',label='加速线路')])])
if args.schedule:
    permissions.append('schedule')
    definition['fields'].extend([dict(key='scheduled', label='启用定时', type='boolean'), dict(key='cron', label='定时规则', type='cron', hint='0 0 8 * * *', visible_when=dict(field='scheduled', equals=True), required=True)])
    definition['schedules'] = [dict(id='daily', label='每日运行', action='run', cron_field='cron', enabled_field='scheduled')]
if args.capability_provider or args.capability_consumer:
    mode = 'capability-provider' if args.capability_provider else 'capability-consumer'
    shutil.copyfile(root / ('plugins/examples/'+mode+'/main.go'), target / 'plugin/main.go')
    definition['fields'] = []
    definition['actions'][0]['input_fields'] = [dict(key='text',label='文本',type='text',required=True)]
    if args.capability_provider:
        definition['actions'][0].update(kind='probe',idempotent=True)
        schema = dict(type='object',properties=dict(text=dict(type='string')),required=['text'])
        definition['provides'] = [{'id':'format','interface':args.interface,'version':args.interface_version,'action':'run','input':schema,'output':schema}]
    else:
        permissions.append('capabilities.call')
        definition['uses'] = [{'id':'formatter','interface':args.interface,'version':args.interface_version}]
manifest = dict(schema_version=1, id=args.id, version=args.version, name=args.name, description='外部插件接入示例', category='示例', config_schema_version=1, data_schema_version=1,
                runtime=dict(kind='wasm', module='bin/plugin.wasm'), package=dict(local=True), permissions=permissions, definition=definition)
manifest['requires'] = dict(api_version=base['capabilities']['contract']['api_version'],
                            features=required_features(manifest), tools=source_report(target, args.baseline)['tools'])
if 'files.publish' in manifest['requires']['tools']:
    manifest['requires']['features']['files.verified-publish'] = 1
manifest['release'] = dict(title=args.name+' '+args.version, notes='首次发布。')
(target / 'CHANGELOG.md').write_text('# 更新日志\n\n## '+args.version+'\n\n首次发布。\n', encoding='utf-8')
(target / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2)+'\n', encoding='utf-8')
(target / 'build.py').write_text('''import argparse, sys
from pathlib import Path
root=Path(__file__).resolve().parent
sys.path.insert(0,str(root/'tools'))
from plugin_release import main
main(['build','--plugin',str(root),'--out',str(root/'plugin.zip'),*sys.argv[1:]])
''', encoding='utf-8')
shutil.copyfile(sdk / 'README.md', target / 'README.md')
print('PLUGIN_KIT_OK id='+args.id+' sdk=standalone output='+str(target))

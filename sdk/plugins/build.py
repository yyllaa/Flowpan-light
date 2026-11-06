import argparse, os, subprocess
from pathlib import Path
p=argparse.ArgumentParser(); p.add_argument('--host',required=True); p.add_argument('--go',default='go'); a=p.parse_args()
root=Path(__file__).resolve().parent; host=str(Path(a.host).resolve()); out=root/'plugin.zip'
if out.exists(): p.error('plugin.zip already exists; choose a new release directory or rename the previous archive')
(root/'bin').mkdir(exist_ok=True)
env=os.environ.copy(); env.update(GOOS='wasip1',GOARCH='wasm',CGO_ENABLED='0')
subprocess.run([a.go,'build','-trimpath','-buildvcs=false','-ldflags=-s -w','-o',str(root/'bin/plugin.wasm'),'./plugin'],cwd=root,env=env,check=True)
subprocess.run([host,'plugin','pack','--dir',str(root),'--out',str(out)],cwd=root,check=True)
subprocess.run([host,'plugin','check',str(out)],cwd=root,check=True)

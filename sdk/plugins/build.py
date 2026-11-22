import argparse, sys
from pathlib import Path
root=Path(__file__).resolve().parent
sys.path.insert(0,str(root/'tools'))
from plugin_release import main
main(['build','--plugin',str(root),'--out',str(root/'plugin.zip'),*sys.argv[1:]])

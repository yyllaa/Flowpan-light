from pathlib import Path
import sys
plugin = Path(__file__).resolve().parent
sys.path.insert(0, str(plugin.parents[1] / "tools"))
from plugin_release import main
main(["build", "--plugin", str(plugin), *sys.argv[1:]])

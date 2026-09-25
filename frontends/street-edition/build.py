#!/usr/bin/env python3
"""Build Street Edition from this repository. Python 3.11+ and go.mod's Go required."""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tomllib

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
OUT = HERE / 'dist'
GO = os.environ.get('KINGPIN_GO', 'go')
# The frontend must be reviewed before accepting a changed contract.
EXPECTED_PROTOCOL, EXPECTED_VIEW = 17, 12

def commit():
    """The commit the build stamps: git's, or where there is no .git (a
    Vercel checkout, #411) the host's VERCEL_GIT_COMMIT_SHA, or unknown."""
    try:
        return subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True,
                                       stderr=subprocess.DEVNULL).strip()
    except (OSError, subprocess.CalledProcessError):
        return os.environ.get('VERCEL_GIT_COMMIT_SHA', 'unknown')

def build():
    protocol = (ROOT / 'internal/protocol/protocol.go').read_text()
    view = (ROOT / 'internal/engine/view.go').read_text()
    actual = (int(re.search(r'const Version = (\d+)', protocol)[1]),
              int(re.search(r'const ViewVersion = (\d+)', view)[1]))
    if actual != (EXPECTED_PROTOCOL, EXPECTED_VIEW):
        raise SystemExit(f'Review the Street Edition frontend for protocol/view {actual} before updating its supported versions.')
    revision = commit()
    # Only clean this builder's generated output, never a caller-supplied path.
    if OUT.exists():
        shutil.rmtree(OUT)
    shutil.copytree(HERE / 'src', OUT)
    for name in ['session.js', 'police.js', 'alerts.js', 'lieutenants.js']:
        shutil.copyfile(ROOT / 'cmd/kingpin-web/web/js' / name, OUT / name)
    upgrades = tomllib.loads((ROOT / 'internal/content/upgrades.toml').read_text())
    crew = tomllib.loads((ROOT / 'internal/content/crew.toml').read_text())
    info = {'commit': revision, 'protocol': EXPECTED_PROTOCOL, 'view': EXPECTED_VIEW,
            'frontRoles': {f['id']: f['role'] for f in upgrades['front']},
            'traits': {k: v['says'] for k, v in crew['trait'].items()}}
    (OUT / 'engine-info.js').write_text('export const engineInfo = ' + json.dumps(info) + ';\n')
    subprocess.run([GO, 'build', '-buildvcs=false', '-o', str(OUT / 'assets/kingpin.wasm'), './cmd/kingpin-wasm'],
                   cwd=ROOT, env={**os.environ, 'GOOS': 'js', 'GOARCH': 'wasm'}, check=True)
    goroot = Path(subprocess.check_output([GO, 'env', 'GOROOT'], text=True).strip())
    shutil.copyfile(goroot / 'lib/wasm/wasm_exec.js', OUT / 'assets/wasm_exec.js')
    shutil.copyfile(goroot / 'LICENSE', OUT / 'assets/GO-LICENSE.txt')
    for name in ['app.js', 'index.html']:
        path = OUT / name
        path.write_text(path.read_text().replace('__BUILD_REVISION__', revision))
    print(f'Built {OUT} from {revision}')

if __name__ == '__main__':
    build()

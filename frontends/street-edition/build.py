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
EXPECTED_PROTOCOL, EXPECTED_VIEW = 28, 22

def commit():
    """The commit the build stamps: git's, or where there is no .git (a
    Vercel checkout, #411) the host's VERCEL_GIT_COMMIT_SHA, or unknown."""
    try:
        return subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True,
                                       stderr=subprocess.DEVNULL).strip()
    except (OSError, subprocess.CalledProcessError):
        return os.environ.get('VERCEL_GIT_COMMIT_SHA', 'unknown')

def go_license(goroot):
    """Go's LICENSE: at GOROOT in the official tarball and a downloaded
    toolchain, one level up in Homebrew's (GOROOT is its libexec)."""
    paths = [goroot / 'LICENSE']
    if goroot.name == 'libexec':
        paths.append(goroot.parent / 'LICENSE')
    for path in paths:
        if path.is_file():
            return path
    raise SystemExit(f'Go\'s LICENSE is not at {" or ".join(map(str, paths))}; '
                     'set KINGPIN_GO to a Go whose install ships it.')

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
    rivals = tomllib.loads((ROOT / 'internal/content/rivals.toml').read_text())
    heat = tomllib.loads((ROOT / 'internal/content/heat.toml').read_text())
    intel = tomllib.loads((ROOT / 'internal/content/intel.toml').read_text())
    laundering = tomllib.loads((ROOT / 'internal/content/laundering.toml').read_text())
    houses = tomllib.loads((ROOT / 'internal/content/houses.toml').read_text())
    assets = tomllib.loads((ROOT / 'internal/content/assets.toml').read_text())
    market = tomllib.loads((ROOT / 'internal/content/market.toml').read_text())
    cities = tomllib.loads((ROOT / 'internal/content/city.toml').read_text())
    law = tomllib.loads((ROOT / 'internal/content/law.toml').read_text())
    # The crew's confirms (#551) read three numbers no rule serves: a
    # blank investigation's loyalty, the enforcers a war needs, and the
    # skill under which a runner is sloppy. The cop dialog (#552) reads
    # a cop's price and how straight their word is at it. The walk away
    # and the war (#554) read the street's window going straight is
    # measured over, the crown's share and the war order's force. The houses
    # and assets (#581) read a move's heat and what each asset does. The
    # market pane's next product (#578) reads the ladder in the file's
    # order and the products a city's supplier will not sell. The
    # risk panel's pressure (#575) reads law.toml's [effects] cuts, and the
    # glossary the numbers the TUI's WORDS fill in from the file.
    info = {'commit': revision, 'protocol': EXPECTED_PROTOCOL, 'view': EXPECTED_VIEW,
            'frontRoles': {f['id']: f['role'] for f in upgrades['front']},
            'traits': {k: v['says'] for k, v in crew['trait'].items()},
            'investigateLoyalty': crew['informant']['investigate_loyalty'],
            'takenOutMuscle': rivals['endings']['taken_out_muscle'],
            'sloppySkill': heat['heat']['sloppy_skill'],
            'copPrice': intel['intel']['cop_price'],
            'copAccuracy': intel['intel']['cop_accuracy'],
            'streetWindow': laundering['businessman']['street_window'],
            'kingpinShare': rivals['endings']['kingpin_share'],
            'warForce': rivals['war']['dial'],
            'moveHeat': houses['houses']['move_heat'],
            'pressure': {'thresholdCut': law['effects']['pressure_threshold_cut'], 'capCut': law['effects']['pressure_cap_cut']},
            'glossary': {'retireHeat': laundering['offshore']['retire_heat'],
                         'pages': {r['level']: r.get('evidence', 0) for r in heat['response']},
                         'hitPages': heat['investigation']['evidence'],
                         'betrayShare': crew['lieutenant']['betray_share'],
                         'betrayCorners': crew['lieutenant']['betray_corners'],
                         'streetWindow': laundering['businessman']['street_window']},
            'assets': {a['id']: {k: a.get(k, 0) for k in ['effect', 'own_ratio', 'capacity_mul', 'lab_mul', 'lab_quality', 'lab_cost_mul']} for a in assets['asset']},
            'ladder': [{'id': p['id'], 'name': p['name'], 'unlock_cash': p.get('unlock_cash', 0)} for p in market['product']],
            'noSupply': {c['id']: [k for k, m in c.get('market', {}).items() if m.get('no_supply')] for c in cities['city']}}
    (OUT / 'engine-info.js').write_text('export const engineInfo = ' + json.dumps(info) + ';\n')
    subprocess.run([GO, 'build', '-buildvcs=false', '-o', str(OUT / 'assets/kingpin.wasm'), './cmd/kingpin-wasm'],
                   cwd=ROOT, env={**os.environ, 'GOOS': 'js', 'GOARCH': 'wasm'}, check=True)
    goroot = Path(subprocess.check_output([GO, 'env', 'GOROOT'], text=True).strip())
    shutil.copyfile(goroot / 'lib/wasm/wasm_exec.js', OUT / 'assets/wasm_exec.js')
    shutil.copyfile(go_license(goroot), OUT / 'assets/GO-LICENSE.txt')
    # Every module imports format.js (#589), so every module is stamped:
    # one URL per module, or a module would load twice.
    for path in [*OUT.glob('*.js'), OUT / 'index.html']:
        path.write_text(path.read_text().replace('__BUILD_REVISION__', revision))
    print(f'Built {OUT} from {revision}')

if __name__ == '__main__':
    build()

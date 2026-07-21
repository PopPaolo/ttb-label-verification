#!/usr/bin/env bash
# Regenerates the golden label set: renders each case's SVG sources to PNG
# (qlmanage — no external dependencies on macOS) and rebuilds golden-batch.zip
# with a manifest.json assembled from the per-case application.json files.
set -euo pipefail
cd "$(dirname "$0")"

for svg in */*.svg; do
  dir=$(dirname "$svg")
  base=$(basename "$svg" .svg)
  qlmanage -t -s 1000 -o "$dir" "$svg" >/dev/null 2>&1
  mv "$dir/$base.svg.png" "$dir/$base.png"
  echo "rendered $dir/$base.png"
done

python3 - <<'EOF'
import json, os, zipfile

cases = sorted(d for d in os.listdir('.') if os.path.isdir(d))
apps = []
with zipfile.ZipFile('golden-batch.zip', 'w', zipfile.ZIP_DEFLATED) as z:
    for case in cases:
        with open(os.path.join(case, 'application.json')) as f:
            app = json.load(f)
        images = []
        for png in sorted(p for p in os.listdir(case) if p.endswith('.png')):
            name = f'{case}-{png}'
            z.write(os.path.join(case, png), name)
            images.append(name)
        app['id'] = case
        app['images'] = images
        apps.append(app)
    z.writestr('manifest.json', json.dumps({'applications': apps}, indent=2))
print(f'golden-batch.zip: {len(apps)} applications')
EOF

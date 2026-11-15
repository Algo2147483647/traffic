"""Generate Map 1 radio facilities from JSON materials. No radio physics here."""
import copy, json, math
from pathlib import Path
folder=Path(__file__).resolve().parents[1]/'example'/'Map1'
p=json.loads((folder/'communications-generation.json').read_text(encoding='utf-8'))
d=p['deployment'];radius=d['radius_m'];origin=d['origin_m']
assert d['pattern']=='hex-pointy' and radius>0
spacing=math.sqrt(3)*radius;rows=1.5*radius
sites=[]
for row in range(math.floor((d['min_y_m']-radius-origin['y'])/rows),math.ceil((d['max_y_m']+radius-origin['y'])/rows)+1):
 y=origin['y']+rows*row
 if not d['min_y_m']-radius<=y<=d['max_y_m']+radius:continue
 for col in range(math.floor((d['min_x_m']-radius-origin['x'])/spacing-row/2),math.ceil((d['max_x_m']+radius-origin['x'])/spacing-row/2)+1):
  x=origin['x']+spacing*(col+row/2)
  if not d['min_x_m']-radius<=x<=d['max_x_m']+radius:continue
  id=f'BS{len(sites)+1:04}'
  sectors=[dict(copy.deepcopy(p['sector_template']),id=f'{id}-S{i+1}',azimuth_deg=az) for i,az in enumerate(p['sector_azimuths_deg'])]
  sites.append(dict(id=id,name=f'Hex {col}, {row}',position_m=dict(x=x,y=y,z=origin['z']),mast_height_m=p['mast_height_m'],coverage_radius_m=radius,sectors=sectors))
layout={key:p[key] for key in ('schema_version','coordinate_system','profile','deployment')};layout['sites']=sites
(folder/'communications.json').write_text(json.dumps(layout,indent=2)+'\n',encoding='utf-8')
print(f'{len(sites)} sites, {sum(len(s["sectors"]) for s in sites)} cells; radius {radius} m; nearest-site spacing {spacing:.3f} m')

"""Generate the reproducible layered four-cell interchange preset."""
import json
from interchanges.trumpet import build_trumpet
from interchanges.cloverleaf import build_cloverleaf, compile_flow_connections
from pathlib import Path
ROOT = Path(__file__).resolve().parents[1]
base = json.loads((ROOT/'example/Map2/road.json').read_text(encoding='utf-8'))
layout=json.loads((ROOT/'example/Map2/generation.json').read_text())
city = {k:v for k,v in base.items() if k not in ('intersections','ways')}
city.update(intersections=[],ways=[],interchanges=[])
nodes={}
def node(id,x,y,layer=0,group='',flow=False):
    n=dict(id=id,name=id.replace('_',' '),position=dict(x=x,y=y),layer=layer,elevation=profile['cloverleaf']['deck_height']*layer,interchange=group)
    if flow:n['traffic_control']={'mode':'flow'}
    city['intersections'].append(n);nodes[id]=n;return id
def way(a,b,lanes=3,structure='ground',points=None):
    w=dict(id=a+'__'+b,name=a+' to '+b,**{'from':a,'to':b},lanes=lanes,lane_width=profile['cloverleaf']['lane_width'],speed_limit=(55 if structure=='ramp' else 100)/3.6,road_class='ramp' if structure=='ramp' else 'arterial',structure=structure)
    if points:w['points']=[nodes[a]['position']]+[dict(x=x,y=y) for x,y in points]+[nodes[b]['position']]
    city['ways'].append(w)
def pair(a,b,lanes=3):way(a,b,lanes);way(b,a,lanes)
import math
profile=json.loads((ROOT/'example/Map2/interchange-design.json').read_text(encoding='utf-8-sig'))
groups={'C':build_cloverleaf(city,nodes,node,way,profile['cloverleaf'],center=layout['center'])}
for item in layout['trumpets']:groups[item['id']]=build_trumpet(city,nodes,node,way,item['id'],item['center'],item['angle'],profile['trumpet'])
for label,(x,y) in layout['corners'].items():node(label,x,y)
def port(ref):
    if '.' not in ref:return(ref,ref)
    group,arm=ref.split('.');return groups[group][arm]
for a,b in layout['links']:
    a,b=port(a),port(b);way(a[1],b[0]);way(b[1],a[0])
compile_flow_connections(city)
city['description']='Four-cell network with a central C/D cloverleaf and four peripheral trumpet interchanges.'
city['world_units_per_km']=profile['world_units_per_km']
out=ROOT/'example/Map2/road.json'
for road in city['ways']:
    road['speed_limit'] = min(road['speed_limit'], 60/3.6)
out.write_text(json.dumps(city,indent=2)+'\n',encoding='utf-8')
print(f'{out}: {len(nodes)} nodes, {len(city["ways"])} directed ways')

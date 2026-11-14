"""Generate the reproducible layered four-cell interchange preset."""
import json
from interchanges.trumpet import build_trumpet
from interchanges.cloverleaf import build_cloverleaf, compile_flow_connections
from pathlib import Path
ROOT = Path(__file__).resolve().parents[1]
base = json.loads((ROOT/'configs/meridian-city.json').read_text(encoding='utf-8'))
city = {k:v for k,v in base.items() if k not in ('intersections','ways')}
city.update(name='Four-Cell Interchange Network', description='Central diamond and four three-arm folded diamonds; ramps connect separate bridge and ground nodes.', bounds=dict(min_x=0,min_y=0,max_x=6000,max_y=6000), intersections=[], ways=[])
city['simulation'] = dict(base['simulation'], people=320, intersection_radius=20, max_people=10000)
nodes={}
def node(id,x,y,layer=0,group='',flow=False):
    n=dict(id=id,name=id.replace('_',' '),position=dict(x=x,y=y),layer=layer,elevation=24*layer,interchange=group)
    if flow:n['traffic_control']={'mode':'flow'}
    city['intersections'].append(n);nodes[id]=n;return id
def way(a,b,lanes=3,structure='ground',points=None):
    w=dict(id=a+'__'+b,name=a+' to '+b,**{'from':a,'to':b},lanes=lanes,lane_width=8,speed_limit=55 if structure=='ramp' else 100,road_class='ramp' if structure=='ramp' else 'arterial',structure=structure)
    if points:w['points']=[nodes[a]['position']]+[dict(x=x,y=y) for x,y in points]+[nodes[b]['position']]
    city['ways'].append(w)
def pair(a,b,lanes=3):way(a,b,lanes);way(b,a,lanes)
import math
profile=json.loads((ROOT/'configs/interchange-design.json').read_text(encoding='utf-8-sig'))
C=build_cloverleaf(city,nodes,node,way,profile['cloverleaf'])
N=build_trumpet(city,nodes,node,way,'N',(3000,600),0,profile['trumpet'])
S=build_trumpet(city,nodes,node,way,'S',(3000,5400),math.pi,profile['trumpet'])
W=build_trumpet(city,nodes,node,way,'W',(600,3000),-math.pi/2,profile['trumpet'])
E=build_trumpet(city,nodes,node,way,'E',(5400,3000),math.pi/2,profile['trumpet'])
for label,x,y in [('NW',600,600),('NE',5400,600),('SW',600,5400),('SE',5400,5400)]:node(label,x,y)
def connect(a,b):way(a[1],b[0]);way(b[1],a[0])
def corner(id):return(id,id)
connect(corner('NW'),N['W']);connect(N['E'],corner('NE'))
connect(corner('SW'),S['E']);connect(S['W'],corner('SE'))
connect(corner('SW'),W['W']);connect(W['E'],corner('NW'))
connect(corner('NE'),E['W']);connect(E['E'],corner('SE'))
connect(N['S'],C['N']);connect(C['S'],S['S'])
connect(W['S'],C['W']);connect(C['E'],E['S'])
compile_flow_connections(city)
city['description']='Four-cell network with a central C/D cloverleaf and four peripheral trumpet interchanges.'
city['world_units_per_km']=1000
out=ROOT/'configs/presets/four-cell-interchanges.json'
out.write_text(json.dumps(city,indent=2)+'\n',encoding='utf-8')
print(f'{out}: {len(nodes)} nodes, {len(city["ways"])} directed ways')

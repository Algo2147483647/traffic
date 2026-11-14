"""Build Map 3 from its JSON materials; all distances are metres."""
import copy, json, math
from pathlib import Path
from interchanges.cloverleaf import build_cloverleaf, compile_flow_connections
ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/'example/Map3'
p=json.loads((OUT/'generation.json').read_text())
base=json.loads((OUT/p['source']/'road.json').read_text())
city={k:copy.deepcopy(v) for k,v in base.items() if k not in ('intersections','ways','interchanges')}
city.update(name='Map 3 - Radial Grid',description='Central cloverleaf, concentric arterial rings and an axis-connected local grid.',intersections=[],ways=[],interchanges=[],bounds=p["bounds"])
cx,cy=p['center'];nodes={}
profile=json.loads((OUT/p['interchange_profile']).read_text())['cloverleaf']
def node(id,x,y,layer=0,group='',flow=False):
    n=dict(id=id,name=id.replace('_',' '),position=dict(x=x,y=y),layer=layer,elevation=profile['deck_height']*layer,interchange=group)
    if flow:n['traffic_control']={'mode':'flow'}
    nodes[id]=n;city['intersections'].append(n);return id
def way(a,b,lanes=1,structure='ground',points=None,kind='arterial'):
    w=dict(id=a+'__'+b,name=a+' to '+b,**{'from':a,'to':b},lanes=lanes,lane_width=p['lane_width_m'],speed_limit=p['speed_limit_kmh']/3.6,road_class=kind,structure=structure)
    if points:w['points']=[nodes[a]['position']]+[dict(x=x,y=y) for x,y in points]+[nodes[b]['position']]
    city['ways'].append(w)
def pair(a,b,lanes=1,points=None,kind='arterial'):
    way(a,b,lanes,points=points,kind=kind);way(b,a,lanes,points=list(reversed(points)) if points else None,kind=kind)
ports=build_cloverleaf(city,nodes,node,way,profile,p['center'])
# Axis nodes are shared with the grid and each circular road, never duplicated.
axis={};spacing=p['grid_spacing_m']
for label,(dx,dy) in {'E':(1,0),'S':(0,1),'W':(-1,0),'N':(0,-1)}.items():
    previous=None
    for r in range(math.ceil(profile["portal_station"]/spacing)*spacing,p['road_outer_radius_m']+1,spacing):
        id=node(f'{label}_{r}',cx+dx*r,cy+dy*r);axis[(dx*r,dy*r)]=id
        if previous:pair(previous,id,p['axis_lanes_per_direction'])
        else:way(ports[label][1],id,p['axis_lanes_per_direction']);way(id,ports[label][0],p['axis_lanes_per_direction'])
        previous=id
for radius,key in [(p['road_inner_radius_m'],'inner_lanes_per_direction'),(p['road_outer_radius_m'],'outer_lanes_per_direction')]:
    arms=['E','S','W','N']
    for i,a in enumerate(arms):
        count=math.ceil(math.pi*radius/2/p['sample_spacing_m'])
        pts=[(cx+radius*math.cos((i+j/count)*math.pi/2),cy+radius*math.sin((i+j/count)*math.pi/2)) for j in range(1,count)]
        pair(f'{a}_{radius}',f'{arms[(i+1)%4]}_{radius}',p[key],pts)
# Keep only grid edges inside the radius, outside the interchange safety envelope.
grid={};R=p['grid_radius_m'];clear=p['grid_clearance_radius_m']
for x in range(-R,R+1,spacing):
    for y in range(-R,R+1,spacing):
        if clear<=math.hypot(x,y)<R:
            grid[(x,y)]=axis.get((x,y),f'G_{x}_{y}')
edges=set()
for a in grid:
    for b in [(a[0]+spacing,a[1]),(a[0],a[1]+spacing)]:
        if b in grid:edges.add((a,b))
while True:
    degree={a:0 for a in grid}
    for a,b in edges:degree[a]+=1;degree[b]+=1
    dead={a for a,n in degree.items() if n<2 and a not in axis}
    if not dead:break
    edges={e for e in edges if not any(a in dead for a in e)}
    for a in dead:del grid[a]
for (x,y),id in grid.items():
    if id not in nodes:node(id,cx+x,cy+y)
for a,b in sorted(edges):
    if a in axis and b in axis:continue
    pair(grid[a],grid[b],p['grid_lanes_per_direction'],kind='local')
compile_flow_connections(city)
for w in city['ways']:w['speed_limit']=min(w['speed_limit'],p['speed_limit_kmh']/3.6)
(OUT/'road.json').write_text(json.dumps(city,indent=2)+'\n')
# Preserve the linear circuits but scale their longitudinal dimension and relocate origin.
metro=json.loads((OUT/p['source']/'metro.json').read_text())
def chain(points):
    out=[0.0]
    for a,b in zip(points,points[1:]):out.append(out[-1]+math.hypot(b['x']-a['x'],b['y']-a['y']))
    return out
def locate(points,c,target):
    best=(float('inf'),0)
    for i,(a,b) in enumerate(zip(points,points[1:])):
        dx,dy=b['x']-a['x'],b['y']-a['y'];den=dx*dx+dy*dy
        t=max(0,min(1,((target[0]-a['x'])*dx+(target[1]-a['y'])*dy)/den))
        dist=math.hypot(a['x']+t*dx-target[0],a['y']+t*dy-target[1])
        if dist<best[0]:best=(dist,c[i]+t*math.sqrt(den))
    return best[1] % c[-1]
for line in metro['lines']:
    old=line['stations'];ox,oy=old[len(old)//2]['x'],old[len(old)//2]['y']
    dx,dy=old[-1]['x']-ox,old[-1]['y']-oy;length=math.hypot(dx,dy);ux,uy=dx/length,dy/length
    scale=p['metro_axis_radius_m']/length
    for q in line['route']:
        x,y=q['x']-ox,q['y']-oy;along=(x*ux+y*uy)*scale;side=-x*uy+y*ux
        q.update(x=cx+along*ux-side*uy,y=cy+along*uy+side*ux)
    stations=[];stops=[];c=chain(line['route']);half=metro['track_spacing_m']/2
    for r in range(-p['metro_axis_radius_m'],p['metro_axis_radius_m']+1,p['metro_station_spacing_m']):
        x,y=cx+r*ux,cy+r*uy;stations.append(dict(x=x,y=y))
        stops.append([locate(line['route'],c,(x-uy*half,y+ux*half)),locate(line['route'],c,(x+uy*half,y-ux*half))])
    line.update(stations=stations,stop_chainages=stops,crossing_station=len(stations)//2)
R=p['metro_ring_radius_m'];half=metro['track_spacing_m']/2
count=math.ceil(2*math.pi*R/p['sample_spacing_m']);count=math.ceil(count/6)*6
routes=[];chains=[]
for direction,radius in [(1,R+half),(-1,R-half)]:
    pts=[dict(x=cx+radius*math.cos(direction*2*math.pi*i/count),y=cy+radius*math.sin(direction*2*math.pi*i/count)) for i in range(count+1)]
    routes.append(pts);chains.append(chain(pts))
ring=dict(id=3,name='Orbital Line',layer=p['metro_ring_layer'],circular=True,stations=[dict(x=cx+R*math.cos(i*math.pi/3),y=cy+R*math.sin(i*math.pi/3)) for i in range(6)],route=routes[0],reverse_route=routes[1],stop_chainages=[[chains[0][i*count//6],chains[1][((6-i)%6)*count//6]] for i in range(6)],crossing_station=-1,loop_speed_kmh=300)
metro['lines'].append(ring)
(OUT/'metro.json').write_text(json.dumps(metro,indent=2)+'\n')
print(f'Map 3: {len(nodes)} nodes, {len(city["ways"])} directed roads, {len(metro["lines"])} metro lines')

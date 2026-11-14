"""Full two-level cloverleaf with four independent collector-distributor roads."""
import math
from .geometry import arc,cubic,rotate,validate_profile

def build_cloverleaf(city,nodes,node,way,profile,center=(3000,3000)):
    p=profile;validate_profile(p)
    offset=p['cd_offset'];r=p['loop_radius'];rr=p['right_radius'];space=p['sample_spacing'];dirs=[];ports={}
    arrivals=['W','N','E','S'];departures=['E','S','W','N']
    def add_way(a,b,lanes,role,structure,speed,points=None):
        way(a,b,lanes,structure,points)
        w=city['ways'][-1];w.update(role=role,speed_limit=speed,geometry='sampled',trip_access=False)
        return w['id']
    for k in range(4):
        angle=k*math.pi/2;level=1 if k%2==0 else 0
        def n(tag,x,y,layer=level):
            id=f'C_D{k}_{tag}';node(id,*rotate((x,y),angle,center),layer,'C',True);return id
        a=n('in',-p['portal_station'],p['mainline_offset'],0)
        b=n('out',p['portal_station'],p['mainline_offset'],0)
        split=n('split',-p['split_station'],p['mainline_offset']);merge=n('merge',p['split_station'],p['mainline_offset'])
        stations=[-p['cd_station'],-rr-offset,-r-offset,r+offset,rr+offset,p['cd_station']]
        cd=[n(tag,x,offset) for tag,x in zip(['cd_in','right_exit','loop_merge','loop_exit','right_merge','cd_out'],stations)]
        structure='bridge' if level else 'ground'
        approach=add_way(a,split,3,'mainline','ramp' if level else 'ground',p['mainline_speed'])
        bypass=add_way(split,merge,3,'mainline',structure,p['mainline_speed'])
        departure=add_way(merge,b,3,'mainline','ramp' if level else 'ground',p['mainline_speed'])
        entrycurve=cubic((-p['split_station'],p['mainline_offset']),(-p['split_station']+70,p['mainline_offset']),(-p['cd_station']-70,offset),(-p['cd_station'],offset),space)
        exitcurve=cubic((p['cd_station'],offset),(p['cd_station']+70,offset),(p['split_station']-70,p['mainline_offset']),(p['split_station'],p['mainline_offset']),space)
        entry=add_way(split,cd[0],2,'cd_transition',structure,p['cd_speed'],[rotate(v,angle,center) for v in entrycurve[1:-1]])
        segments=[add_way(cd[i],cd[i+1],2,'collector_distributor',structure,p['cd_speed']) for i in range(5)]
        exit=add_way(cd[-1],merge,2,'cd_transition',structure,p['cd_speed'],[rotate(v,angle,center) for v in exitcurve[1:-1]])
        dirs.append(dict(a=a,b=b,split=split,merge=merge,cd=cd,approach=approach,bypass=bypass,departure=departure,entry=entry,segments=segments,exit=exit))
        ports.setdefault(arrivals[k],[None,None])[0]=a;ports.setdefault(departures[k],[None,None])[1]=b
    routes=[]
    for k,d in enumerate(dirs):
        angle=k*math.pi/2;left=dirs[(k-1)%4];right=dirs[(k+1)%4]
        looppts=arc((r+offset,r+offset),r,-math.pi/2,1.5*math.pi,space)
        loop=add_way(d['cd'][3],left['cd'][2],1,'loop_ramp','ramp',p['loop_speed'],[rotate(v,angle,center) for v in looppts[1:-1]])
        rightpts=arc((-rr-offset,rr+offset),rr,-math.pi/2,math.pi/2,space)
        direct=add_way(d['cd'][1],right['cd'][4],1,'right_ramp','ramp',p['right_speed'],[rotate(v,angle,center) for v in rightpts[1:-1]])
        for turn,target,path in [
          ('through',k,[d['approach'],d['bypass'],d['departure']]),
          ('left',(k-1)%4,[d['approach'],d['entry'],*d['segments'][:3],loop,*left['segments'][2:],left['exit'],left['departure']]),
          ('right',(k+1)%4,[d['approach'],d['entry'],d['segments'][0],direct,right['segments'][4],right['exit'],right['departure']])]:
            routes.append(dict(origin=arrivals[k],destination=departures[target],turn=turn,way_ids=path))
    city.setdefault('interchanges',[]).append(dict(id='C',name='Central cloverleaf',type='cloverleaf',center=dict(x=center[0],y=center[1]),routes=routes))
    return {k:tuple(v) for k,v in ports.items()}

def compile_flow_connections(city):
    """Explicit lane continuity, with outer-lane exits and merges."""
    for n in city['intersections']:
        if not n.get('interchange'):continue
        ins=[w for w in city['ways'] if w['to']==n['id']];outs=[w for w in city['ways'] if w['from']==n['id']]
        connections=[]
        for a in ins:
            for b in outs:
                if len(outs)>1 and b.get('role') in ('loop_ramp','right_ramp','semi_direct_ramp','cd_transition'):
                    pairs=[(a['lanes']-1,b['lanes']-1)]
                elif len(ins)>1 and a.get('role') in ('loop_ramp','right_ramp','semi_direct_ramp','cd_transition'):
                    pairs=[(i,b['lanes']-1) for i in range(a['lanes'])]
                elif len(outs)>1 and a.get('role')=='branch' and b.get('role')=='branch':pairs=[(0,0)]
                else:pairs=[(i,min(i,b['lanes']-1)) for i in range(a['lanes'])]
                connections.append(dict(from_way_id=a['id'],to_way_id=b['id'],from_lanes=sorted({i for i,j in pairs}),to_lanes=sorted({j for i,j in pairs}),lane_connections=[dict(from_lane=i,to_lane=j) for i,j in pairs]))
        n['traffic_control']={'mode':'flow','connections':connections}

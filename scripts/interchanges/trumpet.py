"""Rotatable three-arm trumpet: one loop, one semi-direct and two right ramps."""
import math
from .geometry import arc,cubic,rotate

def build_trumpet(city,nodes,node,way,prefix,center,angle,profile):
    r=profile['loop_radius'];offset=profile['carriageway_offset'];spacing=profile['sample_spacing'];split=profile['mainline_station'];port=profile['mainline_portal'];branch=profile['branch_portal'];station=profile['branch_station']
    if r<=0 or spacing<=0 or not port>split>r+offset or not branch>station>profile['bridge_start']:raise ValueError('invalid trumpet profile')
    xy=lambda p:rotate(p,angle,center)
    def n(tag,x,y,layer=0):return node(prefix+'_'+tag,*xy((x,y)),layer,prefix,True)
    wi=n('W_in',-port,offset);wo=n('W_out',-port,-offset);ei=n('E_in',port,-offset);eo=n('E_out',port,offset)
    si=n('S_in',offset,branch);so=n('S_out',-offset,branch)
    ws=n('W_split',-split,offset);em=n('E_merge',split,offset);es=n('Loop_split',-r-offset,-offset);wm=n('W_merge',-split,-offset)
    ss=n('S_split',offset,station);sm=n('S_merge',-offset,station)
    nb=n('North_bridge_start',offset,profile['bridge_start'],1);ne=n('North_bridge_end',offset,profile['bridge_end'],1)
    sb=n('South_bridge_start',-offset,-r-offset,1);se=n('South_bridge_end',-offset,profile['bridge_start'],1)
    def road(a,b,lanes,role,structure='ground',points=None):
        way(a,b,lanes,structure,[xy(q) for q in points[1:-1]] if points else None)
        w=city['ways'][-1];w.update(role=role,geometry='sampled',trip_access=False,speed_limit=(25 if role=='semi_direct_ramp' else 30 if role=='loop_ramp' else 40 if role in ('right_ramp','semi_direct_ramp') else 80)/3.6)
        return w['id']
    wa=road(wi,ws,3,'mainline');wt=road(ws,em,3,'mainline');we=road(em,eo,3,'mainline')
    ea=road(ei,es,3,'mainline');et=road(es,wm,3,'mainline');ee=road(wm,wo,3,'mainline')
    sa=road(si,ss,2,'branch');sx=road(sm,so,2,'branch')
    wr=road(ws,sm,1,'right_ramp','ramp',cubic((-split,offset),(-profile['right_control_x'],offset),(-offset,profile['right_control_y']),(-offset,station),spacing))
    sr=road(ss,em,1,'right_ramp','ramp',cubic((offset,station),(offset,profile['right_control_y']),(profile['right_control_x'],offset),(split,offset),spacing))
    na=road(ss,nb,1,'branch','ramp');nt=road(nb,ne,1,'branch','bridge')
    nl=road(ne,wm,1,'semi_direct_ramp','ramp',cubic((offset,profile['bridge_end']),(offset,profile['semi_control_y']),(profile['semi_control_x'],-offset),(-split,-offset),spacing))
    el=road(es,sb,1,'loop_ramp','ramp',arc((-r-offset,-r-offset),r,math.pi/2,1.5*math.pi,spacing))
    st=road(sb,se,1,'branch','bridge');sd=road(se,sm,1,'branch','ramp')
    local=[('W','E','through',[wa,wt,we]),('E','W','through',[ea,et,ee]),('W','S','right',[wa,wr,sx]),('S','E','right',[sa,sr,we]),('E','S','left',[ea,el,st,sd,sx]),('S','W','left',[sa,na,nt,nl,ee])]
    compass={'N':{'W':'W','E':'E','S':'S'},'S':{'W':'E','E':'W','S':'N'},'W':{'W':'S','E':'N','S':'E'},'E':{'W':'N','E':'S','S':'W'}}[prefix]
    names={'N':'North','S':'South','W':'West','E':'East'}
    routes=[dict(origin=compass[a],destination=compass[b],turn=turn,way_ids=ids) for a,b,turn,ids in local]
    city.setdefault('interchanges',[]).append(dict(id=prefix,name=names[prefix]+' trumpet',type='trumpet',center=dict(x=center[0],y=center[1]),routes=routes))
    return {'W':(wi,wo),'E':(ei,eo),'S':(si,so)}

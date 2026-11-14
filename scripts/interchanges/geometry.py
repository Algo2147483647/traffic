"""Deterministic tangent-curve sampling for the interchange compiler."""
import math

def arc(center,radius,start,sweep,spacing):
    if radius<=0 or spacing<=0:raise ValueError('positive radius and sample spacing required')
    count=max(8,math.ceil(abs(sweep)*radius/spacing))
    return [(center[0]+radius*math.cos(start+sweep*i/count),center[1]+radius*math.sin(start+sweep*i/count)) for i in range(count+1)]

def cubic(a,b,c,d,spacing):
    length=sum(math.dist(x,y) for x,y in [(a,b),(b,c),(c,d)])
    count=max(8,math.ceil(length/spacing))
    return [tuple((1-t)**3*a[j]+3*(1-t)**2*t*b[j]+3*(1-t)*t*t*c[j]+t**3*d[j] for j in (0,1)) for t in (i/count for i in range(count+1))]

def rotate(p,angle,origin=(0,0)):
    c,s=math.cos(angle),math.sin(angle)
    return (round(origin[0]+p[0]*c-p[1]*s,5),round(origin[1]+p[0]*s+p[1]*c,5))

def validate_profile(p):
    if any(not isinstance(v,(int,float)) or not math.isfinite(v) or v<=0 for v in p.values()):raise ValueError('all profile values must be finite and positive')
    if not p['mainline_offset']+p['mainline_lanes']*p['lane_width']/2 < p['cd_offset']-p['cd_lanes']*p['lane_width']/2:raise ValueError('mainline and C/D envelopes overlap')
    if not p['portal_station']>p['split_station']>p['cd_station']>p['right_radius']+p['cd_offset']>p['loop_radius']+p['cd_offset']:raise ValueError('invalid exit/merge station order')
    # The outer right-turn curve must clear the inner loop pavement envelope.
    separation=math.sqrt(2)*(p['right_radius']-p['loop_radius'])-p['right_radius']-p['loop_radius']
    if separation < 2*p['lane_width']:raise ValueError('right ramp overlaps loop envelope')

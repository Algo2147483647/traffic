import unittest,math,json
from pathlib import Path
from interchanges.geometry import arc,cubic,validate_profile
class GeometryTests(unittest.TestCase):
 def test_loop_sampling(self):
  pts=arc((210,210),140,-math.pi/2,1.5*math.pi,6)
  self.assertLess(math.dist(pts[0],(210,70)),1e-8);self.assertLess(math.dist(pts[-1],(70,210)),1e-8)
  self.assertTrue(all(math.dist(a,b)<=6.001 for a,b in zip(pts,pts[1:])))
 def test_profile_rejects_overlap(self):
  p=json.loads((Path(__file__).resolve().parents[1]/'example/Map2/interchange-design.json').read_text(encoding='utf-8-sig'))['cloverleaf'];validate_profile(p)
  p['right_radius']=200
  with self.assertRaises(ValueError):validate_profile(p)
 def test_cubic_endpoints(self):
  pts=cubic((0,0),(10,0),(10,10),(20,10),2)
  self.assertEqual(pts[0],(0,0));self.assertEqual(pts[-1],(20,10))
 def test_trumpet_endpoint_tangency(self):
  city=json.loads((Path(__file__).resolve().parents[1]/'example/Map2/road.json').read_text())
  nodes={n['id']:n for n in city['intersections']}
  ways=city['ways']
  for w in ways:
   if nodes[w['from']].get('interchange') not in ('N','S','W','E') or w.get('role') not in ('loop_ramp','right_ramp','semi_direct_ramp'):continue
   pts=w['points']
   self.assertEqual(pts[0],nodes[w['from']]['position']);self.assertEqual(pts[-1],nodes[w['to']]['position'])
   for endpoint,incoming in [(0,True),(-1,False)]:
    neighbours=[v for v in ways if (v['to']==w['from'] if incoming else v['from']==w['to'])]
    for v in neighbours:
     vp=v.get('points',[nodes[v['from']]['position'],nodes[v['to']]['position']])
     a,b=(vp[-2],vp[-1]) if incoming else (vp[0],vp[1])
     c,d=(pts[0],pts[1]) if incoming else (pts[-2],pts[-1])
     u=(b['x']-a['x'],b['y']-a['y']);v=(d['x']-c['x'],d['y']-c['y'])
     cosine=sum(x*y for x,y in zip(u,v))/(math.hypot(*u)*math.hypot(*v))
     self.assertGreater(cosine,math.cos(math.radians(3)),w['id'])
 def test_trumpet_crossings_are_on_separate_levels(self):
  city=json.loads((Path(__file__).resolve().parents[1]/'example/Map2/road.json').read_text())
  nodes={n['id']:n for n in city['intersections']}
  def points(w):return [(p['x'],p['y']) for p in w.get('points',[nodes[w['from']]['position'],nodes[w['to']]['position']])]
  def cross(a,b):return a[0]*b[1]-a[1]*b[0]
  def sub(a,b):return (a[0]-b[0],a[1]-b[1])
  for group in ('N','S','W','E'):
   ways=[w for w in city['ways'] if nodes[w['from']].get('interchange')==group and nodes[w['to']].get('interchange')==group]
   crossings=0
   for k,a in enumerate(ways):
    for b in ways[k+1:]:
     if {a['from'],a['to']} & {b['from'],b['to']}:continue
     ap,bp=points(a),points(b)
     for p,q in zip(ap,ap[1:]):
      for r,s in zip(bp,bp[1:]):
       u,v=sub(q,p),sub(s,r);den=cross(u,v)
       if abs(den)<1e-8:continue
       t=cross(sub(r,p),v)/den;z=cross(sub(r,p),u)/den
       if 0<t<1 and 0<z<1:
        crossings+=1
        # Only the two fixed-height bridge decks may cross the ground mainline.
        self.assertEqual({a['structure'],b['structure']},{'ground','bridge'},(a['id'],b['id']))
        self.assertEqual(abs(nodes[a['from']]['elevation']-nodes[b['from']]['elevation']),6.5)
   self.assertEqual(crossings,4,group)
 def test_compact_geometry_curvature_and_grade(self):
  city=json.loads((Path(__file__).resolve().parents[1]/'example/Map2/road.json').read_text())
  nodes={n['id']:n for n in city['intersections']}
  for w in city['ways']:
   if not w.get('role'):continue
   pts=[(p['x'],p['y']) for p in w.get('points',[nodes[w['from']]['position'],nodes[w['to']]['position']])]
   length=sum(math.dist(a,b) for a,b in zip(pts,pts[1:]))
   # The executor uses smoothstep elevation; its peak slope is 1.5 * rise/run.
   grade=1.5*abs(nodes[w['from']]['elevation']-nodes[w['to']]['elevation'])/length
   self.assertLessEqual(grade,.04,w['id'])
   for a,b,c in zip(pts,pts[1:],pts[2:]):
    area=abs((b[0]-a[0])*(c[1]-a[1])-(b[1]-a[1])*(c[0]-a[0]))
    if area<1e-6:continue
    radius=math.dist(a,b)*math.dist(b,c)*math.dist(c,a)/(2*area)
    self.assertLessEqual(w['speed_limit']**2/radius,2.0,w['id'])
 def test_compact_envelopes(self):
  city=json.loads((Path(__file__).resolve().parents[1]/'example/Map2/road.json').read_text())
  for group,maximum in [('C',1741),('N',1241),('S',1241),('W',1241),('E',1241)]:
   points=[n['position'] for n in city['intersections'] if n.get('interchange')==group]
   for axis in ['x','y']:self.assertLessEqual(max(p[axis] for p in points)-min(p[axis] for p in points),maximum)
if __name__=='__main__':unittest.main()

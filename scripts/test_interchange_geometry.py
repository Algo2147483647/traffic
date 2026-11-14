import unittest,math,json
from pathlib import Path
from interchanges.geometry import arc,cubic,validate_profile
class GeometryTests(unittest.TestCase):
 def test_loop_sampling(self):
  pts=arc((210,210),140,-math.pi/2,1.5*math.pi,6)
  self.assertLess(math.dist(pts[0],(210,70)),1e-8);self.assertLess(math.dist(pts[-1],(70,210)),1e-8)
  self.assertTrue(all(math.dist(a,b)<=6.001 for a,b in zip(pts,pts[1:])))
 def test_profile_rejects_overlap(self):
  p=json.loads((Path(__file__).resolve().parents[1]/'configs/interchange-design.json').read_text(encoding='utf-8-sig'))['cloverleaf'];validate_profile(p)
  p['right_radius']=200
  with self.assertRaises(ValueError):validate_profile(p)
 def test_cubic_endpoints(self):
  pts=cubic((0,0),(10,0),(10,10),(20,10),2)
  self.assertEqual(pts[0],(0,0));self.assertEqual(pts[-1],(20,10))
if __name__=='__main__':unittest.main()

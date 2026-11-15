const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const route=require('./load-typescript.cjs')(path.join(__dirname,'../../frontend/src/rendering/metroRoute.ts'));
const layout=JSON.parse(fs.readFileSync(path.join(__dirname,'../../example/Map3/metro.json'),'utf8'));
test('directional itinerary follows real tracks including ring wrap and reverse',()=>{
 for(const spec of layout.lines){const line={...spec,track:spec.route,reverse_track:spec.reverse_route,stations:spec.stations.map((_,i)=>i)};
  for(let a=0;a<line.stations.length;a++)for(let b=0;b<line.stations.length;b++)if(a!==b){
   const points=route.points(line,a,b);assert.ok(points.length>=2);
   const n=line.stations.length,reverse=line.circular?(a-b+n)%n<(b-a+n)%n:b<a;
   const track=reverse&&line.circular?line.reverse_track:line.track;
   const length=ps=>ps.slice(1).reduce((s,p,i)=>s+Math.hypot(p.x-ps[i].x,p.y-ps[i].y),0);
   let expected=line.stop_chainages[b][reverse?1:0]-line.stop_chainages[a][reverse?1:0];if(expected<0)expected+=length(track);
   assert.ok(Math.abs(length(points)-expected)<1e-5,`${line.name} ${a} -> ${b}`);
   assert.ok(Math.hypot(points[0].x-spec.stations[a].x,points[0].y-spec.stations[a].y)<20);
   assert.ok(Math.hypot(points.at(-1).x-spec.stations[b].x,points.at(-1).y-spec.stations[b].y)<20);
  }
 }
});
test('completed, current and upcoming steps are distinct',()=>{assert.equal(route.status(0,1),'done');assert.equal(route.status(1,1),'current');assert.equal(route.status(2,1),'next');assert.equal(route.status(2,3),'done');});

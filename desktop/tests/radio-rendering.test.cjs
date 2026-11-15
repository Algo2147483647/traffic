const test=require('node:test'),assert=require('node:assert/strict'),path=require('node:path'),load=require('./load-typescript.cjs');
test('communications draws every admitted serving link and no honeycomb boundaries',()=>{
 const {drawRadio}=load(path.resolve(__dirname,'../../frontend/src/rendering/radio.ts'));
 let strokes=0;const ctx=new Proxy({createRadialGradient:()=>({addColorStop(){}}),stroke:()=>strokes++},{get:(o,k)=>k in o?o[k]:()=>{},set:(o,k,v)=>(o[k]=v,true)});
 const state={mapView:'communications',showRadio:true,city:{communications:{sites:[{id:'A',position_m:{x:0,y:0,z:0},mast_height_m:25,coverage_radius_m:256}]}},snapshot:{communications:{cells:[{site_id:'A',associated:2}],terminals:[1,2,3].map(n=>({id:String(n),position_m:{x:n,y:1,z:1},status:n===3?'outage':'connected',serving_cell:n===3?'':'C',neighbours:n===3?[]:[{cell_id:'C',site_id:'A'}]}))}}};
 drawRadio(ctx,state,p=>p,()=>true,.01,[]);assert.equal(strokes,2);
 state.mapView='traffic';strokes=0;drawRadio(ctx,state,p=>p,()=>true,.01,[]);assert.equal(strokes,0);
});

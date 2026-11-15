const test=require('node:test');const assert=require('node:assert/strict');const path=require('node:path');const load=require('./load-typescript.cjs');
test('duplicate city observations retain camera and stale runs cannot replace the current world',()=>{
 const s=load(path.resolve(__dirname,'../../frontend/src/state/store.ts'),{require:()=>({useSyncExternalStore(){}}),localStorage:{getItem:()=>null},performance:{now:()=>1}});
 s.receiveCity({name:'Radial Grid'});s.scene.camera.zoom=5;s.scene.radioTerminal='UE1';s.receiveCity({name:'Radial Grid'});assert.equal(s.scene.camera.zoom,5);assert.equal(s.scene.radioTerminal,'UE1');
 s.receiveSnapshot({kernel:{run:2,topology:2,world:3},tick:10,mode:'traffic'});s.receiveSnapshot({kernel:{run:1,topology:1,world:100},tick:100,mode:'traffic'});assert.equal(s.scene.snapshot.kernel.run,2);
 s.receiveCity({name:'Map 2'});assert.equal(s.scene.camera.zoom,1);assert.equal(s.scene.radioTerminal,'');
});

test('view changes preserve the world and radio history deduplicates paused samples',()=>{
 const s=load(path.resolve(__dirname,'../../frontend/src/state/store.ts'),{require:()=>({useSyncExternalStore(){}}),localStorage:{getItem:()=>null},performance:{now:()=>1}});
 const snapshot={kernel:{run:1,topology:1,world:1},tick:1,mode:'traffic',communications:{time_us:1000000,connected:2,outage:1,terminals:[]}};
 s.receiveSnapshot(snapshot);s.receiveSnapshot(snapshot);assert.equal(s.radioHistory.length,1);
 for(const v of ['all','traffic','communications']){s.setMapView(v);assert.equal(s.scene.snapshot,snapshot);assert.equal(s.radioHistory.length,1)}
 s.receiveSnapshot({...snapshot,kernel:{run:2,topology:1,world:1},communications:{...snapshot.communications,time_us:0}});assert.equal(s.radioHistory.length,1);assert.equal(s.radioHistory[0].time,0);
});

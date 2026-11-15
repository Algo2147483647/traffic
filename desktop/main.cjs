const { app, BrowserWindow, Menu, dialog, session } = require('electron');
const path = require('node:path');
const fs = require('node:fs');
const { startBackend } = require('./backend.cjs');
let backend;
let quitting = false;
let cleaned = false;
let window;
const smoke = process.argv.includes('--smoke-test');
app.setName('Meridian Traffic');

async function createWindow() {
  window = new BrowserWindow({
    width: 1440, height: 940, minWidth: 1000, minHeight: 680, show: false,
    title: 'Meridian Traffic', backgroundColor: '#0b111b',
    webPreferences: { partition: 'traffic-desktop', nodeIntegration: false, contextIsolation: true, sandbox: true, backgroundThrottling: false }
  });
  window.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));
  window.webContents.on('will-navigate', (event, url) => {
    if (url !== backend.url + '/') event.preventDefault();
  });
  window.webContents.on('will-redirect', event => event.preventDefault());
  window.webContents.on('render-process-gone', (_event, details) => fail(new Error(`Window renderer stopped: ${details.reason}`)));
  window.once('ready-to-show', () => window.show());
  await window.loadURL(backend.url + '/');
  if (smoke) {
    const result = await window.webContents.executeJavaScript(process.env.TRAFFIC_SMOKE_FOCUS === "react" ? `(${require("./react-smoke.cjs").toString()})()` : `(async () => {
      const drawnLabels=[];const originalFillText=CanvasRenderingContext2D.prototype.fillText;CanvasRenderingContext2D.prototype.fillText=function(text,...args){if(drawnLabels.length>2000)drawnLabels.splice(0,1000);drawnLabels.push(String(text));return originalFillText.call(this,text,...args);};
      const errors=[];window.addEventListener('error',event=>errors.push(event.message));
      const wait = ms => new Promise(r => setTimeout(r, ms));
      for (let i = 0; i < 100 && document.querySelector('#map-preset')?.options[0]?.value !== "preset-1"; i++) await wait(100);
      if (document.querySelector('#map-preset')?.options[0]?.value !== "preset-1") throw Error('Map selector did not load');
      const coordinates=document.querySelector('#show-coordinates');
      if(!coordinates.checked)coordinates.click();
      const get = p => fetch(p).then(r => r.json());
      const control = body => fetch('/api/control', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }).then(r => { if (!r.ok) throw Error('Control failed'); return r.json(); });
      const preset = ${JSON.stringify(process.env.TRAFFIC_SMOKE_PRESET || '')};
      if (preset) { await control({action:'set_preset',preset_id:preset}); await wait(800); }
      function sliceHeight(lo,hi,all=false){document.querySelector('#height-min').value=String(lo);document.querySelector('#height-max').value=String(hi);const select=document.querySelector('#layer-filter');select.value=all?'all':'slice';select.dispatchEvent(new Event('change',{bubbles:true}));}
      if (${JSON.stringify(process.env.TRAFFIC_SMOKE_FOCUS === 'heights')}) {
        await control({action:'pause'});document.querySelector('#settings-toggle').click();
        let snap=await get('/api/snapshot');const city=await get('/api/city');
        if(snap.metro.lines.map(l=>l.elevation_m).join(',')!=='-50,-55,-60,-65'||!city.ways.some(w=>w.structure==='bridge'&&w.start_elevation===5)||!city.ways.some(w=>w.structure==='tunnel'&&w.start_elevation===-5))throw Error('Physical material migration failed');
        sliceHeight(-51,-49);drawnLabels.length=0;document.querySelector('#fit-button').click();await wait(120);
        const trains=drawnLabels.filter(t=>/^T[0-9]+ /.test(t));if(!trains.length||trains.some(t=>Number(t.match(/^T([0-9]+)/)[1])>=100))throw Error('Height slice mixed metro elevations: '+JSON.stringify(trains));
        document.querySelector('#height-min').value='10';document.querySelector('#height-min').dispatchEvent(new Event('change',{bubbles:true}));if(!document.querySelector('#height-feedback').textContent)throw Error('Invalid range was accepted');sliceHeight(-51,-49);
        const input=document.querySelector('#metro-levels input');input.value='-52.25';document.querySelector('#metro-speed').value='350';document.querySelector('#metro-headway').value='15';document.querySelector('#metro-cars').value='13';document.querySelector('#metro-dwell').value='2';document.querySelector('#metro-apply').click();
        for(let i=0;i<50;i++){snap=await get('/api/snapshot');if(snap.metro.lines[0].elevation_m===-52.25)break;await wait(40);}
        if(snap.metro.lines[0].elevation_m!==-52.25||snap.metro.config.max_speed_kmh!==350||snap.metro.config.headway_s!==15||snap.metro.config.cars!==13)throw Error('Editable settings not applied');
        for(const [id,value] of [['walk-speed-control','4.2'],['time-scale','12'],['safe-gap-control','1.5']]){const e=document.querySelector('#'+id);e.value=value;e.dispatchEvent(new Event('change',{bubbles:true}));await wait(100);}
        snap=await get('/api/snapshot');if(snap.walk_speed_mps!==4.2||snap.time_scale!==12)throw Error('Numeric control still clamped');
        sliceHeight(-53,-52);document.querySelector('#metro-settings').scrollIntoView({block:'start'});await wait(100);if(errors.length)throw Error(errors.join('; '));
        return {heights:snap.metro.lines.map(l=>l.elevation_m),speed:snap.metro.config.max_speed_kmh,cars:snap.metro.config.cars,walk:snap.walk_speed_mps,rate:snap.time_scale};
      }
      if (${JSON.stringify(process.env.TRAFFIC_SMOKE_FOCUS === 'service')}) {
        await control({action:'pause'});await control({action:'set_demand',demand:{trips_per_minute:20,taxi_share:0,metro_share:1}});const initial=(await get('/api/snapshot')).metro;
        if(initial.services.length!==4||initial.services.some(s=>s.plan.fleet<=2))throw Error('Fleet still fixed at two');
        document.querySelector('#rail-service-toggle').click();const panel=document.querySelector('#rail-service-panel');if(panel.hidden)throw Error('Rail service panel did not open');
        const rect=panel.getBoundingClientRect(),drag=document.querySelector('#rail-service-drag');drag.dispatchEvent(new PointerEvent('pointerdown',{button:0,clientX:rect.left+50,clientY:rect.top+20,bubbles:true}));window.dispatchEvent(new PointerEvent('pointermove',{clientX:rect.left+90,clientY:rect.top+50}));window.dispatchEvent(new PointerEvent('pointerup'));if(panel.getBoundingClientRect().left<=rect.left)throw Error('Service panel not draggable');
        document.querySelector('#rail-service-headway').value='900';document.querySelector('#rail-service-apply').click();
        for(let i=0;i<50;i++){if((await get('/api/snapshot')).metro.config.service?.target_headway_s===900)break;await wait(40);}
        let reduced=(await get('/api/snapshot')).metro;if(reduced.services.some(s=>s.plan.fleet>=initial.services.find(x=>x.plan.line===s.plan.line).plan.fleet))throw Error('Planner did not resize fleet');
        document.querySelector('#rail-service-headway').value='120';document.querySelector('#rail-service-apply').click();await wait(200);
        let snap;for(let i=0;i<16;i++){snap=await control({action:'step',delta:4});}
        await wait(150);if(snap.metro.services.every(s=>s.departed===0)||!document.querySelector('#rail-service-trips table'))throw Error('Scheduled services did not depart');
        if(!snap.demand.conserved||snap.metro.generated!==snap.metro.waiting+snap.metro.onboard+snap.metro.completed)throw Error('Service passenger accounting');
        if(errors.length)throw Error(errors.join('; '));return {fleet:snap.metro.services.map(s=>s.plan.fleet),departed:snap.metro.services.map(s=>s.departed),conserved:snap.demand.conserved};
      }
      async function checkExchangeInspector() {
        document.querySelector('#settings-panel').hidden=true;
        const selector=document.querySelector('#layer-filter');selector.value='all';selector.dispatchEvent(new Event('change',{bubbles:true}));
        const positions=new Map(),savedText=CanvasRenderingContext2D.prototype.fillText;
        CanvasRenderingContext2D.prototype.fillText=function(text,x,y,...args){if(String(text).startsWith('Exchange '))positions.set(String(text).split(' / ')[0],{x:x-8,y:y+10});return savedText.call(this,text,x,y,...args);};
        document.querySelector('#fit-button').click();document.querySelector('#zoom-in-button').click();await wait(150);
        const m=(await get('/api/snapshot')).metro,canvas=document.querySelector('#city-canvas'),rect=canvas.getBoundingClientRect();
        const available=m.interchanges.filter(s=>{const p=positions.get(s.name);return p&&p.x>0&&p.y>0&&p.x<rect.width&&p.y<rect.height;});
        const site=available.find(s=>m.station_details.find(d=>d.id===s.id)?.passenger_ids.length)||available[0];
        if(!site)throw Error('No visible exchange label');const p=positions.get(site.name);
        canvas.dispatchEvent(new MouseEvent('click',{clientX:rect.left+p.x,clientY:rect.top+p.y,bubbles:true}));await wait(100);
        if(document.querySelector('#metro-station-inspector').hidden||document.querySelector('#metro-station-name').textContent!==site.name)throw Error('Exchange click did not open details');
        if(document.querySelectorAll('#metro-platforms section').length!==2||document.querySelectorAll('#metro-platforms strong').length!==3)throw Error('Missing lines or directional queues');
        const count=m.station_details.find(d=>d.id===site.id).passenger_ids.length;
        if(!count)throw Error('No station passenger available for itinerary check');
        if(count&&(!document.querySelector('#metro-passenger-select').options.length||!document.querySelector('#metro-passenger-path li.current-step')))throw Error('Passenger itinerary missing');
        if(!document.querySelector('#metro-platforms').textContent.includes('waiting'))throw Error('Queue status missing');
        const passengers=document.querySelector('#metro-passenger-select'),walking=[...passengers.options].find(o=>o.textContent.includes('transferring'));
        if(walking){passengers.value=walking.value;passengers.dispatchEvent(new Event('change',{bubbles:true}));await wait(100);if(!document.querySelector('#metro-passenger-path .current-step').textContent.includes('Transfer at')||!document.querySelector('#metro-passenger-state').textContent.includes('walk remaining'))throw Error('Selected transfer path missing');}

        const routeStrokes=[],originalStroke=CanvasRenderingContext2D.prototype.stroke;
        CanvasRenderingContext2D.prototype.stroke=function(...args){if(['#8eabbc','#f5ce58','#8dbdff'].includes(this.strokeStyle))routeStrokes.push({color:this.strokeStyle,dash:this.getLineDash()});return originalStroke.apply(this,args);};
        passengers.dispatchEvent(new Event('change',{bubbles:true}));await wait(100);
        if(document.querySelector('#metro-route-legend').hidden||!routeStrokes.some(s=>s.color==='#f5ce58')||!routeStrokes.some(s=>s.color==='#8dbdff'&&s.dash.length))throw Error('Passenger map route styles missing');
        if(walking){const selected=passengers.value;await control({action:'step',delta:64});await wait(150);if(passengers.value!==selected||!routeStrokes.some(s=>s.color==='#8eabbc'))throw Error('Route tracking or completed segment missing: '+JSON.stringify({selected,actual:passengers.value,steps:document.querySelector('#metro-passenger-path').textContent,colors:routeStrokes.map(s=>s.color)}));}
        CanvasRenderingContext2D.prototype.stroke=originalStroke;
        CanvasRenderingContext2D.prototype.fillText=savedText;
      }
      if (${JSON.stringify(process.env.TRAFFIC_SMOKE_FOCUS === 'exchange')}) {
        await control({action:'pause'});
        const initial=await get('/api/snapshot');
        await control({action:'set_metro',metro:{...initial.metro.config,headway_s:60,dwell_s:5}});
        await control({action:'set_demand',demand:{trips_per_minute:30,taxi_share:0,metro_share:1}});
        let snap;
        for(let i=0;i<500;i++){
          snap=await control({action:'step',delta:4});
          if(!snap.demand.conserved)throw Error('Shared accounting: '+JSON.stringify(snap.demand));
          if(snap.metro.generated!==snap.metro.waiting+snap.metro.onboard+snap.metro.completed)throw Error('Metro accounting');
          if(snap.metro.transferring>0&&snap.metro.station_details.some(d=>d.passenger_ids.length>1))break;
        }
        if(!snap.metro.transferring)throw Error('No real transfer walkers');
        await wait(150);await checkExchangeInspector();
        if(errors.length)throw Error(errors.join('; '));
        return {station:document.querySelector('#metro-station-name').textContent,passengers:document.querySelector('#metro-passenger-select').options.length,routeSteps:document.querySelectorAll('#metro-passenger-path li').length,transferring:snap.metro.transferring,conserved:snap.demand.conserved};
      }
      document.querySelector('#settings-toggle').click();
      if (document.querySelector('#settings-panel').hidden) throw Error('Settings did not open');
      await control({action:'pause'});
      const spawn=document.querySelector('#spawn-rate-control'),gap=document.querySelector('#safe-gap-control');
      if(gap.value!=='2.5'||gap.type!=='number'||document.querySelector('#taxi-fleet').value!=='32'||document.querySelector('#taxi-demand').value!=='2')throw Error('Settings defaults not calibrated');
      spawn.value='30';spawn.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('#apply-load-button').click();
      for(let i=0;i<50&&(await get('/api/snapshot')).load.spawn_rate_per_second!==0.5;i++)await wait(50);
      if((await get('/api/snapshot')).load.spawn_rate_per_second!==0.5)throw Error('Per-minute demand conversion failed');
      spawn.value='0';spawn.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('#apply-load-button').click();
      for(let i=0;i<50&&(await get('/api/snapshot')).load.spawn_rate_per_second!==0;i++)await wait(50);
      if((await get('/api/snapshot')).load.spawn_rate_per_second!==0)throw Error('Demand reset failed');

      const panel=document.querySelector('#settings-panel'),initial=panel.getBoundingClientRect();
      const drag=document.querySelector('#settings-drag');
      drag.dispatchEvent(new PointerEvent('pointerdown',{pointerId:41,button:0,clientX:initial.left+60,clientY:initial.top+20,bubbles:true}));
      window.dispatchEvent(new PointerEvent('pointermove',{pointerId:41,clientX:initial.left+140,clientY:initial.top+65}));window.dispatchEvent(new PointerEvent('pointerup',{pointerId:41}));
      const moved=panel.getBoundingClientRect();if(moved.left<=initial.left||moved.top<=initial.top)throw Error('Settings drag failed');
      document.querySelector('#settings-resize').dispatchEvent(new PointerEvent('pointerdown',{pointerId:42,button:0,clientX:moved.right,clientY:moved.bottom,bubbles:true}));
      window.dispatchEvent(new PointerEvent('pointermove',{pointerId:42,clientX:moved.right+80,clientY:moved.bottom+30}));window.dispatchEvent(new PointerEvent('pointerup',{pointerId:42}));
      const resized=panel.getBoundingClientRect();if(resized.width<=moved.width)throw Error('Settings resize failed');
      document.querySelector('#settings-maximize').click();if(panel.getBoundingClientRect().width<innerWidth-20)throw Error('Settings maximize failed');
      document.querySelector('#settings-maximize').click();if(Math.abs(panel.getBoundingClientRect().width-resized.width)>1)throw Error('Settings restore failed');

      const metroLayers=(await get('/api/snapshot')).metro.lines.map(l=>l.elevation_m);
      for(const height of [...metroLayers,0,5,-5,null]){sliceHeight(height===null?0:height,height===null?0:height,height===null);drawnLabels.length=0;await wait(160);const visible=drawnLabels.some(label=>/^T[0-9]+ /.test(label));if(visible!==(height===null||metroLayers.includes(height)))throw Error('Metro height visibility failed: '+height);}
      if(document.querySelector('#settings-panel details'))throw Error('Settings still contains collapsed sections');
      document.querySelector('#settings-close').click();
      await control({ action: 'pause' });
      if (!(await get('/api/snapshot')).paused) throw Error('Pause failed');
      await control({ action: 'resume' });
      const before = await get('/api/snapshot'); await wait(400);
      const after = await get('/api/snapshot');
      if (after.time <= before.time) throw Error('Simulation did not advance');
      if (preset && ${JSON.stringify(process.env.TRAFFIC_SMOKE_DETAIL === '1')}) {
        const city=await get('/api/city');
        if(city.world_units_per_km!==1000)throw Error('Missing coordinate scale');
        if(!drawnLabels.includes('X · km')||!drawnLabels.includes('Y · km')||(preset==='four-cell-interchanges'&&!drawnLabels.includes('6 × 6 km')))throw Error('Missing kilometer rulers');
        document.querySelector('#zoom-in-button').click();await wait(100);
        document.querySelector('#zoom-out-button').click();await wait(100);
        for(const group of city.interchanges){
          const canvas=document.querySelector('#city-canvas'),rect=canvas.getBoundingClientRect(),b=city.bounds;
          const scale=Math.min((rect.width-100)/(b.max_x-b.min_x),(rect.height-100)/(b.max_y-b.min_y));
          const x=rect.left+rect.width/2+(group.center.x-(b.min_x+b.max_x)/2)*scale;
          const y=rect.top+rect.height/2+(group.center.y-(b.min_y+b.max_y)/2)*scale;
          canvas.dispatchEvent(new MouseEvent('click',{clientX:x,clientY:y,bubbles:true}));await wait(100);
          if(document.querySelector('#interchange-inspector').hidden)throw Error('Interchange details did not open: '+group.id);
          const movements=document.querySelector('#interchange-movement');
          if(movements.options.length!==group.routes.length)throw Error('Missing interchange movements: '+group.id);
          for(let i=0;i<group.routes.length;i++){movements.value=String(i);movements.dispatchEvent(new Event('change',{bubbles:true}));}
        }
        document.querySelector("#inspector-toggle").click();await wait(100);
      }
      let taxiResult=null;
      drawnLabels.length=0;coordinates.click();await wait(150);
      if(coordinates.checked||drawnLabels.some(text=>text==='X · km'||text==='Y · km'||text==='6 × 6 km'))throw Error('Coordinates did not hide');
      if(localStorage.getItem('traffic-show-coordinates')!=='false')throw Error('Coordinate preference not saved');
      drawnLabels.length=0;coordinates.click();await wait(150);
      if(!coordinates.checked||!drawnLabels.includes('X · km')||!drawnLabels.includes('Y · km'))throw Error('Coordinates did not restore');
      if (${JSON.stringify(process.env.TRAFFIC_SMOKE_TAXI === '1')}) {
        document.querySelector('#taxi-fleet').value='6';document.querySelector('#taxi-demand').value='0';
        document.querySelector('#simulation-mode').value='taxi';document.querySelector('#simulation-mode').dispatchEvent(new Event('change',{bubbles:true}));
        for(let i=0;i<50&&(await get('/api/snapshot')).mode!=='taxi';i++)await wait(100);
        let snap=await get('/api/snapshot');if(snap.mode!=='taxi'||snap.taxi.vehicles.length!==6)throw Error('Taxi mode did not start');
        await control({action:'pause'});
        for(let i=0;i<600;i++){snap=await control({action:'step',delta:4});if(snap.taxi.completed>=2)break;}
        if(snap.taxi.completed<2||snap.taxi.picked<2)throw Error('Taxi pickup/dropoff failed');
        taxiResult={completed:snap.taxi.completed,picked:snap.taxi.picked};
        const planned=snap.people.filter(p=>p.route.length>1);
        if(!planned.length||planned.some(p=>p.route_entry_lanes?.length!==p.route.length||p.route_exit_lanes?.length!==p.route.length))throw Error('Missing expanded lane paths');
        taxiResult.expandedLaneRoutes=planned.length;
        const liveTime=snap.time,liveCreated=snap.taxi.created;
        document.querySelector('#taxi-fleet-range').value='9';document.querySelector('#taxi-fleet-range').dispatchEvent(new Event('input',{bubbles:true}));
        document.querySelector('#taxi-demand-range').value='15';document.querySelector('#taxi-demand-range').dispatchEvent(new Event('input',{bubbles:true}));
        document.querySelector('#taxi-load-apply').click();
        for(let i=0;i<40;i++){snap=await get('/api/snapshot');if(snap.taxi.config.fleet===9)break;await wait(50);}
        if(snap.taxi.config.fleet!==9||snap.taxi.config.orders_per_minute!==15||snap.taxi.vehicles.length!==9)throw Error('Live taxi controls failed');
        if(snap.time!==liveTime||snap.taxi.created!==liveCreated)throw Error('Live controls reset scenario');

        await control({action:'set_mode',mode:'traffic'});if((await get('/api/snapshot')).mode!=='traffic')throw Error('Traffic mode not restored');
        const algorithms=await get('/api/dispatch/algorithms');
        if(!algorithms.some(a=>a.id==='experimental.wait_balanced'))throw Error('Worker manifest missing');
        await control({action:'set_mode',mode:'taxi',taxi:{fleet:6,orders_per_minute:0,algorithm:'experimental.wait_balanced',seed:71429}});
        await control({action:'set_demand',demand:{trips_per_minute:60,taxi_share:1}});
        const beforeWorkerOrders=snap.taxi.created;
        for(let i=0;i<150;i++){snap=await control({action:'step',delta:1});if(snap.taxi.created>beforeWorkerOrders&&snap.taxi.orders.some(o=>o.state==='Assigned'))break;if(snap.taxi.last_error)throw Error(snap.taxi.last_error);await wait(30);}
        if(!snap.taxi.orders.some(o=>o.state==='Assigned'))throw Error('Python worker did not assign');
        const beforeMixTime=snap.time,beforeMixRun=snap.kernel.run;
        await control({action:'set_demand',demand:{trips_per_minute:30,taxi_share:.3,metro_share:.3}});
        snap=await get('/api/snapshot');if(snap.time!==beforeMixTime||snap.kernel.run!==beforeMixRun)throw Error('Mix reset active run');
        for(let i=0;i<80;i++){snap=await control({action:'step',delta:2});if(snap.people.some(p=>p.journey_id)&&snap.demand.car_trips>0&&snap.demand.taxi_trips>0)break;await wait(20);}
        if(!snap.people.some(p=>p.journey_id)||!snap.taxi.vehicles.length)throw Error('Both modes did not coexist');
        if(!snap.demand.conserved||snap.demand.generated!==snap.demand.queued+snap.demand.active+snap.demand.completed+snap.demand.cancelled)throw Error('Demand conservation failed');
        await control({action:'set_walk_speed',walk_speed_mps:1.8});
        const available=await get('/api/algorithms/workers');if(!available.some(a=>a.id==='python.route')||!available.some(a=>a.id==='python.signal'))throw Error('Algorithm manifests missing');
        await control({action:'set_algorithms',algorithms:{route:{id:'python.route',parameters:{prefer_alternative:1}},signal:{id:'python.signal',parameters:{}}}});
        for(let i=0;i<120;i++){snap=await control({action:'step',delta:.05});if(snap.algorithms.applied>=2||snap.algorithms.fallbacks)break;await wait(30);}
        if(snap.algorithms.applied<2||snap.algorithms.fallbacks)throw Error('External algorithms: '+JSON.stringify(snap.algorithms));
        taxiResult.mixed=snap.demand;taxiResult.algorithms=snap.algorithms;
        if(snap.walk_speed_mps!==1.8)throw Error('Walking setting not applied');
        await control({action:'set_algorithms',algorithms:{route:{id:'native'},signal:{id:'native'}}});
        await control({action:'pause'});document.querySelector('#settings-toggle').click();await wait(150);
      }
      if(errors.length) throw Error(errors.join('; '));
      if (typeof require !== 'undefined') throw Error('Renderer has Node access');
      document.querySelector('#metro-cars').value='6';document.querySelector('#metro-speed').value='300';document.querySelector('#metro-enabled').checked=true;
      document.querySelector('#metro-headway').value='60';document.querySelector('#metro-demand').value='30';document.querySelector('#metro-capacity').value='120';document.querySelector('#metro-dwell').value='5';document.querySelector('#metro-apply').click();
      for(let i=0;i<50&&(await get('/api/snapshot')).metro.config.headway_s!==60;i++)await wait(50);
      const configured=(await get('/api/snapshot')).metro.config;
      if(configured.cars!==6||configured.max_speed_kmh!==300||configured.headway_s!==60)throw Error('Metro UI controls failed');
      await control({action:'set_demand',demand:{trips_per_minute:12,taxi_share:.3,metro_share:.3}});
      let metroSnap;
      for(let i=0;i<2000;i++){metroSnap=await control({action:'step',delta:4});if(metroSnap.metro.completed>=2&&metroSnap.metro.departures>=4&&(preset!=='radial-grid-cloverleaf'||metroSnap.metro.transfers_completed>=2))break;await wait(20);}
      const metroResult=metroSnap.metro;
      if(preset==='radial-grid-cloverleaf'&&(metroResult.interchanges.length!==6||metroResult.transfers_completed<2||metroResult.lines.some(l=>l.elevation_m>-50)))throw Error('Map 3 transfers or underground levels missing');
      if(!metroSnap.demand.conserved||metroSnap.demand.metro_trips===0)throw Error('Three mode accounting failed: '+JSON.stringify(metroSnap.demand));
      if(metroResult.completed<2||metroResult.departures<4||metroResult.generated!==metroResult.waiting+metroResult.onboard+metroResult.completed)throw Error('Metro did not deliver conserved passengers: '+JSON.stringify({time:metroSnap.time,generated:metroResult.generated,waiting:metroResult.waiting,onboard:metroResult.onboard,completed:metroResult.completed,departures:metroResult.departures}));
      if(metroResult.trains.some(t=>t.car_poses.length!==6||t.car_length_m!==22.8||t.car_width_m!==3||t.passengers>t.capacity||t.cars.length!==6||Math.abs(t.length_m-136.8)>1e-6||t.cars.reduce((a,b)=>a+b,0)!==t.passengers||t.cars.some((n,i)=>n>t.car_capacities[i])||t.speed_mps>300/3.6))throw Error('Metro car manifest or speed invalid');
      if(preset==='preset-1'&&(metroResult.lines.length!==3||metroResult.stations.length!==15||metroResult.trains.length<6))throw Error('Radial metro missing');
      const changedLevel=preset==='radial-grid-cloverleaf'?-70.25:-20.25;const levelInput=document.querySelector('#metro-levels input');levelInput.value=String(changedLevel);document.querySelector('#metro-apply').click();
      for(let i=0;i<50&&(await get('/api/snapshot')).metro.lines[0].elevation_m!==changedLevel;i++)await wait(50);
      if((await get('/api/snapshot')).metro.lines[0].elevation_m!==changedLevel)throw Error('Metro level control not connected');
      if(errors.length)throw Error(errors.join('; '));
      // Rendering regression: real cars and tunnel widths must scale together.
      const carWidths=[],tunnelWidths=[],colors=['#72bde8','#c7a1ef','#7ed4b1'];
      const fillRect=CanvasRenderingContext2D.prototype.fillRect,stroke=CanvasRenderingContext2D.prototype.stroke;
      CanvasRenderingContext2D.prototype.fillRect=function(x,y,w,h){if(colors.includes(this.fillStyle))carWidths.push(w);return fillRect.call(this,x,y,w,h);};
      CanvasRenderingContext2D.prototype.stroke=function(...args){if(colors.includes(this.strokeStyle))tunnelWidths.push(this.lineWidth);return stroke.apply(this,args);};
      document.querySelector('#fit-button').click();await wait(100);const carBefore=carWidths[0],tunnelBefore=tunnelWidths[0];carWidths.length=0;tunnelWidths.length=0;
      document.querySelector('#zoom-in-button').click();await wait(100);
      if(!carBefore||!tunnelBefore||Math.abs(carWidths[0]/carBefore-1.5)>1e-6||Math.abs(tunnelWidths[0]/tunnelBefore-1.5)>1e-6)throw Error('Cars or tunnels retain fixed pixel dimensions');
      CanvasRenderingContext2D.prototype.fillRect=fillRect;CanvasRenderingContext2D.prototype.stroke=stroke;document.querySelector('#fit-button').click();
      document.querySelector('#settings-panel').hidden=false;document.querySelector('#metro-settings').scrollIntoView({block:'start'});await wait(150);
      if (${JSON.stringify(process.env.TRAFFIC_SMOKE_FOCUS === 'metro')}) {
        document.querySelector('#settings-panel').hidden=true;
        sliceHeight(changedLevel,changedLevel);await wait(100);
        const m=(await get('/api/snapshot')).metro,line=m.lines[0],tr=m.trains.find(t=>t.line===line.id&&t.passengers>0)||m.trains[0];
        const xs=line.track.map(p=>p.x),ys=line.track.map(p=>p.y),minX=Math.min(...xs),maxX=Math.max(...xs),minY=Math.min(...ys),maxY=Math.max(...ys);
        const canvas=document.querySelector('#city-canvas'),rect=canvas.getBoundingClientRect(),scale=Math.min((rect.width-100)/(maxX-minX),(rect.height-100)/(maxY-minY));
        const x=rect.width/2+(tr.position.x-(minX+maxX)/2)*scale,y=rect.height/2+(tr.position.y-(minY+maxY)/2)*scale;
        canvas.dispatchEvent(new WheelEvent('wheel',{deltaY:-Math.log(32)/.0015,clientX:rect.left+x,clientY:rect.top+y,bubbles:true,cancelable:true}));
        const capture=canvas.setPointerCapture;canvas.setPointerCapture=()=>{};
        canvas.dispatchEvent(new PointerEvent('pointerdown',{pointerId:77,clientX:rect.left+x,clientY:rect.top+y,bubbles:true}));
        canvas.dispatchEvent(new PointerEvent('pointermove',{pointerId:77,clientX:rect.left+rect.width/2,clientY:rect.top+rect.height/2,bubbles:true}));
        canvas.dispatchEvent(new PointerEvent('pointerup',{pointerId:77,bubbles:true}));canvas.setPointerCapture=capture;await wait(150);
      }
      if (${JSON.stringify(process.env.TRAFFIC_SMOKE_FOCUS === 'roads')}) {
        document.querySelector('#settings-panel').hidden=true;
        sliceHeight(0,0);document.querySelector('#fit-button').click();await wait(150);
      }
      if (${JSON.stringify(process.env.TRAFFIC_SMOKE_FOCUS === 'overview')}) {
        document.querySelector('#settings-panel').hidden=true;
        const selector=document.querySelector('#layer-filter');selector.value='all';selector.dispatchEvent(new Event('change',{bubbles:true}));
        document.querySelector('#fit-button').click();document.querySelector('#zoom-in-button').click();document.querySelector('#zoom-in-button').click();await wait(150);
      }
      if(preset==='radial-grid-cloverleaf')await checkExchangeInspector();
      if(errors.length)throw Error(errors.join('; '));
      return { metroResult:{transfers:metroResult.transfers_completed,exchanges:(metroResult.interchanges||[]).length,completed:metroResult.completed,departures:metroResult.departures,generated:metroResult.generated},taxiResult, title: document.title, people: after.people.length, time: after.time, presets: document.querySelector('#map-preset').options.length };
    })()`);
    const output = process.env.TRAFFIC_SMOKE_OUTPUT || path.join(app.getPath('temp'), 'traffic-desktop-smoke.png');
    fs.writeFileSync(output, (await window.webContents.capturePage()).toPNG());
    console.log(JSON.stringify({ ...result, screenshot: output }));
    app.quit();
  }
}

function fail(error) {
  if (quitting) return;
  console.error(error);
  process.exitCode = 1;
  if (!smoke) dialog.showErrorBox('Meridian Traffic', error.message);
  if (backend) app.quit();
  else app.exit(1);
}

if (!app.requestSingleInstanceLock()) app.quit();
else {
  app.on('second-instance', () => {
    if (window && !window.isDestroyed()) { if (window.isMinimized()) window.restore(); window.show(); window.focus(); }
    else if (backend && !quitting) createWindow().catch(fail);
  });
  app.whenReady().then(async () => {
    const platform = process.platform === 'darwin' ? 'mac' : process.platform === 'win32' ? 'win' : 'linux';
    backend = await startBackend({
      runtime: app.isPackaged ? path.join(process.resourcesPath, 'traffic-runtime') : path.join(__dirname, 'runtime', `${platform}-${process.arch}`),
      configs: app.isPackaged ? path.join(process.resourcesPath, 'example') : path.join(__dirname, '../example'),
      onExit: fail
    });
    if (quitting) { await backend.stop(); return; }
    const ses = session.fromPartition('traffic-desktop');
    ses.setPermissionRequestHandler((_wc, _permission, callback) => callback(false));
    ses.setPermissionCheckHandler(() => false);
    ses.webRequest.onBeforeSendHeaders((details, callback) => {
      if (new URL(details.url).origin !== backend.url) return callback({ cancel: true });
      callback({ requestHeaders: { ...details.requestHeaders, 'X-Traffic-Token': backend.token } });
    });
    Menu.setApplicationMenu(Menu.buildFromTemplate([
      ...(process.platform === 'darwin' ? [{ role: 'appMenu' }] : []),
      { label: 'File', submenu: [{ role: process.platform === 'darwin' ? 'close' : 'quit' }] },
      { role: 'editMenu' },
      { label: 'View', submenu: [{ role: 'reload' }, { role: 'togglefullscreen' }, ...(!app.isPackaged ? [{ role: 'toggleDevTools' }] : [])] }
    ]));
    await createWindow();
    app.on('activate', () => { if (!BrowserWindow.getAllWindows().length && !quitting) createWindow().catch(fail); });
  }).catch(fail);
  app.on('window-all-closed', () => { if (process.platform !== 'darwin' || smoke) app.quit(); });
  app.on('before-quit', event => {
    quitting = true;
    if (backend && !cleaned) {
      event.preventDefault();
      backend.stop().finally(() => { cleaned = true; app.exit(process.exitCode || 0); });
    }
  });
}

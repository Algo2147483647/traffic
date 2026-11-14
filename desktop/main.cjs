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
    const result = await window.webContents.executeJavaScript(`(async () => {
      const drawnLabels=[];const originalFillText=CanvasRenderingContext2D.prototype.fillText;CanvasRenderingContext2D.prototype.fillText=function(text,...args){if(drawnLabels.length>2000)drawnLabels.splice(0,1000);drawnLabels.push(String(text));return originalFillText.call(this,text,...args);};
      const errors=[];window.addEventListener('error',event=>errors.push(event.message));
      const wait = ms => new Promise(r => setTimeout(r, ms));
      for (let i = 0; i < 100 && document.querySelector('#map-preset').options[0]?.value !== "preset-1"; i++) await wait(100);
      if (document.querySelector('#map-preset').options[0]?.value !== "preset-1") throw Error('Map selector did not load');
      const coordinates=document.querySelector('#show-coordinates');
      if(!coordinates.checked)coordinates.click();
      const get = p => fetch(p).then(r => r.json());
      const control = body => fetch('/api/control', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }).then(r => { if (!r.ok) throw Error('Control failed'); return r.json(); });
      const preset = ${JSON.stringify(process.env.TRAFFIC_SMOKE_PRESET || '')};
      if (preset) { await control({action:'set_preset',preset_id:preset}); await wait(800); }
      document.querySelector('#settings-toggle').click();
      if (document.querySelector('#settings-panel').hidden) throw Error('Settings did not open');
      await control({action:'pause'});
      const spawn=document.querySelector('#spawn-rate-control'),gap=document.querySelector('#safe-gap-control');
      if(gap.value!=='2.5'||gap.max!=='10'||document.querySelector('#taxi-fleet').value!=='32'||document.querySelector('#taxi-demand').value!=='2')throw Error('Settings defaults not calibrated');
      spawn.value='30';spawn.dispatchEvent(new Event('input'));document.querySelector('#apply-load-button').click();
      for(let i=0;i<50&&(await get('/api/snapshot')).load.spawn_rate_per_second!==0.5;i++)await wait(50);
      if((await get('/api/snapshot')).load.spawn_rate_per_second!==0.5)throw Error('Per-minute demand conversion failed');
      spawn.value='0';spawn.dispatchEvent(new Event('input'));document.querySelector('#apply-load-button').click();
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

      for (const layer of ['ground','bridge','ramp','all']) {
        const select=document.querySelector('#layer-filter');select.value=layer;select.dispatchEvent(new Event('change'));await wait(80);
      }
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
        if(!drawnLabels.includes('X · km')||!drawnLabels.includes('Y · km')||!drawnLabels.includes('6 × 6 km'))throw Error('Missing kilometer rulers');
        document.querySelector('#zoom-in-button').click();await wait(100);
        document.querySelector('#zoom-out-button').click();await wait(100);
        for(const group of city.interchanges){
          const canvas=document.querySelector('#city-canvas'),rect=canvas.getBoundingClientRect(),b=city.bounds;
          const scale=Math.min((rect.width-68)/(b.max_x-b.min_x),(rect.height-68)/(b.max_y-b.min_y));
          const x=rect.left+rect.width/2+(group.center.x-(b.min_x+b.max_x)/2)*scale;
          const y=rect.top+rect.height/2+(group.center.y-(b.min_y+b.max_y)/2)*scale;
          canvas.dispatchEvent(new MouseEvent('click',{clientX:x,clientY:y,bubbles:true}));await wait(100);
          if(document.querySelector('#interchange-inspector').hidden)throw Error('Interchange details did not open: '+group.id);
          const movements=document.querySelector('#interchange-movement');
          if(movements.options.length!==group.routes.length)throw Error('Missing interchange movements: '+group.id);
          for(let i=0;i<group.routes.length;i++){movements.value=String(i);movements.dispatchEvent(new Event('change'));}
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
        document.querySelector('#simulation-mode').value='taxi';document.querySelector('#simulation-mode').dispatchEvent(new Event('change'));
        for(let i=0;i<50&&(await get('/api/snapshot')).mode!=='taxi';i++)await wait(100);
        let snap=await get('/api/snapshot');if(snap.mode!=='taxi'||snap.taxi.vehicles.length!==6)throw Error('Taxi mode did not start');
        await control({action:'pause'});
        for(let i=0;i<400;i++){snap=await control({action:'step',delta:4});if(snap.taxi.completed>=2)break;}
        if(snap.taxi.completed<2||snap.taxi.picked<2)throw Error('Taxi pickup/dropoff failed');
        taxiResult={completed:snap.taxi.completed,picked:snap.taxi.picked};
        const planned=snap.people.filter(p=>p.route.length>1);
        if(!planned.length||planned.some(p=>p.route_entry_lanes?.length!==p.route.length||p.route_exit_lanes?.length!==p.route.length))throw Error('Missing expanded lane paths');
        taxiResult.expandedLaneRoutes=planned.length;
        const liveTime=snap.time,liveCreated=snap.taxi.created;
        document.querySelector('#taxi-fleet-range').value='9';document.querySelector('#taxi-fleet-range').dispatchEvent(new Event('input'));
        document.querySelector('#taxi-demand-range').value='15';document.querySelector('#taxi-demand-range').dispatchEvent(new Event('input'));
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
        await control({action:'set_demand',demand:{trips_per_minute:30,taxi_share:.5}});
        snap=await get('/api/snapshot');if(snap.time!==beforeMixTime||snap.kernel.run!==beforeMixRun)throw Error('Mix reset active run');
        for(let i=0;i<80;i++){snap=await control({action:'step',delta:2});if(snap.people.some(p=>p.journey_id)&&snap.demand.car_trips>0&&snap.demand.taxi_trips>0)break;await wait(20);}
        if(!snap.people.some(p=>p.journey_id)||!snap.taxi.vehicles.length)throw Error('Both modes did not coexist');
        if(snap.demand.generated!==snap.demand.queued+snap.demand.active+snap.demand.completed+snap.demand.cancelled)throw Error('Demand conservation failed');
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
      return { taxiResult, title: document.title, people: after.people.length, time: after.time, presets: document.querySelector('#map-preset').options.length };
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
      configs: app.isPackaged ? path.join(process.resourcesPath, 'configs') : path.join(__dirname, '../configs'),
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

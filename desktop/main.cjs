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
      const wait = ms => new Promise(r => setTimeout(r, ms));
      for (let i = 0; i < 100 && document.querySelector('#map-preset').disabled; i++) await wait(100);
      if (document.querySelector('#map-preset').disabled) throw Error('Map selector did not load');
      const get = p => fetch(p).then(r => r.json());
      const control = body => fetch('/api/control', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }).then(r => { if (!r.ok) throw Error('Control failed'); return r.json(); });
      await control({ action: 'pause' });
      if (!(await get('/api/snapshot')).paused) throw Error('Pause failed');
      await control({ action: 'resume' });
      const before = await get('/api/snapshot'); await wait(400);
      const after = await get('/api/snapshot');
      if (after.time <= before.time) throw Error('Simulation did not advance');
      if (typeof require !== 'undefined') throw Error('Renderer has Node access');
      return { title: document.title, people: after.people.length, time: after.time, presets: document.querySelector('#map-preset').options.length };
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

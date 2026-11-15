const { test } = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { startBackend } = require('../backend.cjs');

test('bundled engine authenticates, simulates, switches presets, streams, and exits with parent', { timeout: 90000 }, async t => {
  const platform = process.platform === 'win32' ? 'win' : process.platform === 'darwin' ? 'mac' : 'linux';
  const backend = await startBackend({ runtime: path.resolve(__dirname, '../runtime', `${platform}-${process.arch}`), configs: path.resolve(__dirname, '../../example') });
  t.after(() => backend.stop());
  const headers = { 'X-Traffic-Token': backend.token, 'Content-Type': 'application/json' };
  const get = async route => {
    const r = await fetch(backend.url + route, { headers });
    assert.equal(r.status, 200); return r.json();
  };
  const control = async body => {
    const r = await fetch(backend.url + '/api/control', { method: 'POST', headers, body: JSON.stringify(body) });
    assert.equal(r.status, 200); return r.json();
  };
  assert.equal((await fetch(backend.url + '/api/health')).status, 401);
  assert.equal((await get('/api/health')).status, 'ok');
  const html = await fetch(backend.url, { headers });
  assert.match(html.headers.get('content-security-policy'), /frame-ancestors 'none'/);
  assert.match(await html.text(), /id="root"/);
  await control({ action: 'pause' });
  const paused = await get('/api/snapshot');
  assert.equal(paused.paused, true);
  const radio=await get('/api/communications');
  assert.equal(radio.layout.deployment.radius_m,256);
  assert.equal(radio.layout.sites.length,700);
  assert.equal(radio.observation.cells.length,2100);
  assert.equal(radio.observation.connected+radio.observation.outage+radio.observation.unsupported,radio.observation.terminals.length);
  assert.equal(radio.observation.cells.reduce((n,c)=>n+c.associated,0),radio.observation.connected);
  assert.equal(radio.observation.time_us,Math.round(paused.time*1e6));
  await control({ action: 'step', delta: 0.05 });
  assert.ok((await get('/api/snapshot')).time > paused.time);
  const presets = await get('/api/presets');
  assert.equal(presets.presets.length, 2);
  await control({ action: 'set_preset', preset_id: 'four-cell-interchanges' });
  assert.equal((await get('/api/city')).name, 'Four-Cell Interchange Network');
  assert.equal((await fetch(backend.url+'/api/communications',{headers})).status,404);
  assert.ok(!presets.presets.some(p=>p.id==='radial-grid-cloverleaf'));
  const abort = new AbortController();
  const stream = await fetch(backend.url + '/api/stream', { headers, signal: abort.signal });
  const { value } = await stream.body.getReader().read();
  assert.match(new TextDecoder().decode(value), /event: city/);
  abort.abort();
  // EOF also occurs when the owning Electron process crashes. No HTTP shutdown
  // endpoint or OS-specific process-tree utilities should be needed.
  backend.child.stdin.end();
  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('Engine survived parent pipe closure')), 5000);
    backend.child.once('exit', () => { clearTimeout(timer); resolve(); });
  });
  await backend.stop();
  assert.notEqual(backend.child.exitCode, null);
  await assert.rejects(fetch(backend.url + '/api/health', { headers }));
});

test('missing engine rejects startup without hanging', async () => {
  await assert.rejects(startBackend({ runtime: path.join(__dirname, 'missing'), configs: __dirname }), /ENOENT/);
});

const { spawn } = require('node:child_process');
const { randomBytes } = require('node:crypto');
const path = require('node:path');
const readline = require('node:readline');

async function startBackend({ runtime, configs, onExit = () => {} }) {
  const token = randomBytes(32).toString('hex');
  const child = spawn(path.join(runtime, process.platform === 'win32' ? 'traffic.exe' : 'traffic'), [
    '-desktop', '-dispatch-dir', path.join(configs, '../experiments/dispatch'), '-config', path.join(configs, 'meridian-city.json'), '-preset-dir', path.join(configs, 'presets')
  ], { windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'], env: { ...process.env, TRAFFIC_DESKTOP_TOKEN: token } });
  let stopping = false;
  let diagnostics = '';
  child.stderr.on('data', chunk => { diagnostics = (diagnostics + chunk).slice(-16000); });
  child.stdin.on('error', () => {});
  const exited = new Promise(resolve => child.once('exit', (code, signal) => {
    resolve();
    if (!stopping) onExit(new Error(`Traffic engine exited (${code ?? signal}).\n${diagnostics}`));
  }));
  async function stop() {
    stopping = true;
    if (child.exitCode !== null || child.signalCode !== null) return;
    child.stdin.end();
    const timer = setTimeout(() => child.kill(), 4000);
    try { await exited; } finally { clearTimeout(timer); }
  }
  try {
    const url = await new Promise((resolve, reject) => {
      const lines = readline.createInterface({ input: child.stdout });
      const timer = setTimeout(() => finish(new Error(`Traffic engine startup timed out.\n${diagnostics}`)), 60000);
      const fail = error => finish(error);
      const earlyExit = () => finish(new Error(`Traffic engine failed to start.\n${diagnostics}`));
      function finish(error, url) {
        clearTimeout(timer); lines.close(); child.off('error', fail); child.off('exit', earlyExit);
        error ? reject(error) : resolve(url);
      }
      child.once('error', fail);
      child.once('exit', earlyExit);
      lines.once('line', line => {
        try {
          const { url } = JSON.parse(line);
          if (!/^http:\/\/127\.0\.0\.1:\d+$/.test(url)) throw new Error('Invalid engine address');
          finish(null, url);
        } catch (error) { finish(error); }
      });
    });
    return { url, token, child, stop };
  } catch (error) {
    stopping = true;
    child.kill();
    throw error;
  }
}
module.exports = { startBackend };

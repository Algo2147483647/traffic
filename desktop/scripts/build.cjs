const path = require('node:path');
const fs = require('node:fs');
const { spawnSync } = require('node:child_process');

function build(platform = process.platform, arch = process.arch) {
  const goos = { win32: 'windows', darwin: 'darwin', linux: 'linux' }[platform];
  const goarch = { x64: 'amd64', arm64: 'arm64' }[arch];
  if (!goos || !goarch) throw new Error(`Unsupported platform: ${platform}/${arch}`);
  const folder = path.resolve(__dirname, '../runtime', `${platform === 'darwin' ? 'mac' : platform === 'win32' ? 'win' : 'linux'}-${arch}`);
  fs.mkdirSync(folder, { recursive: true });
  const project=path.resolve(__dirname,'../..');
  for (const [command,args] of [
    ['go',['run','./scripts/api-types']],
    [process.execPath,[path.join(project,'node_modules/typescript/bin/tsc'),'-p','frontend/tsconfig.json']],
    [process.execPath,[path.join(project,'node_modules/vite/bin/vite.js'),'build','--config','frontend/vite.config.ts']]
  ]) {
    const built=spawnSync(command,args,{cwd:project,stdio:'inherit',env:process.env});
    if(built.error||built.status!==0)throw built.error||new Error(`Frontend build failed (${built.status})`);
  }
  const result = spawnSync('go', ['build', '-trimpath', '-ldflags=-s -w', '-o', path.join(folder, platform === 'win32' ? 'traffic.exe' : 'traffic'), './cmd/traffic'], {
    cwd: path.resolve(__dirname, '../..'), stdio: 'inherit',
    env: { ...process.env, GOOS: goos, GOARCH: goarch, CGO_ENABLED: '0' }
  });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`Go build failed (${result.status})`);
  return folder;
}
module.exports = { build };
if (require.main === module) build(process.argv[2], process.argv[3]);

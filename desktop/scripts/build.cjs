const path = require('node:path');
const fs = require('node:fs');
const { spawnSync } = require('node:child_process');

function build(platform = process.platform, arch = process.arch) {
  const goos = { win32: 'windows', darwin: 'darwin', linux: 'linux' }[platform];
  const goarch = { x64: 'amd64', arm64: 'arm64' }[arch];
  if (!goos || !goarch) throw new Error(`Unsupported platform: ${platform}/${arch}`);
  const folder = path.resolve(__dirname, '../runtime', `${platform === 'darwin' ? 'mac' : platform === 'win32' ? 'win' : 'linux'}-${arch}`);
  fs.mkdirSync(folder, { recursive: true });
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

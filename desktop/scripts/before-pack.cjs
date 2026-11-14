const { build } = require('./build.cjs');
module.exports = async context => {
  const arch = { 1: 'x64', 3: 'arm64' }[context.arch];
  if (!arch) throw new Error(`Unsupported Electron architecture: ${context.arch}`);
  build(context.electronPlatformName, arch);
};

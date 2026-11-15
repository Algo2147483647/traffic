const ts = require("../../node_modules/typescript");
const fs = require("node:fs");
const vm = require("node:vm");
module.exports = function (file, globals = {}) {
  const source = fs.readFileSync(file, "utf8");
  const code = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const context = { ...globals, exports: {} };
  vm.runInNewContext(code, context);
  return context.exports;
};

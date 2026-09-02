import assert from "node:assert/strict";
import test from "node:test";

async function render() {
  const workerUrl = new URL("../dist/server/index.js", import.meta.url);
  workerUrl.searchParams.set("test", `${process.pid}-${Date.now()}`);
  const { default: worker } = await import(workerUrl.href);
  return worker.fetch(new Request("http://localhost/", { headers: { accept: "text/html" } }), {
    ASSETS: { fetch: async () => new Response("Not found", { status: 404 }) },
  }, { waitUntil() {}, passThroughOnException() {} });
}

test("server-renders the Meridian traffic field", async () => {
  const response = await render();
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type") ?? "", /^text\/html\b/i);
  const html = await response.text();
  assert.match(html, /<title>Meridian City Traffic Field<\/title>/i);
  assert.match(html, /Traffic field/);
  assert.doesNotMatch(html, /codex-preview|SkeletonPreview|Your site is taking shape/i);
  assert.match(html, /Full screen/);
  assert.match(html, /Population density/);
  assert.match(html, /Apply traffic load/);
  assert.match(html, /Permitted turn/);
});

test("publishes complete social metadata", async () => {
  const response = await render();
  const html = await response.text();
  assert.match(html, /property="og:title" content="Meridian City Traffic Field"/i);
  assert.match(html, /property="og:image" content="https:\/\/meridian-traffic-field\.algo2147483647\.chatgpt\.site\/og\.png"/i);
  assert.match(html, /name="twitter:card" content="summary_large_image"/i);
});

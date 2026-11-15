const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const load = require('./load-typescript.cjs');

test('map gestures pan, zoom, select visible stations and detach completely', () => {
  const listeners = new Map();
  const state = { showRadio: false, camera: { panX: 0, panY: 0, zoom: 1 }, transform: {baseScale:1}, city: { intersections: [], interchanges: [] }, snapshot: { people: [], metro: { interchanges: [{ id: 'exchange', position: { x: 20, y: 30 }, stations: [1] }], stations: [{ id: 1, elevation_m: -50 }] } } };
  const canvas = {
    addEventListener: (name, fn) => listeners.set(name, fn),
    removeEventListener: (name, fn) => { if (listeners.get(name) === fn) listeners.delete(name); },
    getBoundingClientRect: () => ({ left: 10, top: 15 }),
    classList: { add() {}, remove() {} },
  };
  let selection, zoom, draws = 0;
  const { bindMapGestures } = load(path.resolve(__dirname, '../../frontend/src/interaction/mapGestures.ts'), { require: () => ({ scene: state, select: s => selection = s }) });
  const detach = bindMapGestures(canvas, { draw: () => draws++, screen: p => p, zoomAt: (...args) => zoom = args, nodeVisible: () => true, levelVisible: () => true, personLevel: () => 0 });
  listeners.get('pointerdown')({ pointerId: 1, clientX: 50, clientY: 50 });
  listeners.get('pointermove')({ pointerId: 1, clientX: 70, clientY: 80 });
  listeners.get('pointerup')({ pointerId: 1 });
  assert.equal(state.camera.panX, 20);
  assert.equal(state.camera.panY, 30);
  assert.equal(draws, 1);
  listeners.get('wheel')({ clientX: 30, clientY: 45, deltaY: -100, preventDefault() {} });
  assert.ok(zoom[0] > 1);
  assert.equal(zoom[1], 20);
  listeners.get('click')({ clientX: 30, clientY: 45 });
  assert.equal(selection.type, 'metro-station');
  assert.equal(selection.id, 'exchange');
  state.showRadio=true;state.city.communications={sites:[{id:'BS1',position_m:{x:20,y:30,z:0},mast_height_m:25}]};
  listeners.get('click')({clientX:30,clientY:45});assert.equal(selection.type,'radio-site');assert.equal(selection.id,'BS1');
  detach();
  assert.equal(listeners.size, 0);
});

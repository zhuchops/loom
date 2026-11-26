// Loom's web view front end. Go keeps the editor and lays out its text on a
// character grid; this page draws that grid and the chrome around it, and
// sends keys, text and pointer events back.
'use strict';

const $ = (id) => document.getElementById(id);
// The page's calls go to Go's Page service by name, as the window they come
// from. A call that fails is logged by Go; the page needs no answer.
const pagePrefix = 'github.com/zhuchops/loom/internal/editor.Page.';
const page = new Proxy({}, {
  get: (_, name) => (...args) => {
    const p = window.wails.Call.ByName(pagePrefix + String(name), ...args);
    p.catch(() => {});
    return p;
  },
});
const bridge = () => page;
const isMac = navigator.platform.toUpperCase().includes('MAC');
// Only macOS gives the window glass; elsewhere everything is painted opaque.
if (!isMac) document.documentElement.classList.add('solid');

const grid = $('grid');
const cursorEl = $('cursor');
const input = $('input');
const measureEl = $('measure');

const state = {
  cols: 0, rows: 0, cellW: 8, cellH: 20,
  rowEls: [], rowChars: [],
  look: null, cursor: null, ui: {},
  started: false,
  gridX: 0, gridY: 0, paneW: 0, paneH: 0,
  ground: '', // the theme's text background, which runs leave unpainted
};

// ---- Look -----------------------------------------------------------------

function fontList(...names) {
  return names.filter(Boolean).map((n) => (/^[\w-]+$/.test(n) && !/\s/.test(n) ? n : `"${n}"`)).join(', ');
}

function applyLook(look) {
  state.look = look;
  const root = document.documentElement.style;
  root.setProperty('--code-font', fontList(look.family, look.fallback, 'Menlo', 'DejaVu Sans Mono', 'Consolas', 'monospace'));
  root.setProperty('--code-size', `${look.size}px`);
  root.setProperty('--ui-font', fontList(look.ui, '-apple-system', 'BlinkMacSystemFont', 'Avenir Next', 'Inter', 'Segoe UI', 'sans-serif'));
  root.setProperty('--ui-size', `${look.uiSize || 13}px`);
  document.body.classList.toggle('no-ligatures', !look.ligatures);
  grid.style.fontWeight = look.weight || 400;
  grid.style.letterSpacing = `${look.spacing || 0}px`;
  // Measure a cell in the code font.
  measureEl.style.fontFamily = getComputedStyle(document.documentElement).getPropertyValue('--code-font');
  measureEl.style.fontSize = `${look.size}px`;
  measureEl.style.letterSpacing = `${look.spacing || 0}px`;
  measureEl.textContent = 'M'.repeat(200);
  state.cellW = Math.max(1, measureEl.getBoundingClientRect().width / 200);
  state.cellH = Math.max(1, Math.round(look.size * (look.height || 1.5)));
  root.setProperty('--cell-w', `${state.cellW}px`);
  root.setProperty('--cell-h', `${state.cellH}px`);
  glide.on = !!(look.animations && look.glide);
  for (let y = 0; y < state.rowEls.length; y++) state.rowEls[y].style.top = `${y * state.cellH}px`;
  fit();
}

function applyTheme(theme) {
  const root = document.documentElement.style;
  for (const [k, v] of Object.entries(theme)) {
    if (k === 'scheme') document.documentElement.style.colorScheme = v;
    else root.setProperty(`--${k}`, v);
  }
  if (theme.bg) state.ground = theme.bg;
}

// ---- Grid -----------------------------------------------------------------

// fit sizes the grid in cells and tells the editor when that changes.
function fit() {
  // Where the grid sits in the editor, for the cursor, which is placed
  // without asking the page for its layout on every frame.
  state.gridX = grid.offsetLeft;
  state.gridY = grid.offsetTop;
  const panes = $('panes');
  state.paneW = panes.clientWidth;
  state.paneH = panes.clientHeight;
  // What is drawn over the panes follows the grid.
  if (state.ui && state.ui.panes && typeof renderPanes === 'function') { renderPanes(state.ui.panes); renderMarkdown(state.ui.panes); }
  const r = grid.getBoundingClientRect();
  const cols = Math.max(4, Math.floor(r.width / state.cellW));
  const rows = Math.max(5, Math.floor(r.height / state.cellH));
  if (!state.started) {
    state.started = true;
    state.cols = cols; state.rows = rows;
    bridge().Start(cols, rows);
    return;
  }
  if (cols !== state.cols || rows !== state.rows) {
    state.cols = cols; state.rows = rows;
    bridge().Resize(cols, rows);
  }
}

function ensureRows(n) {
  while (state.rowEls.length < n) {
    const el = document.createElement('div');
    el.className = 'row';
    el.style.top = `${state.rowEls.length * state.cellH}px`;
    grid.appendChild(el);
    state.rowEls.push(el);
    state.rowChars.push([]);
  }
  while (state.rowEls.length > n) {
    state.rowEls.pop().remove();
    state.rowChars.pop();
  }
}

const escapes = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' };
const escapeCell = (t) => t.replace(/[&<>"]/g, (c) => escapes[c]);

// drawRow sets a row as one piece of HTML, which the page parses far faster
// than it builds the same spans one by one; a scroll redraws every row.
function drawRow(y, runs) {
  const el = state.rowEls[y];
  const chars = [];
  const ground = state.ground;
  const W = state.cellW;
  let html = '';
  for (const [text, x, cells, fg, bg, flags, under] of runs || []) {
    let cls = 'run';
    if (flags & 1) cls += ' b';
    if (flags & 2) cls += ' i';
    if (under) cls += ' u';
    if (cells > 1 && [...text].length === 1) cls += ' g';
    let style = `left:${x * W}px;width:${cells * W}px;color:${fg}`;
    if (bg !== ground) style += `;background:${bg}`;
    if (under) style += `;text-decoration-color:${under}`;
    html += `<span class="${cls}" style="${style}">${escapeCell(text)}</span>`;
    if (cells === text.length) for (let i = 0; i < text.length; i++) chars[x + i] = [text[i], flags];
    else chars[x] = [text, flags];
  }
  el.innerHTML = html;
  state.rowChars[y] = chars;
}

// ---- Cursor ---------------------------------------------------------------

let blinkTimer = 0;
let cursorLook = '';
let cursorText = null;

// glide moves the cursor to where it was sent over a few display frames,
// at whatever rate the display runs. A new position starts from where the
// cursor is on screen, so held keys give one smooth motion.
const glide = {
  on: false, x: 0, y: 0, fromX: 0, fromY: 0, toX: 0, toY: 0, start: 0, raf: 0, placed: false,
  duration: 60,
  to(x, y) {
    if (!this.on || !this.placed || (x === this.toX && y === this.toY && !this.raf)) {
      cancelAnimationFrame(this.raf);
      this.raf = 0;
      this.x = this.toX = x; this.y = this.toY = y;
      this.placed = true;
      this.paint();
      return;
    }
    this.fromX = this.x; this.fromY = this.y;
    this.toX = x; this.toY = y;
    this.start = performance.now();
    if (!this.raf) this.raf = requestAnimationFrame((t) => this.step(t));
  },
  step(now) {
    const k = Math.min(1, (now - this.start) / this.duration);
    const e = 1 - Math.pow(1 - k, 3);
    this.x = this.fromX + (this.toX - this.fromX) * e;
    this.y = this.fromY + (this.toY - this.fromY) * e;
    this.paint();
    this.raf = k < 1 ? requestAnimationFrame((t) => this.step(t)) : 0;
  },
  paint() { cursorEl.style.transform = `translate3d(${this.x}px, ${this.y}px, 0)`; },
};

function drawCursor(c) {
  state.cursor = c;
  if (!c) {
    cursorEl.style.display = 'none';
    glide.placed = false;
    return;
  }
  if (cursorEl.style.display !== 'block') cursorEl.style.display = 'block';
  glide.to(state.gridX + c.x * state.cellW, state.gridY + c.y * state.cellH);
  if (composing) placeInput();
  const idle = !document.hasFocus();
  const look = `${c.shape || 'block'}${idle ? ' idle' : ''}`;
  if (look !== cursorLook) {
    cursorLook = look;
    cursorEl.className = look;
  }
  const under = (state.rowChars[c.y] || [])[c.x];
  const text = under && under[0] !== '▏' ? under[0] : '';
  const weight = under && under[1] & 1 ? '700' : '';
  if (text !== cursorText) { cursorText = text; cursorEl.textContent = text; }
  if (cursorEl.style.fontWeight !== weight) cursorEl.style.fontWeight = weight;
  // A moving cursor doesn't blink.
  clearTimeout(blinkTimer);
  cursorEl.classList.remove('blink');
  if (c.blink) blinkTimer = setTimeout(() => cursorEl.classList.add('blink'), 530);
}

// ---- Frames ---------------------------------------------------------------

function applyFrame(f) {
  if (f.theme) applyTheme(f.theme);
  if (f.look) applyLook(f.look);
  if (f.rows !== state.rowEls.length) ensureRows(f.rows);
  if (f.grid) for (const [y, runs] of Object.entries(f.grid)) drawRow(Number(y), runs);
  drawCursor(f.cursor);
  if (f.ui) applyUI(f.ui);
  // Ask for the next frame once this one is on screen: the editor never
  // sends more than the page can draw, so nothing piles up.
  requestAnimationFrame(() => bridge().Drawn());
}

function applyUI(ui) {
  state.ui = ui;
  applyChrome(ui);
  requestAnimationFrame(fit); // the chrome may have changed the room for text
}

// ---- Keyboard -------------------------------------------------------------

const namedKeys = new Set(['Enter', 'Escape', 'Tab', 'Backspace', 'Delete', 'ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown',
  'Home', 'End', 'PageUp', 'PageDown', 'F1', 'F2', 'F3', 'F4', 'F5', 'F6', 'F7', 'F8', 'F9', 'F10', 'F11', 'F12']);
const codeChars = {
  Minus: '-', Equal: '=', BracketLeft: '[', BracketRight: ']', Backslash: '\\', Semicolon: ';', Quote: "'",
  Comma: ',', Period: '.', Slash: '/', Backquote: '`', Space: ' ',
};

// charOf is the Latin character of a physical key, whatever the layout.
function charOf(code) {
  if (code.startsWith('Key')) return code.slice(3).toLowerCase();
  if (code.startsWith('Digit')) return code.slice(5);
  return codeChars[code] || '';
}

function keyEvent(e) {
  return { key: e.key, char: charOf(e.code), ctrl: e.ctrlKey, alt: e.altKey, shift: e.shiftKey, meta: e.metaKey };
}

input.addEventListener('keydown', (e) => {
  if (e.isComposing || e.keyCode === 229) return;
  // A held key's repeats that waited while the page was busy are stale:
  // drop them, so the cursor stops the moment the key is let go.
  if (e.repeat && performance.now() - e.timeStamp > 50) { e.preventDefault(); return; }
  const named = namedKeys.has(e.key);
  const optionChord = e.altKey && (e.code.startsWith('Key') || e.code.startsWith('Digit'));
  if (!named && !e.ctrlKey && !e.metaKey && !optionChord) return; // typing arrives as text
  if (e.metaKey && isMac && !['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Backspace', 'Delete'].includes(e.key)) return; // the menu bar
  e.preventDefault();
  bridge().Key(keyEvent(e));
});

input.addEventListener('input', (e) => {
  if (e.isComposing) return;
  const text = input.value;
  input.value = '';
  if (text) bridge().Text(text);
});

// Input methods show their candidates next to the field that has the
// keyboard, so while one is composing the field sits at the cursor. The rest
// of the time it stays off screen, where the system's writing suggestions,
// which attach to any text field, can't be seen.
let composing = false;
function placeInput() {
  const c = state.cursor;
  if (!composing || !c) {
    input.style.left = '-1000px';
    input.style.top = '0px';
    return;
  }
  const g = grid.getBoundingClientRect();
  input.style.left = `${g.left + c.x * state.cellW}px`;
  input.style.top = `${g.top + c.y * state.cellH}px`;
  input.style.height = `${state.cellH}px`;
}

input.addEventListener('compositionstart', () => { composing = true; placeInput(); });
input.addEventListener('compositionend', (e) => {
  composing = false;
  placeInput();
  input.value = '';
  if (e.data) bridge().Text(e.data);
});

function focusInput() {
  if (document.activeElement !== input) input.focus({ preventScroll: true });
}
window.addEventListener('focus', () => { focusInput(); cursorLook = ''; if (state.cursor) drawCursor(state.cursor); });
window.addEventListener('blur', () => { cursorLook = ''; cursorEl.classList.add('idle'); });

// ---- Pointer on the grid --------------------------------------------------

function cellAt(e) {
  const r = grid.getBoundingClientRect();
  return [Math.floor((e.clientX - r.left) / state.cellW), Math.floor((e.clientY - r.top) / state.cellH)];
}

let held = false;
let lastCell = '';
$('editor').addEventListener('mousedown', (e) => {
  e.preventDefault();
  focusInput();
  const [x, y] = cellAt(e);
  held = e.button === 0;
  lastCell = `${x},${y}`;
  bridge().Mouse('down', x, y, e.button, true, e.shiftKey);
});
window.addEventListener('mousemove', (e) => {
  const [x, y] = cellAt(e);
  const at = `${x},${y}`;
  if (at === lastCell) return;
  lastCell = at;
  const inside = e.target.closest && e.target.closest('#editor');
  if (!held && !inside) return;
  if (typeof tabDrag !== 'undefined' && tabDrag) return;
  bridge().Mouse('move', x, y, 0, held, e.shiftKey);
});
window.addEventListener('mouseup', (e) => {
  if (!held && e.button === 0) return;
  const [x, y] = cellAt(e);
  held = false;
  bridge().Mouse('up', x, y, e.button, false, e.shiftKey);
});
$('editor').addEventListener('contextmenu', (e) => e.preventDefault());

// A swipe is never perfectly straight, so keep to its main axis.
let wheelX = 0;
let wheelY = 0;
$('editor').addEventListener('wheel', (e) => {
  e.preventDefault();
  const scale = e.deltaMode === 1 ? state.cellH : e.deltaMode === 2 ? state.cellH * state.rows : 1;
  if (Math.abs(e.deltaY) >= Math.abs(e.deltaX)) { wheelY += e.deltaY * scale; wheelX = 0; } else { wheelX += e.deltaX * scale; wheelY = 0; }
  const dy = Math.trunc(wheelY / state.cellH);
  const dx = Math.trunc(wheelX / state.cellW / 2);
  if (!dx && !dy) return;
  wheelY -= dy * state.cellH;
  wheelX -= dx * state.cellW * 2;
  const [x, y] = cellAt(e);
  bridge().Wheel(x, y, dx, dy);
}, { passive: false });

// ---- Start ----------------------------------------------------------------

new ResizeObserver(() => { if (state.look) fit(); }).observe(grid);

async function start() {
  try { state.logo = await bridge().Logo(64); } catch (e) { state.logo = ''; }
  window.wails.Events.On('frame', (ev) => applyFrame(ev.data));
  focusInput();
  // The first frame brings the look; until then use the defaults to start.
  applyLook({ size: 13, height: 1.5, spacing: 0, ligatures: true, uiSize: 13 });
}

// The runtime is a module, which runs after these scripts; wait for it.
(function wait() {
  if (window.wails) start();
  else setTimeout(wait, 5);
})();

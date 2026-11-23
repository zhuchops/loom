// The interface's icons, drawn on a 16 by 16 grid.
'use strict';

// Errors in the page go to Loom's log, as there are no developer tools.
window.addEventListener('error', (e) => {
  try { window.go.editor.Bridge.Log(`${e.message} at ${e.filename}:${e.lineno}:${e.colno}`); } catch (_) { /* not ready */ }
});

const iconPaths = {
  search: 'C7,7,4.5 L10.2,10.2,14,14',
  folder: 'L2,12.8,2,3.5,6,3.5,7.6,5.3,14,5.3,14,12.8,2,12.8,2,11',
  folderOpen: 'L2,12.8,2,3.5,6,3.5,7.6,5.3,13,5.3,13,7.5 L2,12.8,4.2,7.5,15,7.5,12.8,12.8,2,12.8,2.5,11.6',
  file: 'L9.5,2,4,2,4,14,12,14,12,4.5,9.5,2,9.5,4.5,12,4.5',
  newFile: 'L8,14,3,14,3,2,8.5,2,11,4.5,11,7 L12,9,12,15 L9,12,15,12',
  newFolder: 'L8,12.8,1.5,12.8,1.5,3.5,5.5,3.5,7,5.3,13.5,5.3,13.5,7.5 L12.5,9,12.5,15 L9.5,12,15.5,12',
  branch: 'C4.5,3.5,1.8 C4.5,12.5,1.8 C11.5,5.5,1.8 L4.5,5.3,4.5,10.7 L11.5,7.3,11,8.6,9.5,9.4,4.5,9.8',
  sidebar: 'L2,4,2,13,14,13,14,3,2,3,2,4 L6.3,3,6.3,13',
  settings: 'L2,4.5,8,4.5 L12,4.5,14,4.5 C10,4.5,2 L2,11.5,4,11.5 L8,11.5,14,11.5 C6,11.5,2',
  close: 'L4.2,4.2,11.8,11.8 L11.8,4.2,4.2,11.8',
  command: 'L2,4,2,13,14,13,14,3,2,3,2,4 L4.8,6,7,8,4.8,10 L8.5,10,11.2,10',
  tabs: 'L2,6,2,14,11,14,11,5,2,5,2,6 L5,5,5,2,14,2,14,11,11,11',
  check: 'L3,8.4,6.5,11.8,13,4.5',
  error: 'C8,8,6 L5.8,5.8,10.2,10.2 L10.2,5.8,5.8,10.2',
  warning: 'L8,2.5,14.3,13.3,1.7,13.3,8,2.5,8.6,3.5 L8,6.2,8,9.4 L8,11,8,12.2',
  info: 'C8,8,6 L8,7.2,8,11.5 L8,4.4,8,5.7',
  plus: 'L8,3.5,8,12.5 L3.5,8,12.5,8',
  minus: 'L3.5,8,12.5,8',
  refresh: 'L13,8,12.3,10.8,10,12.6,7,12.8,4.2,11,3,8,4.2,5,7,3.2,10,3.5,12.6,5.6 L13,2.5,13,6,9.5,6',
  right: 'L6,4,10,8,6,12',
  down: 'L4,6,8,10,12,6',
  up: 'L4,10,8,6,12,10',
};

const iconCache = {};

// icon returns the SVG markup of a named icon at size pixels.
function icon(name, size = 15) {
  const key = `${name}:${size}`;
  if (iconCache[key]) return iconCache[key];
  let body = '';
  for (const part of (iconPaths[name] || '').split(' ')) {
    if (!part) continue;
    const nums = part.slice(1).split(',').map(Number);
    if (part[0] === 'C') {
      body += `<circle cx="${nums[0]}" cy="${nums[1]}" r="${nums[2]}"/>`;
    } else {
      const pts = [];
      for (let i = 0; i < nums.length; i += 2) pts.push(`${nums[i]},${nums[i + 1]}`);
      body += `<polyline points="${pts.join(' ')}"/>`;
    }
  }
  const svg = `<svg class="icon" width="${size}" height="${size}" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round">${body}</svg>`;
  iconCache[key] = svg;
  return svg;
}

// fileIcon draws a file's monogram, or the file glyph when it has none.
function fileIcon(mark) {
  const [text, ink] = mark || ['', ''];
  if (!text) return `<span class="ficon" style="color:var(--dim)">${icon('file', 15)}</span>`;
  return `<span class="ficon mono${text.length > 2 ? ' small' : ''}" style="color:${ink}">${escapeHTML(text)}</span>`;
}

// ltr keeps a path left to right inside an element that is cut on the left
// (direction: rtl), so its slashes stay where they are.
function ltr(s) { return `\u200e${escapeHTML(s)}\u200e`; }

function escapeHTML(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

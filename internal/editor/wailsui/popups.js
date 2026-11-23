// Popups over the text: pickers, the find bar, completion, signature help,
// hover, which-key, toasts and notification details. They are glass: they
// blur what lies under them.
'use strict';

const ROW = 32; // picker row height

function layer(id, make) {
  let el = $(id);
  if (!el) {
    el = make();
    el.id = id;
    $('layers').appendChild(el);
  }
  return el;
}

function drop(id) {
  const el = $(id);
  if (el) el.remove();
}

// ---- Keycaps --------------------------------------------------------------

function chip(s) { return `<span class="chip">${escapeHTML(s)}</span>`; }

// keysHTML draws a key hint ("Space f  :files") as keycaps.
function keysHTML(hint) {
  return hint.trim().split('  ').filter((g) => g.trim()).map((g) => {
    g = g.trim();
    if (g.length > 1 && g[0] === ':') return `<span class="excmd">${escapeHTML(g)}</span>`;
    return g.split(/\s+/).map(chip).join('<span class="gap"></span>');
  }).join('<span class="sep"></span>');
}

function hintsHTML(pairs) {
  let html = '';
  for (let i = 0; i + 1 < pairs.length; i += 2) html += `<span class="hint">${chip(pairs[i])}<span>${escapeHTML(pairs[i + 1])}</span></span>`;
  return html;
}

// ---- Text fields ----------------------------------------------------------

const runes = (s) => [...s].length;

// fieldHTML draws a field's text around its caret and selection.
function fieldHTML(f, placeholder, active = true) {
  if (!f.before && !f.sel && !f.after) {
    return (active ? '<span class="caret"></span>' : '') + `<span class="placeholder">${escapeHTML(placeholder || '')}</span>`;
  }
  const caret = active ? '<span class="caret"></span>' : '';
  const sel = f.sel ? `<span class="fsel${active ? '' : ' idle'}" data-part="1">${escapeHTML(f.sel)}</span>` : '';
  return `<span data-part="0">${escapeHTML(f.before)}</span>` + (f.caretFirst ? caret + sel : sel + caret) + `<span data-part="2">${escapeHTML(f.after)}</span>`;
}

// fieldIndex is the rune index under the pointer in a field drawn by fieldHTML.
function fieldIndex(el, f, e) {
  const parts = [f.before, f.sel || '', f.after];
  const range = document.caretRangeFromPoint(e.clientX, e.clientY);
  if (!range || !el.contains(range.startContainer)) {
    const r = el.getBoundingClientRect();
    return e.clientX < r.left + 20 ? 0 : runes(parts.join(''));
  }
  const span = range.startContainer.parentElement && range.startContainer.parentElement.closest('[data-part]');
  if (!span) return runes(parts.join(''));
  const part = Number(span.dataset.part);
  let at = 0;
  for (let i = 0; i < part; i++) at += runes(parts[i]);
  return at + runes(range.startContainer.textContent.slice(0, range.startOffset));
}

// wireField makes a field take presses, Shift-clicks and drags.
function wireField(el, which, getField) {
  el.addEventListener('mousedown', (e) => {
    e.preventDefault();
    e.stopPropagation();
    focusInput();
    act('field', fieldIndex(el, getField(), e), `${which}-${e.shiftKey ? 'shift' : 'press'}`);
    const move = (ev) => act('field', fieldIndex(el, getField(), ev), `${which}-drag`);
    const up = () => { window.removeEventListener('mousemove', move); window.removeEventListener('mouseup', up); };
    window.addEventListener('mousemove', move);
    window.addEventListener('mouseup', up);
  });
}

// ---- Pickers --------------------------------------------------------------

function highlightLabel(label, hits) {
  if (!hits) return escapeHTML(label);
  const r = [...label];
  let html = '';
  let at = 0;
  for (const [a, b] of hits) {
    html += escapeHTML(r.slice(at, a).join('')) + `<b>${escapeHTML(r.slice(a, b).join(''))}</b>`;
    at = b;
  }
  return html + escapeHTML(r.slice(at).join(''));
}

function rowHTML(row) {
  let html = '';
  const ic = row.icon;
  if (ic) {
    if (ic[0] === 'logo') html += `<img class="logo" src="${state.logo || ''}" alt="">`;
    else if (ic[0] === 'file') html += fileIcon([ic[1], ic[2]]);
    else html += `<span class="ficon" style="color:${ic[2]}">${icon(ic[1], 14)}</span>`;
  }
  html += `<span class="mlabel">${highlightLabel(row.label, row.hits)}</span>`;
  if (row.detail) html += `<span class="mdetail">${ltr(row.detail)}</span>`;
  const h = row.hint;
  if (h) {
    if (h[0] === 'check') html += `<span class="mhint check">${icon('check', 14)}</span>`;
    else if (h[0] === 'toggle') html += `<span class="mhint toggle${h[1] === 'on' ? ' on' : ''}"><span></span></span>`;
    else if (h[0] === 'keys') html += `<span class="mhint keys">${keysHTML(h[1])}</span>`;
    else html += `<span class="mhint text">${escapeHTML(h[1])}</span>`;
  }
  return html;
}

function visibleRows(m) {
  const top = (state.titleH || 38) + Math.min(64, window.innerHeight / 12);
  const avail = window.innerHeight - top - 40 - 34 - 48 - 34;
  if (m.preview) return Math.max(1, Math.floor((Math.min(420, avail) - 12) / ROW));
  return Math.max(1, Math.min(12, Math.floor((avail - 12) / ROW)));
}

let menuRowsSent = 0;
function renderMenu(m) {
  if (!changed('menu', m)) return;
  if (!m) { drop('menu'); return; }
  const scrim = layer('menu', () => {
    const s = h('div', 'scrim top');
    s.addEventListener('mousedown', (e) => { if (e.target === s) { e.preventDefault(); act('menu-close'); } });
    return s;
  });
  const rows = visibleRows(m);
  if (rows !== menuRowsSent) { menuRowsSent = rows; act('menu-rows', rows); }
  const wide = m.preview && window.innerWidth - 32 >= 700;
  scrim.replaceChildren();
  const card = h('div', 'card glass picker' + (wide ? ' wide' : ''));
  card.addEventListener('mousedown', (e) => { if (!e.target.closest('.field')) { e.preventDefault(); focusInput(); } });
  const head = h('div', 'phead', `<span>${escapeHTML(m.title)}</span>` + (m.count ? `<span class="pcount">${escapeHTML(m.count)}</span>` : ''));
  card.appendChild(head);
  if (m.confirmation) {
    card.appendChild(h('div', 'pconfirm', 'Review the changes, then press Enter to apply them.'));
  } else {
    const field = h('div', 'field pfield', (m.prompt ? '' : `<span class="ficon">${icon('search', 14)}</span>`) + `<span class="ftext">${fieldHTML(m.field, m.placeholder)}</span>`);
    wireField(field.querySelector('.ftext'), 'menu', () => m.field);
    card.appendChild(field);
  }
  const body = !m.prompt || m.total > 0;
  if (body) {
    const main = h('div', 'pbody');
    const list = h('div', 'plist');
    if (m.empty !== undefined && m.total === 0) {
      list.appendChild(h('div', 'pempty', escapeHTML(m.empty)));
      list.style.height = '72px';
    } else {
      const shown = Math.min(rows, m.total);
      list.style.height = `${(m.preview ? Math.min(420, rows * ROW + 12) : shown * ROW + 12)}px`;
      m.rows.forEach((row, k) => {
        const i = m.from + k;
        if (i < m.top || i >= m.top + rows) return;
        const el = h('div', 'prow' + (i === m.selected ? ' sel' : ''), rowHTML(row));
        el.style.top = `${6 + (i - m.top) * ROW}px`;
        el.addEventListener('click', () => act('menu-pick', i));
        list.appendChild(el);
      });
      if (m.total > rows) {
        const track = h('div', 'pscroll');
        const thumb = h('div', 'thumb');
        const span = rows * ROW;
        thumb.style.height = `${Math.max(24, span * rows / m.total)}px`;
        thumb.style.top = `${(span - Math.max(24, span * rows / m.total)) * Math.min(m.top, m.total - rows) / (m.total - rows) + 6}px`;
        track.appendChild(thumb);
        list.appendChild(track);
      }
      let wheel = 0;
      list.addEventListener('wheel', (e) => {
        e.preventDefault();
        wheel += e.deltaY;
        const dy = Math.trunc(wheel / ROW);
        if (dy) { wheel -= dy * ROW; act('menu-scroll', dy); }
      }, { passive: false });
    }
    main.appendChild(list);
    if (m.preview && wide) main.appendChild(previewEl(m.preview));
    card.appendChild(main);
  }
  card.appendChild(h('div', 'pfoot', hintsHTML(m.hints)));
  scrim.appendChild(card);
}

function codeHTML(lines) {
  const width = String(lines.length ? lines[lines.length - 1][0] : 1).length;
  return lines.map(([n, cur, diag, runs]) => {
    const text = (runs || []).map(([t, c]) => `<span style="color:${c}">${escapeHTML(t)}</span>`).join('');
    const style = diag ? ` style="--diag:${diag}"` : '';
    return `<div class="cline${cur ? ' cur' : ''}${diag ? ' diag' : ''}"${style}><span class="num" style="width:${width}ch">${n}</span>${text}</div>`;
  }).join('');
}

function previewEl(p) {
  const el = h('div', 'ppreview');
  if (p.empty) {
    el.appendChild(h('div', 'pempty', escapeHTML(p.empty)));
    return el;
  }
  el.appendChild(h('div', 'ptitle', ltr(p.title)));
  const body = h('div', 'pcode');
  if (p.message) body.appendChild(h('div', 'pmessage', escapeHTML(p.message)));
  if (p.code) body.appendChild(h('div', 'code', codeHTML(p.code)));
  let wheel = 0;
  body.addEventListener('wheel', (e) => {
    if (!p.code) return;
    e.preventDefault();
    wheel += e.deltaY;
    const dy = Math.trunc(wheel / 19);
    if (dy) { wheel -= dy * 19; act('preview-scroll', dy); }
  }, { passive: false });
  el.appendChild(body);
  return el;
}

// ---- Find bar -------------------------------------------------------------

function renderFind(f) {
  if (!changed('find', f)) return;
  if (!f) { drop('find'); return; }
  const el = layer('find', () => { const c = h('div', 'card glass findbar'); chromeDown(c); return c; });
  const ed = $('editor').getBoundingClientRect();
  el.style.top = `${ed.top + 8}px`;
  el.style.right = `${window.innerWidth - ed.right + 14}px`;
  el.replaceChildren();
  const row1 = h('div', 'frow');
  row1.appendChild(button('tbtn', icon(f.replaceOn ? 'down' : 'right', 13), () => act('bar', -1, 'toggle-replace'), 'Toggle Replace'));
  const find = h('div', 'field ffield' + (f.field === 0 ? ' active' : ''), `<span class="ftext">${fieldHTML(f.find, 'Find', f.field === 0)}</span>`);
  wireField(find.querySelector('.ftext'), 'find', () => f.find);
  const opts = h('span', 'fopts');
  if (f.count) opts.appendChild(h('span', 'fcount' + (f.bad ? ' bad' : ''), escapeHTML(f.count)));
  for (const [label, action, on] of [['Aa', 'case', f.case], ['ab', 'word', f.word], ['.*', 'regex', f.regex]]) {
    opts.appendChild(button('fopt' + (on ? ' on' : ''), label, () => act('bar', -1, action)));
  }
  find.appendChild(opts);
  row1.appendChild(find);
  row1.appendChild(button('tbtn', icon('up', 14), () => act('bar', -1, 'prev'), 'Previous'));
  row1.appendChild(button('tbtn', icon('down', 14), () => act('bar', -1, 'next'), 'Next'));
  row1.appendChild(button('tbtn', icon('close', 13), () => act('bar', -1, 'close'), 'Close'));
  el.appendChild(row1);
  if (f.replaceOn) {
    const row2 = h('div', 'frow');
    row2.appendChild(h('span', 'tspace'));
    const rep = h('div', 'field ffield' + (f.field === 1 ? ' active' : ''), `<span class="ftext">${fieldHTML(f.replace, 'Replace', f.field === 1)}</span>`);
    wireField(rep.querySelector('.ftext'), 'replace', () => f.replace);
    row2.appendChild(rep);
    row2.appendChild(button('tbutton', 'Replace', () => act('bar', -1, 'replace')));
    row2.appendChild(button('tbutton', 'All', () => act('bar', -1, 'all')));
    el.appendChild(row2);
  }
}

// ---- Near the cursor ------------------------------------------------------

// beside places a popup of size w by h below cell (x, y), or above it when
// there is more room there. It returns [left, top, above].
function beside(x, y, w, ht, preferAbove) {
  const g = grid.getBoundingClientRect();
  const ed = $('editor').getBoundingClientRect();
  const px = g.left + x * state.cellW - 14;
  const top = ed.top + 2;
  const bottom = ed.bottom - 2;
  const below = g.top + (y + 1) * state.cellH + 2;
  const above = g.top + y * state.cellH - 2;
  let onTop = below + ht > bottom && above - top > bottom - below;
  if (preferAbove && above - ht >= top) onTop = true;
  const left = Math.max(6, Math.min(px, window.innerWidth - w - 6));
  if (onTop) return [left, Math.max(top, above - ht), true];
  return [left, below, false];
}

function proseHTML(text) {
  let html = '';
  let code = false;
  let block = [];
  for (const raw of text.trim().split('\n')) {
    const t = raw.trim();
    if (t.startsWith('```')) {
      if (code) { html += `<pre>${escapeHTML(block.join('\n'))}</pre>`; block = []; }
      code = !code;
      continue;
    }
    if (code) { block.push(raw); continue; }
    if (t === '---') html += '<hr>';
    else if (t.startsWith('#')) html += `<p class="strong">${escapeHTML(t.replace(/^#+\s*/, ''))}</p>`;
    else if (t === '') html += '<p class="gap"></p>';
    else html += `<p>${escapeHTML(raw)}</p>`;
  }
  if (block.length) html += `<pre>${escapeHTML(block.join('\n'))}</pre>`;
  return html;
}

function renderCompletion(c) {
  if (!changed('completion', c)) return;
  drop('completion'); drop('cdocs'); drop('signature');
  if (!c) return;
  let listAbove = false;
  if (c.items) {
    const el = layer('completion', () => h('div', 'card glass completion'));
    el.innerHTML = c.items.map(([label, detail], k) => {
      const i = c.first + k;
      return `<div class="crow${i === c.selected ? ' sel' : ''}" data-i="${i}"><span class="clabel">${escapeHTML(label)}</span><span class="cdetail">${escapeHTML(detail || '')}</span></div>`;
    }).join('');
    el.querySelectorAll('.crow').forEach((row) => row.addEventListener('mousedown', (e) => { e.preventDefault(); act('complete', Number(row.dataset.i)); }));
    let wheel = 0;
    el.addEventListener('wheel', (e) => { e.preventDefault(); wheel += e.deltaY; const d = Math.trunc(wheel / 27); if (d) { wheel -= d * 27; act('complete-scroll', d); } }, { passive: false });
    const w = el.offsetWidth;
    const ht = el.offsetHeight;
    const [left, top, above] = beside(c.x, c.y, w, ht, false);
    listAbove = above;
    el.style.left = `${left}px`;
    el.style.top = `${top}px`;
    if (c.docs) {
      const d = layer('cdocs', () => h('div', 'card glass prose cdocs'));
      d.innerHTML = proseHTML(c.docs);
      let x = left + w + 6;
      if (x + 330 > window.innerWidth) x = left - 6 - 330;
      d.style.left = `${Math.max(6, x)}px`;
      d.style.top = `${top}px`;
    }
  }
  if (c.signature) {
    const [before, active, after] = c.signature;
    const el = layer('signature', () => h('div', 'card glass signature'));
    el.innerHTML = `<span class="dim">${escapeHTML(before)}</span><span class="active">${escapeHTML(active)}</span><span class="dim">${escapeHTML(after)}</span>`;
    const [left, top] = beside(c.x, c.y, el.offsetWidth, el.offsetHeight, !listAbove);
    el.style.left = `${left}px`;
    el.style.top = `${top}px`;
  }
}

function renderHover(hv) {
  if (!changed('hover', hv)) return;
  drop('hover');
  if (!hv) return;
  const el = layer('hover', () => h('div', 'card glass prose hoverbox'));
  el.innerHTML = (hv.title ? `<div class="ptitle caps">${escapeHTML(hv.title)}</div>` : '') + proseHTML(hv.text);
  el.addEventListener('mousedown', (e) => { e.preventDefault(); focusInput(); });
  const [left, top] = beside(hv.x, hv.y, el.offsetWidth, el.offsetHeight, false);
  el.style.left = `${left}px`;
  el.style.top = `${top}px`;
}

// ---- Which-key, toasts, details --------------------------------------------

function renderWhichKey(w) {
  if (!changed('whichkey', w)) return;
  drop('whichkey');
  if (!w) return;
  const el = layer('whichkey', () => h('div', 'card glass whichkey'));
  const cols = w.entries.length <= 8 ? 1 : Math.max(1, Math.min(4, Math.floor(($('editor').offsetWidth - 40) / 250)));
  el.style.setProperty('--cols', cols);
  el.classList.toggle('single', cols === 1);
  el.innerHTML = `<div class="ptitle caps">${escapeHTML(w.title)}</div><div class="wgrid">` +
    w.entries.map(([k, t]) => `<div class="wentry">${chip(k)}<span>${escapeHTML(t)}</span></div>`).join('') + '</div>';
}

let toastShown = 0;
let toastTimer = 0;
function renderToast(t) {
  if (!changed('toast', t)) return;
  if (!t) { drop('toast'); return; }
  if (t.id === toastShown) return;
  toastShown = t.id;
  const el = layer('toast', () => h('div', 'card glass toast'));
  el.className = 'card glass toast';
  el.innerHTML = `<span class="ticon" style="color:${t.bad ? 'var(--red)' : 'var(--accent)'}">${icon(t.bad ? 'error' : 'info', 15)}</span><span class="ttext">${escapeHTML(t.text)}</span>`;
  el.appendChild(button('tbtn small', icon('close', 12), () => drop('toast')));
  el.onmousedown = (e) => { e.preventDefault(); if (!e.target.closest('button')) { drop('toast'); act('notifications'); } };
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => drop('toast'), t.stay);
}

function renderDetail(d) {
  if (!changed('detail', d)) return;
  if (!d) { drop('detail'); return; }
  const scrim = layer('detail', () => {
    const s = h('div', 'scrim');
    s.addEventListener('mousedown', (e) => { if (e.target === s) { e.preventDefault(); act('detail-close'); } });
    return s;
  });
  scrim.replaceChildren();
  const card = h('div', 'card glass detail');
  card.appendChild(h('div', 'dhead', escapeHTML(d.title)));
  const body = h('div', 'dtext', escapeHTML(d.text));
  card.appendChild(body);
  card.appendChild(h('div', 'pfoot', hintsHTML(d.hints)));
  scrim.appendChild(card);
  body.scrollTop = d.scroll * 21;
  card.addEventListener('mousedown', (e) => { e.preventDefault(); focusInput(); });
}

function applyPopups(ui) {
  renderMenu(ui.menu);
  renderFind(ui.find);
  renderCompletion(ui.completion);
  renderHover(ui.hover);
  renderWhichKey(ui.whichkey);
  renderToast(ui.toast);
  renderDetail(ui.detail);
}

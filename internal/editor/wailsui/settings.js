// The settings window, and the installed fonts it offers.
'use strict';

// ---- Fonts ------------------------------------------------------------------

const monoCandidates = ['Menlo', 'Monaco', 'SF Mono', 'Courier New', 'Andale Mono', 'PT Mono', 'JetBrains Mono', 'JetBrainsMono Nerd Font',
  'Fira Code', 'FiraCode Nerd Font', 'Fira Mono', 'Source Code Pro', 'Cascadia Code', 'Cascadia Mono', 'Hack', 'Hack Nerd Font', 'Iosevka',
  'Iosevka Term', 'IBM Plex Mono', 'Roboto Mono', 'Ubuntu Mono', 'DejaVu Sans Mono', 'Liberation Mono', 'Noto Sans Mono', 'Inconsolata',
  'Victor Mono', 'Monaspace Neon', 'Monaspace Argon', 'Monaspace Xenon', 'Monaspace Radon', 'Monaspace Krypton', 'Geist Mono', 'Berkeley Mono',
  'Operator Mono', 'Input Mono', 'Comic Mono', 'Anonymous Pro', 'Space Mono', 'Red Hat Mono', 'Commit Mono', 'Maple Mono', 'Lilex',
  'Recursive Mono', 'Intel One Mono', 'Martian Mono', 'Hasklig', 'Consolas', 'Lucida Console', 'Droid Sans Mono', 'Meslo LG S', 'MesloLGS NF',
  'Go Mono', 'Azeret Mono', 'Sometype Mono', 'Spline Sans Mono', 'Atkinson Hyperlegible Mono', '0xProto', 'Monoid', 'Fantasque Sans Mono'];
const uiCandidates = ['SF Pro Text', 'SF Pro Display', 'Helvetica Neue', 'Helvetica', 'Avenir Next', 'Avenir', 'Inter', 'Roboto', 'Open Sans',
  'Lato', 'Ubuntu', 'Cantarell', 'Noto Sans', 'DejaVu Sans', 'Segoe UI', 'Arial', 'Verdana', 'Gill Sans', 'Futura', 'Optima', 'Lucida Grande',
  'IBM Plex Sans', 'Source Sans 3', 'Source Sans Pro', 'Fira Sans', 'Geist', 'Manrope', 'Nunito', 'Poppins', 'Montserrat', 'Work Sans',
  'Charter', 'Georgia', 'Iowan Old Style', 'Palatino', 'Baskerville', 'Rubik', 'Atkinson Hyperlegible', 'Public Sans', 'Libre Franklin'];

// installedFonts checks which candidates the system has: an installed family
// changes the width of a sample compared with the generic fallbacks.
function installedFonts() {
  const ctx = document.createElement('canvas').getContext('2d');
  const sample = 'mmmmmmmmmmlli1WQ@#';
  const width = (font) => { ctx.font = `72px ${font}`; return ctx.measureText(sample).width; };
  const bases = ['monospace', 'serif', 'sans-serif'].map((g) => [g, width(g)]);
  const found = [];
  const seen = new Set();
  for (const name of [...monoCandidates, ...uiCandidates]) {
    if (seen.has(name)) continue;
    seen.add(name);
    if (!bases.some(([g, w]) => width(`"${name}", ${g}`) !== w)) continue;
    ctx.font = `72px "${name}", serif`;
    const mono = Math.abs(ctx.measureText('iiiiiiii').width - ctx.measureText('WWWWWWWW').width) < 0.5;
    found.push({ name, mono });
  }
  return found.sort((a, b) => a.name.localeCompare(b.name));
}

let fontsSent = false;
function sendFonts() {
  if (fontsSent) return;
  fontsSent = true;
  act('settings-fonts', 0, JSON.stringify(installedFonts()));
}

// ---- The window -------------------------------------------------------------

function segmentsHTML(labels, on) {
  return `<span class="segments">${labels.map((l, k) => `<button tabindex="-1" class="seg${k === on ? ' on' : ''}" data-k="${k}">${escapeHTML(l)}</button>`).join('')}</span>`;
}

function settingRowEl(r) {
  const el = h('div', `srow ${r.k}` + (r.sel ? ' sel' : ''));
  el.dataset.i = r.i;
  const dot = '<span class="udot"></span>';
  switch (r.k) {
    case 'heading':
      el.innerHTML = `<span class="caption">${escapeHTML(r.label)}</span>`;
      return el;
    case 'note':
      el.innerHTML = `<span class="note">${escapeHTML(r.label)}</span>`;
      return el;
    case 'preview':
      el.innerHTML = `<div class="spreview code">${r.lines.map((parts) => `<div>${parts.map(([t, c, it]) => `<span style="color:${c}${it ? ';font-style:italic' : ''}">${escapeHTML(t)}</span>`).join('')}</div>`).join('')}</div>`;
      return el;
    case 'tool':
      el.innerHTML = `<span class="tlabel">${escapeHTML(r.label)}</span><span class="tdetail${r.ok ? ' ok' : ''}">${escapeHTML(r.detail)}</span>`;
      return el;
    case 'language':
      el.innerHTML = `<span class="chev">${icon(r.open ? 'down' : 'right', 11)}</span><span class="stitle">${escapeHTML(r.name)}</span><span class="ssummary${r.changed ? ' changed' : ''}">${escapeHTML(r.summary)}</span>`;
      el.addEventListener('click', () => act('settings-row', r.i));
      return el;
    case 'langValue':
    case 'modes':
      el.innerHTML = `<span class="stitle${r.k === 'langValue' ? ' indent' : ''}">${escapeHTML(r.label)}</span>${segmentsHTML(r.labels, r.on)}`;
      break;
    case 'binding':
      el.innerHTML = `<span class="stitle">${escapeHTML(r.label)}${r.changed ? dot : ''}</span>` +
        (r.changed ? '<button tabindex="-1" class="sreset">Reset</button>' : '') +
        `<span class="mhint keys">${r.keys ? keysHTML(r.keys) : '<span class="faint">—</span>'}</span>`;
      el.addEventListener('click', (e) => { if (!e.target.closest('.sreset')) act('settings-row', r.i); });
      break;
    case 'theme': {
      el.innerHTML = `<div class="shead"><span class="stext"><span class="stitle${r.locked ? ' faint' : ''}">${escapeHTML(r.title)}</span><span class="shelp">${escapeHTML(r.help)}</span></span>` +
        (r.locked ? chip('Project') : r.user ? '<button tabindex="-1" class="sreset show">Reset</button>' : '') + '</div>';
      const grid = h('div', 'themes');
      for (const t of r.themes) {
        const card = h('div', 'swatch' + (t.current ? ' current' : ''));
        card.style.background = t.bg;
        let lines = '';
        t.lines.forEach((line, k) => {
          lines += `<div class="sline" style="top:${14 + k * 14}px;left:${34 + (k >= 2 ? 10 : 0)}px">${line.map(([w, c]) => `<span style="width:${w}px;background:${c}"></span>`).join('')}</div>`;
        });
        card.innerHTML = `<div class="sbar" style="background:${t.side}">${'<span></span>'.repeat(4).replaceAll('<span>', `<span style="background:${t.bar}">`)}</div>${lines}` +
          `<div class="sname" style="background:${t.side};color:${t.ink}">${escapeHTML(t.title)}</div>` + (t.current ? `<span class="scheck">${icon('check', 11)}</span>` : '');
        card.addEventListener('click', () => act('settings-theme', r.i, t.name));
        grid.appendChild(card);
      }
      el.appendChild(grid);
      break;
    }
    case 'setting': {
      const muted = r.locked || !r.live;
      let control = '';
      if (r.control === 'switch') control = `<span class="mhint toggle${r.on ? ' on' : ''}"><span></span></span>`;
      else if (r.control === 'choice') control = segmentsHTML(r.labels, r.on);
      else if (r.control === 'number') {
        control = `<span class="stepper${r.wide ? ' wide' : ''}"><button tabindex="-1" class="step" data-by="-1"${r.less ? '' : ' disabled'}>${icon('minus', 12)}</button>` +
          `<span class="value">${escapeHTML(r.value)}</span><button tabindex="-1" class="step" data-by="1"${r.more ? '' : ' disabled'}>${icon('plus', 12)}</button></span>`;
      } else if (r.control === 'font') {
        const style = r.family ? ` style="font-family:${fontList(r.family)}"` : '';
        control = `<button tabindex="-1" class="fontbox${r.default ? ' dim' : ''}${r.missing ? ' missing' : ''}"><span${style}>${escapeHTML(r.value)}</span>${icon('down', 11)}</button>`;
      }
      el.innerHTML = `<span class="stext"><span class="stitle${muted ? ' faint' : ''}">${escapeHTML(r.title)}${r.user && !r.locked ? dot : ''}</span>` +
        (r.help ? `<span class="shelp">${escapeHTML(r.help)}</span>` : '') + '</span>' +
        (r.locked ? chip('Project') : r.user ? '<button tabindex="-1" class="sreset">Reset</button>' : '') + control;
      if (r.control === 'switch' || r.control === 'font') el.addEventListener('click', (e) => { if (!e.target.closest('.sreset')) act('settings-row', r.i); });
      el.querySelectorAll('.step').forEach((b) => b.addEventListener('click', () => act('settings-step', r.i, b.dataset.by)));
      break;
    }
    default:
      break;
  }
  el.querySelectorAll('.seg').forEach((b) => b.addEventListener('click', (e) => { e.stopPropagation(); act('settings-choose', r.i, b.dataset.k); }));
  el.querySelectorAll('.sreset').forEach((b) => b.addEventListener('click', (e) => { e.stopPropagation(); act('settings-reset', r.i); }));
  return el;
}

let settingsScroll = 0;
function renderSettings(s) {
  if (!changed('settings', s)) return;
  if (!s) { drop('settings'); settingsScroll = 0; return; }
  sendFonts();
  const scrim = layer('settings', () => {
    const el = h('div', 'scrim light');
    el.addEventListener('mousedown', (e) => { if (e.target === el) { e.preventDefault(); act('settings-close'); } });
    return el;
  });
  const old = scrim.querySelector('.scontent');
  if (old) settingsScroll = old.scrollTop;
  scrim.replaceChildren();
  const card = h('div', 'card glass settings');
  card.addEventListener('mousedown', (e) => { e.preventDefault(); focusInput(); });
  const head = h('div', 'shead-bar', '<span class="gtitle">Settings</span>');
  const search = h('div', 'field sfield' + (s.searching ? ' active' : ''), `${icon('search', 13)}<span class="ftext">` +
    (s.query ? `<span>${escapeHTML(s.query)}</span>` : '') + (s.searching ? '<span class="caret"></span>' : '') +
    (s.query ? '' : '<span class="placeholder">Search all settings</span>') + '</span>');
  if (s.query) search.appendChild(button('tbtn small', icon('close', 11), () => act('settings-search-clear')));
  head.appendChild(search);
  head.appendChild(button('tbtn', icon('close', 13), () => act('settings-close'), 'Close'));
  card.appendChild(head);
  const body = h('div', 'sbody');
  const nav = h('div', 'snav');
  s.pages.forEach((title, k) => {
    nav.appendChild(button('spage' + (k === s.page && !s.query ? ' on' : ''), escapeHTML(title), () => act('settings-page', k)));
  });
  body.appendChild(nav);
  const content = h('div', 'scontent');
  if (s.picker) {
    const p = s.picker;
    const top = h('div', 'pickhead');
    top.appendChild(button('sback', icon('right', 12) + 'Back', () => act('settings-font-back')));
    top.appendChild(h('span', 'stitle strong', escapeHTML(p.title)));
    content.appendChild(top);
    content.appendChild(h('div', 'field sfield active wide', `${icon('search', 13)}<span class="ftext">${p.query ? `<span>${escapeHTML(p.query)}</span>` : ''}<span class="caret"></span>${p.query ? '' : '<span class="placeholder">Search installed fonts</span>'}</span>`));
    const list = h('div', 'flist');
    if (!p.choices.length) list.appendChild(h('div', 'pempty', 'No installed font matches'));
    p.choices.forEach(([label, family, current], k) => {
      const row = h('div', 'frow2' + (k === p.sel ? ' sel' : ''), `<span class="fname">${escapeHTML(label)}</span>` +
        (family ? `<span class="fsample" style="font-family:${fontList(family)}">${escapeHTML(p.sample)}</span>` : '<span></span>') +
        (current ? `<span class="fcheck">${icon('check', 14)}</span>` : '<span class="fcheck"></span>'));
      row.addEventListener('click', () => act('settings-font', -1, family));
      list.appendChild(row);
      if (k === p.sel && p.reveal) requestAnimationFrame(() => row.scrollIntoView({ block: 'nearest' }));
    });
    content.appendChild(list);
  } else {
    if (s.failure) content.appendChild(h('div', 'sfail', `<span style="color:var(--red)">${icon('error', 14)}</span><span>${escapeHTML(s.failure)}</span>`));
    for (const r of s.rows || []) content.appendChild(settingRowEl(r));
  }
  body.appendChild(content);
  card.appendChild(body);
  const foot = h('div', 'pfoot sfoot', hintsHTML(s.hints));
  const links = h('span', 'slinks');
  for (const [label, which] of s.links || []) links.appendChild(button('slink', escapeHTML(label), () => act('settings-link', -1, which)));
  foot.appendChild(links);
  card.appendChild(foot);
  scrim.appendChild(card);
  content.scrollTop = settingsScroll;
  if (s.reveal !== undefined && s.reveal >= 0) {
    const row = content.querySelector(`.srow[data-i="${s.reveal}"]`);
    if (row) row.scrollIntoView({ block: 'nearest' });
  }
  if (s.recording) {
    const r = s.recording;
    const over = h('div', 'srecord-scrim');
    over.addEventListener('mousedown', (e) => { if (e.target === over) { e.preventDefault(); act('settings-record', -1, 'cancel'); } });
    const box = h('div', 'card glass srecord');
    box.innerHTML = `<div class="gtitle">${escapeHTML(r.title)}</div><div class="shelp">Press the new keys, then Enter. ${escapeHTML(r.mode)} mode.</div>` +
      `<div class="rwell">${r.keys ? keysHTML(r.keys) : '<span class="faint">Waiting for keys…</span>'}</div>` +
      `<div class="rnote" style="color:${r.note ? r.note[1] : 'var(--dim)'}">${r.note ? escapeHTML(r.note[0]) : ''}</div>`;
    const buttons = h('div', 'dbuttons');
    buttons.appendChild(button('dbtn left', 'Remove keys', () => act('settings-record', -1, 'clear')));
    buttons.appendChild(button('dbtn', 'Cancel', () => act('settings-record', -1, 'cancel')));
    buttons.appendChild(button('dbtn' + (r.ready ? ' default' : ''), 'Save', () => act('settings-record', -1, 'save')));
    box.appendChild(buttons);
    over.appendChild(box);
    card.appendChild(over);
  }
}

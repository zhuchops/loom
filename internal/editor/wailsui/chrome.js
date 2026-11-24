// The chrome around the text: title bar, sidebar, tabs, path bar, status bar
// and dialogs. Go sends their state as data; each part is redrawn when its
// data changes.
'use strict';

const shown = {}; // each part's last data, as JSON

function changed(part, data) {
  const json = JSON.stringify(data === undefined ? null : data);
  if (shown[part] === json) return false;
  shown[part] = json;
  return true;
}

function act(name, i = -1, s = '') { bridge().Act(name, i, s); }
function run(id) { bridge().Command(id); }

// Clicks on the chrome must not take the keyboard from the editor.
function chromeDown(el) {
  el.addEventListener('mousedown', (e) => {
    if (e.target.closest('input')) return;
    e.preventDefault();
    focusInput();
  });
}

function h(tag, cls, html) {
  const el = document.createElement(tag);
  if (cls) el.className = cls;
  if (html !== undefined) el.innerHTML = html;
  return el;
}

function button(cls, html, onClick, title) {
  const el = h('button', cls, html);
  el.tabIndex = -1;
  if (title) el.title = title;
  el.addEventListener('click', (e) => { e.stopPropagation(); onClick(e); });
  return el;
}

// ---- Title bar ------------------------------------------------------------

function renderTitle(t) {
  const el = $('title');
  if (!changed('title', t)) return;
  el.replaceChildren();
  const left = h('div', 'title-left');
  if (isMac) left.appendChild(h('div', 'lights'));
  if (!t.noProject) left.appendChild(button('tbtn' + (t.sidebar ? ' on' : ''), icon('sidebar'), () => run('sidebar'), 'Toggle Sidebar'));
  const project = button('tproject', `<span class="pname">${escapeHTML(t.project)}</span>` +
    (t.sessions > 1 ? `<span class="pill">${t.sessions}</span>` : '') + icon('down', 11), () => run('sessions'), 'Switch Session');
  left.appendChild(project);
  if (t.branch) left.appendChild(button('tbranch', icon('branch', 13) + `<span>${escapeHTML(t.branch)}</span>`, () => run('branch'), 'Switch Branch'));
  el.appendChild(left);

  const [label, keys, cmd] = t.search;
  const search = button('tsearch', icon('search', 12) + `<span class="label">${escapeHTML(label)}</span><kbd>${escapeHTML(keys)}</kbd>`, () => run(cmd));
  el.appendChild(search);

  const right = h('div', 'title-right');
  if (!t.noProject) {
    const git = button('tbtn', icon('branch'), () => run('git'), 'Source Control');
    if (t.gitDot) git.appendChild(h('span', 'dot'));
    right.appendChild(git);
  }
  right.appendChild(button('tbtn', icon('command'), () => run('palette'), 'Command Palette'));
  right.appendChild(button('tbtn', icon('settings'), () => run('settings'), 'Settings'));
  el.appendChild(right);
}

// ---- Sidebar --------------------------------------------------------------

function renderSidebar(s) {
  const el = $('sidebar');
  if (!s) {
    if (changed('sidebar', null)) el.replaceChildren();
    return;
  }
  const reveal = s.reveal;
  delete s.reveal;
  if (changed('sidebar', s)) {
    el.style.width = `${s.width}px`;
    el.replaceChildren();
    const head = h('div', 'side-head');
    if (s.filter !== undefined) {
      const box = h('div', 'side-filter' + (s.filtering ? ' active' : ''), icon('search', 12) + `<span class="text">${escapeHTML(s.filter)}</span>` + (s.filtering ? '<span class="caret"></span>' : ''));
      box.appendChild(button('tbtn small', icon('close', 12), () => act('tree-clear-filter')));
      head.appendChild(box);
    } else {
      head.appendChild(h('span', 'side-title', 'FILES'));
      if (s.sort) head.appendChild(h('span', 'side-sort', escapeHTML(s.sort)));
      head.appendChild(button('tbtn', icon('newFile'), () => act('tree-key', -1, 'a'), 'New File'));
      head.appendChild(button('tbtn', icon('newFolder'), () => act('tree-key', -1, 'A'), 'New Folder'));
      head.appendChild(button('tbtn', icon('search'), () => act('tree-key', -1, '/'), 'Filter'));
    }
    el.appendChild(head);
    const list = h('div', 'side-list');
    s.rows.forEach((r, i) => {
      const row = h('div', 'trow' + (r.sel ? ' sel' : '') + (r.cur ? ' cur' : '') + (r.cut ? ' cut' : '') + (i === 0 ? ' root' : ''));
      row.style.paddingLeft = `${11 + r.d * 13}px`;
      let guides = '';
      for (let k = 1; k < r.d; k++) guides += `<span class="guide" style="left:${11 + (k - 1) * 13 + 13}px"></span>`;
      let html = guides;
      if (r.dir) {
        html += `<span class="chev">${icon(r.open ? 'down' : 'right', 11)}</span><span class="ficon folder">${icon(r.open ? 'folderOpen' : 'folder', 15)}</span>`;
      } else {
        html += `<span class="chev"></span>${fileIcon(r.icon)}`;
      }
      html += `<span class="tname"${r.git && !r.dir ? ` style="color:${r.git[1]}"` : ''}>${escapeHTML(r.n)}</span>`;
      if (r.git) html += r.dir ? `<span class="gdot" style="background:${r.git[1]}"></span>` : `<span class="gmark" style="color:${r.git[1]}">${escapeHTML(r.git[0])}</span>`;
      row.innerHTML = html;
      row.addEventListener('click', () => act('tree-open', i));
      row.addEventListener('contextmenu', (e) => { e.preventDefault(); act('tree-context', i); });
      list.appendChild(row);
    });
    el.appendChild(list);
    const edge = h('div', 'side-edge');
    edge.addEventListener('mousedown', (e) => {
      e.preventDefault();
      const move = (ev) => { el.style.width = `${Math.max(162, Math.min(576, ev.clientX))}px`; };
      const up = (ev) => {
        window.removeEventListener('mousemove', move);
        window.removeEventListener('mouseup', up);
        act('tree-width', Math.round(Math.max(162, Math.min(576, ev.clientX))));
      };
      window.addEventListener('mousemove', move);
      window.addEventListener('mouseup', up);
    });
    el.appendChild(edge);
  }
  if (reveal !== undefined) {
    const row = el.querySelectorAll('.trow')[reveal];
    if (row) row.scrollIntoView({ block: 'nearest' });
  }
}

// ---- Tabs -----------------------------------------------------------------

let tabDrag = null;
function renderTabs(tabs) {
  const el = $('tabs');
  if (!changed('tabs', tabs)) return;
  el.replaceChildren();
  if (!tabs) return;
  tabs.forEach((t, i) => {
    const tab = h('div', 'tab' + (t.active ? ' active' : ''));
    const mark = t.welcome ? `<img class="logo" src="${state.logo || ''}" alt="">` : fileIcon(t.icon);
    tab.innerHTML = `${mark}<span class="tlabel">${escapeHTML(t.name)}</span>` + (t.folder ? `<span class="tfolder">${escapeHTML(t.folder)}</span>` : '');
    const close = button('tclose' + (t.dirty ? ' dirty' : ''), icon('close', 12), () => act('tab-close', i), 'Close');
    tab.appendChild(close);
    tab.addEventListener('mousedown', (e) => {
      if (e.button === 1) { e.preventDefault(); act('tab-close', i); return; }
      if (e.button !== 0 || e.target.closest('.tclose')) return;
      act('tab', i);
      tabDrag = { from: i, at: i };
    });
    tab.addEventListener('mousemove', () => {
      if (!tabDrag || tabDrag.at === i) return;
      tabDrag.at = i;
      act('tab-move', i);
    });
    el.appendChild(tab);
    if (t.active) requestAnimationFrame(() => tab.scrollIntoView({ inline: 'nearest', block: 'nearest' }));
  });
}
// Dragged over the text, a tab offers to split the pane it is over.
window.addEventListener('mousemove', (e) => {
  if (!tabDrag || !(e.buttons & 1)) return;
  const over = e.target.closest && e.target.closest('#editor');
  if (!over) {
    if (tabDrag.over) { tabDrag.over = false; act('tab-over', -1, '-1'); }
    return;
  }
  const [x, y] = cellAt(e);
  const at = `${x},${y}`;
  if (at === tabDrag.cell) return;
  tabDrag.cell = at;
  tabDrag.over = true;
  act('tab-over', x, String(y));
});
window.addEventListener('mouseup', () => {
  if (tabDrag && tabDrag.over) act('tab-drop');
  tabDrag = null;
});

// ---- Path bar -------------------------------------------------------------

function renderCrumbs(c) {
  const el = $('crumbs');
  if (!changed('crumbs', c)) return;
  el.replaceChildren();
  if (!c) return;
  const sep = `<span class="sep">${icon('right', 11)}</span>`;
  let html = '';
  for (const d of c.dirs) html += `<span class="dir">${escapeHTML(d)}</span>${sep}`;
  html += `${fileIcon(c.icon)}<span class="file">${escapeHTML(c.name)}</span>`;
  for (const s of c.scopes || []) html += `${sep}<span class="scope">${escapeHTML(s)}</span>`;
  el.innerHTML = html;
}

// ---- Status bar -----------------------------------------------------------

function renderStatus(s) {
  const el = $('status');
  if (!changed('status', s)) return;
  el.replaceChildren();
  if (!s) return;
  const left = h('div', 'status-left');
  if (s.mode) {
    const [mode, fill, ink] = s.mode;
    const badge = h('span', 'mode', escapeHTML(mode));
    badge.style.background = fill;
    badge.style.color = ink;
    left.appendChild(badge);
  }
  if (s.keys) left.appendChild(h('span', 'keys', escapeHTML(s.keys)));
  if (s.problems) {
    const p = s.problems;
    const b = button('sbtn', `<span style="color:${p.errorInk || 'var(--dim)'}">${icon('error', 13)}${p.errors}</span>` +
      `<span style="color:${p.warningInk || 'var(--dim)'}">${icon('warning', 13)}${p.warnings}</span>`, () => run('problems'), 'Problems');
    left.appendChild(b);
  }
  el.appendChild(left);
  const right = h('div', 'status-right');
  for (const [text, cmd] of [...(s.segments || [])].reverse()) {
    if (cmd) right.appendChild(button('sbtn', escapeHTML(text), () => run(cmd)));
    else right.appendChild(h('span', 'sseg', escapeHTML(text)));
  }
  el.appendChild(right);
}

// ---- Dialog ---------------------------------------------------------------

function renderDialog(q) {
  if (!changed('dialog', q)) return;
  const old = $('dialog');
  if (old) old.remove();
  if (!q) return;
  const scrim = h('div', 'scrim');
  scrim.id = 'dialog';
  const card = h('div', 'card glass dialog');
  card.appendChild(h('div', 'dtitle', escapeHTML(q.title)));
  card.appendChild(h('div', 'dbody', escapeHTML(q.body)));
  const row = h('div', 'dbuttons');
  // Buttons run right to left, the default on the right.
  q.choices.forEach((label, i) => {
    row.prepend(button('dbtn' + (i === q.selected ? ' default' : ''), escapeHTML(label), () => bridge().Answer(i)));
  });
  card.appendChild(row);
  scrim.appendChild(card);
  chromeDown(scrim);
  $('layers').appendChild(scrim);
}

function applyChrome(ui) {
  renderTitle(ui.title);
  renderSidebar(ui.sidebar);
  renderTabs(ui.tabs);
  renderCrumbs(ui.crumbs);
  renderStatus(ui.status);
  renderDialog(ui.dialog);
  applyPopups(ui);
  applyPanels(ui);
  renderSettings(ui.settings);
}

for (const id of ['title', 'sidebar', 'tabs', 'crumbs', 'status']) chromeDown($(id));

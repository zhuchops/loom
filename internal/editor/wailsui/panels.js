// Panels: Source Control, diffs, the directory editor, and what is drawn over
// the panes (the Welcome page, scrollbars and dividers).
'use strict';

const GIT_ROW = 30;
const DIFF_ROW = 21;

function modal(id, cls, close) {
  const scrim = layer(id, () => {
    const s = h('div', 'scrim');
    s.addEventListener('mousedown', (e) => { if (e.target === s) { e.preventDefault(); close(); } });
    return s;
  });
  scrim.replaceChildren();
  const card = h('div', `card glass ${cls}`);
  card.addEventListener('mousedown', (e) => { e.preventDefault(); focusInput(); });
  scrim.appendChild(card);
  return card;
}

function wheelSteps(el, step, send) {
  let acc = 0;
  el.addEventListener('wheel', (e) => {
    e.preventDefault();
    acc += e.deltaY;
    const n = Math.trunc(acc / step);
    if (n) { acc -= n * step; send(n); }
  }, { passive: false });
}

// ---- Source Control -------------------------------------------------------

let gitRowsSent = 0;
function renderGit(g) {
  if (!changed('git', g)) return;
  if (!g) { drop('git'); return; }
  const rows = Math.max(4, Math.min(14, Math.floor((Math.min(440, window.innerHeight - 200)) / GIT_ROW)));
  if (rows !== gitRowsSent) { gitRowsSent = rows; act('git-rows', rows); }
  const card = modal('git', 'gitpanel', () => act('panel-close'));
  const head = h('div', 'ghead', '<span class="gtitle">Source Control</span>');
  if (g.branch) head.appendChild(button('tbranch', icon('branch', 13) + `<span>${escapeHTML(g.branch)}</span>`, () => act('git-button', -1, 'branch')));
  const buttons = h('span', 'gbuttons');
  if (g.ready) {
    buttons.appendChild(button('tbutton', 'Pull', () => act('git-button', -1, 'pull')));
    buttons.appendChild(button('tbutton', 'Push', () => act('git-button', -1, 'push')));
    buttons.appendChild(button('tbutton' + (g.staged ? ' primary' : ''), 'Commit', () => act('git-button', -1, 'commit')));
  }
  head.appendChild(buttons);
  card.appendChild(head);
  const list = h('div', 'glist');
  list.style.height = `${rows * GIT_ROW + 16}px`;
  if (!g.ready) list.appendChild(h('div', 'pempty', 'Reading repository…'));
  else if (!g.total) list.appendChild(h('div', 'pempty', `<span style="color:var(--green)">${icon('check', 24)}</span><span>No changes. The working tree is clean.</span>`));
  (g.rows || []).forEach((r, k) => {
    const i = g.scroll + k;
    if (r.header) { list.appendChild(h('div', 'gheader', escapeHTML(r.header))); return; }
    const row = h('div', 'grow' + (i === g.selected ? ' sel' : ''));
    row.innerHTML = fileIcon(r.icon) + `<span class="gname">${escapeHTML(r.name)}</span>` + (r.dir ? `<span class="gdir">${ltr(r.dir)}</span>` : '');
    row.appendChild(button('tbtn small gstage', icon(r.staged ? 'minus' : 'plus', 13), () => act('git-stage', i), r.staged ? 'Unstage' : 'Stage'));
    row.appendChild(h('span', 'gmark', escapeHTML(r.mark[0]))).style.color = r.mark[1];
    row.addEventListener('click', () => act('git-open', i));
    list.appendChild(row);
  });
  wheelSteps(list, GIT_ROW, (n) => act('git-scroll', n));
  card.appendChild(list);
  const narrow = card.offsetWidth < 600;
  card.appendChild(h('div', 'pfoot', hintsHTML(narrow ? ['↵', 'diff', 's', 'stage', 'esc', 'close']
    : ['↵', 'diff', 's', 'stage', 'a', 'stage all', 'c', 'commit', 'd', 'discard', 'esc', 'close'])));
}

// ---- Diff -----------------------------------------------------------------

let diffRowsSent = 0;
function renderDiff(d) {
  if (!changed('diff', d)) return;
  if (!d) { drop('diff'); return; }
  const height = Math.min(620, window.innerHeight - 120);
  const rows = Math.max(4, Math.floor((height - 46 - 38 - 6) / DIFF_ROW));
  if (rows !== diffRowsSent) { diffRowsSent = rows; act('diff-rows', rows); }
  const card = modal('diff', 'diffview', () => act('panel-close'));
  card.style.height = `${height}px`;
  const head = h('div', 'ghead', `${fileIcon(d.icon)}<span class="gtitle path">${ltr(d.title)}</span>` +
    `<span class="stat add">${escapeHTML(d.added)}</span><span class="stat del">${escapeHTML(d.removed)}</span>`);
  head.appendChild(button('tbtn', icon('close', 13), () => act('panel-close'), 'Close'));
  card.appendChild(head);
  const body = h('div', 'dbodyc code');
  body.innerHTML = d.lines.map(([old, cur, sign, text, kind, at], k) =>
    `<div class="dline ${kind}${at ? ' at' : ''}" data-i="${d.scroll + k}"><span class="n">${old}</span><span class="n">${cur}</span><span class="sign">${sign}</span><span class="t">${escapeHTML(text)}</span></div>`).join('');
  body.querySelectorAll('.dline').forEach((el) => el.addEventListener('click', () => act('diff-cursor', Number(el.dataset.i))));
  wheelSteps(body, DIFF_ROW, (n) => act('diff-scroll', n));
  card.appendChild(body);
  card.appendChild(h('div', 'pfoot', hintsHTML([']c', 'next change', '[c', 'previous', '↵', 'open here', 'esc', 'close'])));
}

// ---- Directory editor -------------------------------------------------------

function renderDirectory(dir) {
  if (!changed('directory', dir)) return;
  const ed = $('editor');
  ed.querySelectorAll('.dirhead, .dirfoot').forEach((el) => el.remove());
  drop('dirscrim');
  ed.classList.toggle('directory', !!dir);
  if (!dir) return;
  const scrim = h('div', 'scrim dirscrim');
  scrim.id = 'dirscrim';
  scrim.addEventListener('mousedown', (e) => { e.preventDefault(); act('directory-close'); });
  $('app').appendChild(scrim);
  ed.appendChild(h('div', 'dirhead', `<span class="ficon folder">${icon('folderOpen', 15)}</span><span class="gtitle path">${ltr(dir.path)}</span>` +
    (dir.dirty ? '<span class="unsaved">Unsaved changes</span>' : '')));
  ed.appendChild(h('div', 'dirfoot pfoot', hintsHTML(['↵', 'open', ':w', 'apply edits', 'esc', 'close'])));
}

// ---- Over the panes ---------------------------------------------------------

function renderPanes(panes) {
  if (!changed('panes', [panes, state.cellW, state.cellH])) return;
  const el = $('panes');
  el.replaceChildren();
  const ox = grid.offsetLeft;
  const oy = grid.offsetTop;
  const W = state.cellW;
  const H = state.cellH;
  const box = (b, extend) => {
    const r = { left: ox + b[0] * W, top: oy + b[1] * H, width: b[2] * W, height: b[3] * H };
    if (extend) {
      // Panes at the edges reach the edge of the editor.
      if (b[0] === 0) { r.width += r.left; r.left = 0; }
      if (b[1] === 0) { r.height += r.top; r.top = 0; }
      if (b[0] + b[2] >= state.cols) r.width = el.clientWidth - r.left;
      if (b[1] + b[3] >= state.rows) r.height = el.clientHeight - r.top;
    }
    return r;
  };
  const place = (node, r) => Object.assign(node.style, { left: `${r.left}px`, top: `${r.top}px`, width: `${r.width}px`, height: `${r.height}px` });
  for (const p of panes || []) {
    if (p.divider) {
      const d = h('div', 'divider' + (p.vertical ? ' v' : ' h'));
      place(d, box(p.divider, false));
      el.appendChild(d);
      continue;
    }
    if (p.welcome) {
      const w = welcomeEl(p.welcome, p.i);
      place(w, box(p.box, true));
      el.appendChild(w);
      continue;
    }
    if (p.scroll) {
      const [top, shown, total] = p.scroll;
      const r = box(p.box, true);
      const track = h('div', 'pscrollbar');
      place(track, { left: r.left + r.width - 10, top: r.top, width: 10, height: r.height });
      const thumb = h('div', 'thumb');
      const th = Math.max(24, r.height * shown / total);
      thumb.style.height = `${th}px`;
      thumb.style.top = `${(r.height - th) * Math.min(top, total - shown) / Math.max(1, total - shown)}px`;
      track.appendChild(thumb);
      const move = (e) => {
        const t = track.getBoundingClientRect();
        const want = Math.round((e.clientY - t.top) * total / Math.max(1, t.height) - shown / 2);
        act('pane-scroll', want, String(p.i));
      };
      track.addEventListener('mousedown', (e) => {
        e.preventDefault();
        e.stopPropagation();
        move(e);
        const up = () => { window.removeEventListener('mousemove', move); window.removeEventListener('mouseup', up); };
        window.addEventListener('mousemove', move);
        window.addEventListener('mouseup', up);
      });
      el.appendChild(track);
    }
  }
}

function welcomeEl(w, pane) {
  const el = h('div', 'welcome');
  el.addEventListener('mousedown', (e) => { e.preventDefault(); e.stopPropagation(); focusInput(); });
  const col = h('div', 'wcol');
  col.appendChild(h('div', 'whero', `<img src="${state.logo || ''}" alt=""><div class="wname">Loom</div><div class="wsub">${ltr(w.sub)}</div>`));
  col.appendChild(h('div', 'wcaption', 'START'));
  let i = 0;
  for (const [ic, label, hint] of w.start || []) {
    const k = i++;
    const row = h('div', 'witem' + (w.focused && k === w.selected ? ' sel' : ''), `<span class="wicon">${icon(ic, 16)}</span><span class="wlabel">${escapeHTML(label)}</span><span class="mhint keys">${keysHTML(hint)}</span>`);
    row.addEventListener('click', () => act('welcome', k, String(pane)));
    col.appendChild(row);
  }
  if ((w.recent || []).length) {
    col.appendChild(h('div', 'wcaption gapped', escapeHTML(w.recentTitle)));
    const list = h('div', 'wrecent');
    for (const [mark, name, detail] of w.recent) {
      const k = i++;
      const ic = mark === 'dir' ? `<span class="ficon folder">${icon('folder', 16)}</span>` : fileIcon(mark);
      const row = h('div', 'witem recent' + (w.focused && k === w.selected ? ' sel' : ''), `<span class="wicon">${ic}</span><span class="wtext"><span class="wlabel">${escapeHTML(name)}</span><span class="wdetail">${escapeHTML(detail)}</span></span>`);
      row.addEventListener('click', () => act('welcome', k, String(pane)));
      list.appendChild(row);
      if (w.focused && k === w.selected) requestAnimationFrame(() => row.scrollIntoView({ block: 'nearest' }));
    }
    col.appendChild(list);
  }
  col.appendChild(h('div', 'wfoot', hintsHTML(['Space', 'all shortcuts', ':', 'commands', '↑↓', 'choose'])));
  el.appendChild(col);
  return el;
}

function applyPanels(ui) {
  renderGit(ui.git);
  renderDiff(ui.diff);
  renderDirectory(ui.directory);
  renderPanes(ui.panes);
}

import * as api from './api.js';
import { $, arabic, el, plural } from './dom.js';
import { banner, cardView, flagView, notesWord, previewCard, proposalView, tally } from './cards.js';
import { editBody, editorView } from './editor.js';
import { composeExample, splitExample } from './example.js';
import { FIX_LABEL, KEY_LABEL, PLAY_KEY, messageTone } from './labels.js';

/**
 * @typedef {{
 *   view: import('./types.js').View | null,
 *   current: string,
 *   shown: string,
 *   mode: 'review' | 'edit',
 *   draft: import('./types.js').Note | null,
 *   parts: import('./types.js').Parts | null,
 *   original: string,
 *   message: { text: string, tone: string },
 *   playing: string,
 *   voices: import('./types.js').Voice[] | null,
 *   voicesNote: string,
 *   poll: number,
 *   seq: number,
 *   pending: number,
 *   finished: boolean,
 *   ended: boolean,
 *   summary: import('./types.js').Summary | null,
 * }} State
 */

/** @type {State} */
const state = {
  view: null,
  current: '',
  shown: '',
  mode: 'review',
  draft: null,
  parts: null,
  original: '',
  message: { text: '', tone: 'info' },
  playing: '',
  voices: null,
  voicesNote: '',
  poll: 0,
  seq: 0,
  pending: 0,
  finished: false,
  ended: false,
  summary: null,
};

const player = new Audio();

/**
 * @returns {import('./types.js').Entry | null}
 */
function entryOf() {
  return state.view && state.view.entry ? state.view.entry : null;
}

/**
 * @returns {number}
 */
function position() {
  return state.view ? state.view.list.findIndex((x) => x.id === state.current) : -1;
}

/**
 * @param {string} action
 * @param {Record<string, unknown>} [body]
 * @returns {Promise<import('./types.js').Result>}
 */
function post(action, body) {
  const e = entryOf();
  return api.post(action, e ? e.id : '', { if_rev: e ? e.rev : undefined, ...body });
}

/**
 * @param {unknown} err
 */
function failed(err) {
  if (err instanceof api.ApiError) {
    say(err.message, err.status === 409 ? 'info' : 'bad');
    if (err.status === 409) load(state.current, true);
    return;
  }
  state.finished = true;
  state.ended = true;
  clearTimeout(state.poll);
  stopAudio();
  render();
}

/**
 * @param {string} id
 * @param {boolean} [quiet]
 * @returns {Promise<void>}
 */
async function load(id, quiet) {
  state.seq += 1;
  state.pending += 1;
  const seq = state.seq;
  let view;
  try {
    view = await api.loadState(id);
  } catch (err) {
    failed(err);
    return;
  } finally {
    state.pending -= 1;
  }
  if (seq !== state.seq) return;
  announce(view);
  state.view = view;
  state.current = view.entry ? view.entry.id : '';
  if (quiet && state.mode === 'edit') {
    renderChrome();
  } else {
    render();
  }
  schedule();
}

/**
 * @param {string} action
 * @param {Record<string, unknown>} [body]
 * @returns {Promise<import('./types.js').Result | undefined>}
 */
async function act(action, body) {
  let res;
  try {
    res = await post(action, body);
  } catch (err) {
    failed(err);
    return undefined;
  }
  if (res.message) say(res.message, messageTone(res.tone));
  const show = res.show || '';
  if (show !== state.current) stopAudio();
  state.current = show;
  await load(show);
  return res;
}

/**
 * @param {string} kind
 */
async function remake(kind) {
  say('♪ Making the ' + kind + ' audio with Google Text-to-Speech…', 'audio');
  if (!(await act('remake', { clip: kind }))) return;
  const e = entryOf();
  if (e && ready(e, kind)) play(kind);
}

/**
 * @param {string} kind
 */
function removeClip(kind) {
  if (!window.confirm('Delete the ' + kind + " MP3? Remake or 'mufradat audio' makes it again.")) return;
  act('remove', { clip: kind });
}

/**
 * @param {import('./types.js').View} view
 */
function announce(view) {
  const before = new Map((state.view ? state.view.list : []).map((item) => [item.id, item]));
  const here = view.entry ? view.entry.id : '';
  for (const item of view.list) {
    const old = before.get(item.id);
    if (!old) continue;
    const asked = old.asking && !item.asking;
    const checked = old.checking && !item.checking;
    if (!asked && !checked) continue;
    const where = item.id === here ? '' : ' (' + item.lemma + ')';
    if (asked && item.ready) {
      say("✦ Mistral's version of " + item.lemma + ' is ready' + where, 'mistral');
    } else if (item.notice) {
      say(item.notice + where, messageTone(item.notice_tone));
    }
  }
}

function schedule() {
  clearTimeout(state.poll);
  const v = state.view;
  if (!v || state.finished || !v.list.some((x) => x.asking || x.checking)) return;
  state.poll = window.setTimeout(() => load(state.current, true), 1000);
}

/**
 * @param {string} text
 * @param {string} [tone]
 */
function say(text, tone) {
  state.message = { text, tone: tone || 'info' };
  renderStatus();
}

function snapshot() {
  return JSON.stringify([state.draft, state.parts]);
}

function dirty() {
  return state.mode === 'edit' && snapshot() !== state.original;
}

function leaveEdit() {
  return !dirty() || window.confirm('Discard your changes to this note?');
}

window.addEventListener('beforeunload', (ev) => {
  if (dirty()) ev.preventDefault();
});

/**
 * @param {string} id
 */
function go(id) {
  if (!leaveEdit()) return;
  if (state.mode === 'edit') {
    state.mode = 'review';
    state.draft = null;
    state.parts = null;
  }
  if (id !== state.current) stopAudio();
  state.current = id;
  load(id);
}

/**
 * @param {number} d
 */
function move(d) {
  const list = state.view ? state.view.list : [];
  const at = position();
  if (at < 0) {
    if (d < 0 && list.length) go(list[list.length - 1].id);
    return;
  }
  const next = at + d;
  if (next < 0) return;
  go(next >= list.length ? '' : list[next].id);
}

/**
 * @param {import('./types.js').Entry} e
 * @param {string} kind
 * @returns {boolean}
 */
function ready(e, kind) {
  const c = (e.clips || []).find((x) => x.kind === kind);
  return !!(c && c.ready);
}

/**
 * @param {string} kind
 */
function play(kind) {
  const e = entryOf();
  if (!e) return;
  if (!ready(e, kind)) {
    say('There is no audio for this ' + kind + " yet; 'Remake' or 'mufradat audio' makes it", 'info');
    return;
  }
  player.pause();
  player.src = api.audioURL(e.id, kind);
  state.playing = kind;
  renderStatus();
  player.play().catch((err) => {
    state.playing = '';
    say('Could not play the ' + kind + ': ' + err.message, 'bad');
  });
}

function stopAudio() {
  const was = state.playing;
  state.playing = '';
  player.pause();
  if (was) renderStatus();
}

player.addEventListener('ended', () => {
  state.playing = '';
  renderStatus();
});

player.addEventListener('error', () => {
  if (!state.playing) return;
  state.playing = '';
  say('Could not play the clip', 'bad');
});

async function finish() {
  if (!leaveEdit()) return;
  let sum;
  try {
    sum = await api.finish();
  } catch (err) {
    failed(err);
    return;
  }
  state.finished = true;
  state.summary = sum;
  clearTimeout(state.poll);
  stopAudio();
  render();
}

/**
 * @param {boolean} fromProposal
 */
function startEdit(fromProposal) {
  const e = entryOf();
  if (!e) return;
  const source = fromProposal && e.proposal ? e.proposal : e.note;
  const draft = structuredClone(source);
  draft.forms = draft.forms || [];
  state.draft = draft;
  state.parts = splitExample(draft.example);
  state.original = snapshot();
  state.mode = 'edit';
  stopAudio();
  render();
  const first = $('stage').querySelector('.editor input, .editor textarea');
  if (first instanceof HTMLElement) first.focus();
}

function cancelEdit() {
  state.mode = 'review';
  state.draft = null;
  state.parts = null;
  say('Left the note as it was', 'info');
  render();
}

async function saveEdit() {
  const draft = state.draft;
  if (!draft) return;
  const example = state.parts ? composeExample(state.parts) : draft.example || '';
  if (example === null) {
    say('The highlighted word has to appear in the sentence', 'bad');
    return;
  }
  let res;
  try {
    res = await post('save', { note: editBody(draft, example) });
  } catch (err) {
    failed(err);
    return;
  }
  state.mode = 'review';
  state.draft = null;
  state.parts = null;
  if (res.message) say(res.message, messageTone(res.tone));
  await load(res.show || '');
}

/**
 * @returns {import('./types.js').Action[]}
 */
function actions() {
  const v = state.view;
  if (!v || state.finished) return [];
  if (state.mode === 'edit') {
    return [
      { key: 'ctrl+enter', label: 'Save changes', run: saveEdit, primary: true },
      { key: 'esc', label: 'Cancel', run: cancelEdit },
    ];
  }
  /** @type {import('./types.js').Action[]} */
  const list = [];
  const e = v.entry;
  if (!e) {
    const first = v.list.find((x) => x.state === 'open');
    if (first) list.push({ key: 'enter', label: 'First open note', run: () => go(first.id), primary: true, nav: true });
    if (v.list.length) list.push({ key: 'left', label: 'Back to the notes', run: () => move(-1), nav: true });
    if (v.history) list.push({ key: 'u', label: 'Undo', run: () => act('undo') });
    return list;
  }
  if (e.proposal) {
    list.push(
      { key: 'y', label: 'Use this version', run: () => act('use'), primary: true, tone: 'mistral' },
      { key: 'n', label: 'Keep yours', run: () => act('drop') },
      { key: 'e', label: 'Edit it first', run: () => startEdit(true) },
    );
  } else {
    list.push(
      { key: 'enter', label: e.state === 'open' ? 'Card is right' : 'Next open note', run: () => act('keep'), primary: true },
      { key: 'e', label: 'Edit', run: () => startEdit(false) },
    );
    (e.fixes || []).forEach((f, k) => {
      list.push({
        key: String(k + 1),
        label: FIX_LABEL[f.source] + (f.words.length > 1 ? 's' : ''),
        arabic: f.words.join(' '),
        run: () => act('ending', { source: f.source }),
        tone: 'good',
      });
    });
    if (e.state === 'open' || e.state === 'kept') {
      if (e.asking) {
        list.push({ key: 'esc', label: 'Stop Mistral', run: () => act('stop'), tone: 'mistral' });
      } else if (v.mistral) {
        list.push({ key: 'c', label: 'Ask Mistral', run: () => act('ask'), tone: 'mistral' });
      }
    }
  }
  for (const kind of Object.keys(PLAY_KEY)) {
    if (ready(e, kind)) list.push({ key: PLAY_KEY[kind], label: 'Play ' + kind, run: () => play(kind), tone: 'audio' });
  }
  list.push(
    { key: 'left', label: 'Previous', run: () => move(-1), push: true, nav: true },
    { key: 'right', label: 'Next', run: () => move(1), nav: true },
  );
  if (v.history) list.push({ key: 'u', label: 'Undo', run: () => act('undo') });
  return list;
}

/**
 * @param {KeyboardEvent} ev
 * @returns {string}
 */
function keyOf(ev) {
  if (ev.altKey || ev.isComposing) return '';
  if (ev.code === 'Enter' || ev.code === 'NumpadEnter' || ev.key === 'Enter') {
    return ev.ctrlKey || ev.metaKey ? 'ctrl+enter' : 'enter';
  }
  if (ev.ctrlKey || ev.metaKey) return '';
  if (ev.key === 'Escape') return 'esc';
  if (ev.key === 'ArrowLeft') return 'left';
  if (ev.key === 'ArrowRight') return 'right';
  if (ev.code && ev.code.startsWith('Key')) return ev.code.slice(3).toLowerCase();
  if (/^(Digit|Numpad)\d$/.test(ev.code || '')) return ev.code.slice(-1);
  return '';
}

document.addEventListener('keydown', (ev) => {
  const key = keyOf(ev);
  if (!key) return;
  const typing = ev.target instanceof Element && ev.target.closest('input, textarea, select');
  if (typing && key !== 'esc' && key !== 'ctrl+enter') return;
  const action = actions().find((a) => a.key === key);
  if (!action) return;
  ev.preventDefault();
  run(action);
});

/**
 * @param {import('./types.js').Action} action
 */
function run(action) {
  if (state.pending && !action.nav) return;
  action.run();
}

function render() {
  renderChrome();
  renderStage();
}

function renderChrome() {
  renderTop();
  renderList();
  renderFooter();
}

function renderTop() {
  const v = state.view;
  if (!v) return;
  const s = state.summary || v.summary;
  const at = position();
  $('where').textContent = at >= 0 && !state.finished ? 'note ' + (at + 1) + ' of ' + s.total : 'summary';
  const counts = [el('span', null, s.open + ' open')];
  if (s.kept) counts.push(el('span', 'good', '✓ ' + s.kept + ' right'));
  if (s.edited) counts.push(el('span', 'good', '✎ ' + s.edited + ' edited'));
  if (s.rewritten) counts.push(el('span', 'mistral', '✦ ' + s.rewritten + ' rewritten'));
  $('counts').replaceChildren(...counts);
  $('fill').style.width = (s.total ? (100 * (s.total - s.open)) / s.total : 100) + '%';
  $('finish').hidden = state.finished;
  const e = v.entry;
  document.title = (e && !state.finished ? e.note.lemma + ' · ' : '') + 'mufradat review';
}

/**
 * @param {import('./types.js').ListItem} item
 * @returns {HTMLElement}
 */
function icon(item) {
  if (item.asking || item.checking) return el('span', 'icon spinner');
  if (item.ready) return el('span', 'icon mistral', '✦');
  if (item.state === 'kept') return el('span', 'icon good', '✓');
  if (item.state === 'edited') return el('span', 'icon good', '✎');
  if (item.state === 'rewritten') return el('span', 'icon mistral', '✦');
  return el('span', 'icon dot tone-' + item.tone);
}

function renderList() {
  const v = state.view;
  if (!v) return;
  const ol = el('ol');
  for (const item of v.list) {
    const button = el('button', 'item ' + item.state + (item.id === state.current ? ' current' : ''));
    button.type = 'button';
    button.title = item.english + ' · ' + plural(item.flags, 'flag', 'flags');
    button.append(icon(item), arabic('word', [{ t: item.lemma }]), el('span', 'num', item.rank ? '#' + item.rank : ''));
    button.addEventListener('click', () => go(item.id));
    ol.append(el('li', null, button));
  }
  const open = v.list.filter((x) => x.state === 'open').length;
  const heading = v.all ? 'Every note' : 'Flagged notes';
  $('list').replaceChildren(el('h2', null, el('span', null, heading), el('span', null, open + ' / ' + v.list.length)), ol);
  const current = $('list').querySelector('.current');
  if (current) current.scrollIntoView({ block: 'nearest' });
}

function renderFooter() {
  $('actions').replaceChildren(...actions().map(actionButton));
  renderStatus();
}

/**
 * @param {import('./types.js').Action} a
 * @returns {HTMLButtonElement}
 */
function actionButton(a) {
  let cls = 'action';
  if (a.primary) cls += ' primary';
  if (a.tone) cls += ' tone-' + a.tone;
  if (a.push) cls += ' push';
  const button = el('button', cls, el('kbd', null, KEY_LABEL[a.key] || a.key), el('span', null, a.label), a.arabic ? arabic('', [{ t: a.arabic }]) : null);
  button.type = 'button';
  button.addEventListener('click', () => run(a));
  return button;
}

function renderStatus() {
  const message = $('message');
  message.className = 'message ' + state.message.tone;
  message.textContent = state.message.text;
  const parts = [];
  if (state.playing) parts.push(el('span', 'playing', '▶ playing the ' + state.playing));
  const v = state.view;
  if (v && !state.finished) {
    const here = v.list.find((x) => x.id === state.current && x.asking);
    const others = v.list.filter((x) => x.asking && x.id !== state.current).length;
    const checking = v.list.filter((x) => x.checking).length;
    if (here) parts.push(el('span', 'asking', el('span', 'spinner'), 'Mistral is writing a new version'));
    if (others) parts.push(el('span', 'asking', el('span', 'spinner'), 'Mistral is working on ' + plural(others, 'other note', 'other notes')));
    if (checking) parts.push(el('span', 'asking', el('span', 'spinner'), 'Checking ' + plural(checking, 'change', 'changes') + ' again'));
  }
  $('activity').replaceChildren(...parts);
}

function renderStage() {
  const stage = $('stage');
  const v = state.view;
  if (!v) return;
  if (state.finished) {
    stage.replaceChildren(finishedView());
    return;
  }
  const e = v.entry;
  if (!e) {
    stage.replaceChildren(doneView());
    stage.focus({ preventScroll: true });
    return;
  }
  let side;
  if (state.mode === 'edit' && state.draft) {
    side = editorView(state.draft, state.parts, refreshPreview, saveEdit);
  } else if (e.proposal) {
    side = proposalView(e);
  } else {
    side = flagsView(e);
  }
  const card = cardView(state.mode === 'edit' && state.draft ? previewCard(state.draft, state.parts, e.note) : e.card);
  stage.replaceChildren(card, side);
  if (state.shown !== e.id) {
    stage.scrollTop = 0;
    state.shown = e.id;
  }
  if (state.mode !== 'edit') stage.focus({ preventScroll: true });
}

function refreshPreview() {
  const e = entryOf();
  const card = $('stage').querySelector('.card');
  if (card && e && state.draft) card.replaceWith(cardView(previewCard(state.draft, state.parts, e.note)));
}

/**
 * @param {import('./types.js').Entry} e
 * @returns {HTMLElement}
 */
function flagsView(e) {
  const v = state.view;
  const side = el('section', 'side');
  /** @type {Record<string, string>} */
  const titles = {
    open: e.flags.length ? plural(e.flags.length, 'flag', 'flags') + ' to look at' : 'Nothing is flagged on this note',
    kept: '✓ You marked this card as right',
    edited: '✎ You edited this note',
    rewritten: "✦ You kept Mistral's version",
  };
  side.append(el('h2', 'side-title ' + e.state, titles[e.state]));
  if (e.state === 'edited' || e.state === 'rewritten') {
    side.append(el('p', 'side-note', v && v.verify
      ? 'The change is checked again automatically; the flags below are from before it.'
      : "'mufradat check' will check it again; the flags below are from before the change."));
  } else if (!e.verified && e.flags.length === 0) {
    side.append(el('p', 'side-note', v && v.verify
      ? "'mufradat check' has not cross-checked this note yet, so there are no flags to show."
      : 'Vowel checking is not available, so there are no flags to show.'));
  }
  if (e.asking) {
    side.append(banner('mistral', el('span', 'spinner'),
      el('span', 'grow', 'Mistral is writing a new version · ' + e.seconds + 's'),
      actionButton({ key: 'esc', label: 'Stop', run: () => act('stop'), tone: 'mistral' })));
  }
  if (e.checking) side.append(banner('info', el('span', 'spinner'), el('span', 'grow', 'Checking the change again…')));
  if (e.notice) side.append(banner(messageTone(e.notice_tone), el('span', 'grow', e.notice)));
  for (const f of e.flags) side.append(flagView(f));
  if (v && v.audio) side.append(clipsView(e));
  return side;
}

/**
 * @param {string} label
 * @param {string} title
 * @param {boolean} enabled
 * @param {() => void} onClick
 * @returns {HTMLButtonElement}
 */
function clipButton(label, title, enabled, onClick) {
  const button = el('button', 'small', label);
  button.type = 'button';
  button.title = title;
  button.disabled = !enabled;
  button.addEventListener('click', onClick);
  return button;
}

/**
 * @param {import('./types.js').Entry} e
 * @returns {HTMLElement}
 */
function clipsView(e) {
  const v = state.view;
  const box = el('section', 'clips', el('h3', null, 'Audio'));
  for (const c of e.clips) {
    const row = el('div', 'clip', el('span', 'kind', c.kind));
    row.append(
      clipButton(c.ready ? '↻ Remake' : '↻ Make', 'Synthesize this clip again with Google Text-to-Speech', true, () => remake(c.kind)),
      clipButton('✖ Remove', 'Delete this MP3', c.ready, () => removeClip(c.kind)));
    if (!c.ready) row.append(el('span', 'note', 'not made yet'));
    box.append(row);
  }
  if (v && v.voice) box.append(voiceRow(v.voice));
  if (!state.voices) loadVoices();
  return box;
}

/**
 * @param {import('./types.js').Voice} v
 * @returns {string}
 */
function voiceLabel(v) {
  const name = v.name.replace(/^ar-XA-/, '');
  return v.gender ? name + ' · ' + v.gender.toLowerCase() : name;
}

/**
 * @param {string} current
 * @returns {HTMLElement}
 */
function voiceRow(current) {
  const known = state.voices || [];
  const select = el('select');
  const options = known.length ? known.slice() : [{ name: current }];
  if (!options.some((o) => o.name === current)) options.unshift({ name: current });
  for (const o of options) {
    const option = el('option', null, voiceLabel(o));
    option.value = o.name;
    select.append(option);
  }
  select.value = current;
  select.disabled = !known.length;
  select.title = current;
  select.addEventListener('change', () => act('voice', { voice: select.value }));
  const row = el('div', 'clip', el('span', 'kind', 'voice'), select);
  if (state.voicesNote) row.append(el('span', 'note', state.voicesNote));
  return row;
}

async function loadVoices() {
  state.voices = [];
  let body;
  try {
    body = await api.loadVoices();
  } catch (err) {
    if (!(err instanceof api.ApiError)) {
      failed(err);
      return;
    }
    state.voicesNote = err.message;
    body = null;
  }
  if (body) {
    state.voices = body.voices || [];
    state.voicesNote = state.voices.length ? '' : 'no voices to choose from';
  }
  if (state.mode !== 'edit' && !state.finished) renderStage();
}

/**
 * @returns {HTMLElement}
 */
function doneView() {
  const v = /** @type {import('./types.js').View} */ (state.view);
  const s = v.summary;
  const done = s.open === 0;
  const box = el('section', 'done',
    el('h2', done ? 'all' : '', done ? '✓ All ' + notesWord(v, s.total) + ' are done' : s.total - s.open + ' of ' + notesWord(v, s.total) + ' are done'),
    tally(s));
  if (v.next) box.append(el('p', 'next', v.next));
  return box;
}

/**
 * @returns {HTMLElement}
 */
function finishedView() {
  const v = /** @type {import('./types.js').View} */ (state.view);
  const s = state.summary || v.summary;
  const box = el('section', 'finished', el('h2', null, state.ended ? 'The review has ended' : 'Review finished'), tally(s));
  box.append(el('p', 'next', state.ended
    ? 'The review was stopped in the terminal, which printed the summary. You can close this tab.'
    : 'The summary is in your terminal too. You can close this tab.'));
  return box;
}

async function start() {
  let view;
  try {
    view = await api.loadState('');
  } catch (err) {
    failed(err);
    return;
  }
  state.view = view;
  const first = view.list.find((x) => x.state === 'open') || view.list[0];
  if (first) {
    await load(first.id);
  } else {
    render();
  }
}

$('finish').addEventListener('click', finish);
start();

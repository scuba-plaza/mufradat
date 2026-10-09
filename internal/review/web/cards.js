import { arabic, el, plural, spans } from './dom.js';
import { metaOf } from './labels.js';
import { composeExample, markupSpans } from './example.js';

/**
 * @param {string} tone
 * @param {...(Node | string | null)} kids
 * @returns {HTMLDivElement}
 */
export function banner(tone, ...kids) {
  return el('div', 'banner tone-' + tone, ...kids);
}

/**
 * @param {import('./types.js').Card} c
 * @returns {HTMLElement}
 */
export function cardView(c) {
  const card = el('section', 'card',
    el('div', 'card-head', el('div', 'headword', arabic('word', c.head)), el('div', 'meta', c.meta)),
    el('div', 'meaning', c.english || '—'));
  if (c.hint) card.append(el('div', 'hint', c.hint));
  if (c.forms && c.forms.length) {
    const forms = el('div', 'forms');
    for (const f of c.forms) forms.append(el('span', 'form', el('span', 'lbl', f.label), arabic('', f.spans)));
    card.append(forms);
  }
  card.append(el('div', 'example',
    el('div', 'sentence-row', arabic('sentence', c.example)),
    el('div', 'translation', c.example_en)));
  if (c.comment) card.append(el('div', 'comment', c.comment));
  return card;
}

/**
 * @param {import('./types.js').Note} draft
 * @param {import('./types.js').Parts | null} parts
 * @param {import('./types.js').Note} original
 * @returns {import('./types.js').Card}
 */
export function previewCard(draft, parts, original) {
  const example = parts ? composeExample(parts) ?? parts.sentence : draft.example || '';
  return {
    head: [{ t: draft.lemma || '' }],
    meta: metaOf({ ...draft, rank: original.rank, origin: original.origin }),
    english: draft.english,
    hint: draft.hint,
    forms: (draft.forms || [])
      .filter((f) => f.label || f.ar)
      .map((f) => ({ label: f.label, spans: [{ t: f.ar }] })),
    example: markupSpans(example),
    example_en: draft.example_en,
    comment: draft.comment,
  };
}

/**
 * @param {import('./types.js').Flag} f
 * @returns {HTMLElement}
 */
export function flagView(f) {
  const block = el('article', 'flag tone-' + f.tone + (f.faded ? ' faded' : ''),
    el('header', null, el('span', 'symbol', f.symbol), el('h3', null, f.title), el('span', 'where', f.where)));
  if (f.rows.length) {
    const rows = el('dl');
    for (const r of f.rows) {
      const value = el('dd');
      if (r.spans.length) value.append(r.rtl ? arabic('value', r.spans) : el('span', 'value', spans(r.spans)));
      if (r.note) value.append(el('span', 'note', r.note));
      rows.append(el('dt', null, r.label), value);
    }
    block.append(rows);
  }
  if (f.explain && !f.faded) block.append(el('p', 'explain', f.explain));
  return block;
}

/**
 * @param {string} kind
 * @param {string} sign
 * @param {import('./types.js').Span[]} list
 * @param {boolean} rtl
 * @returns {HTMLElement}
 */
function changeLine(kind, sign, list, rtl) {
  let text;
  if (!list || !list.length) {
    text = el('span', 'text none', '(none)');
  } else if (rtl) {
    text = arabic('text', list);
  } else {
    text = el('span', 'text', spans(list));
  }
  return el('div', 'line ' + kind, el('span', 'sign', sign), text);
}

/**
 * @param {import('./types.js').Entry} e
 * @returns {HTMLElement}
 */
export function proposalView(e) {
  const side = el('section', 'side', el('h2', 'side-title claude', '✦ Claude Code suggests this version'));
  for (const c of e.changes || []) {
    side.append(el('article', 'change', el('h3', null, c.label), changeLine('old', '−', c.old, c.rtl), changeLine('new', '+', c.new, c.rtl)));
  }
  side.append(el('p', 'side-note', 'Fields that did not change are not shown.'));
  return side;
}

/**
 * @param {import('./types.js').Summary} s
 * @returns {HTMLElement}
 */
export function tally(s) {
  const rows = el('div', 'tally');
  /**
   * @param {number} n
   * @param {string} symbol
   * @param {string} cls
   * @param {string} text
   */
  const add = (n, symbol, cls, text) => {
    if (!n) return;
    rows.append(el('span', 'n', String(n)), el('span', 'sym ' + cls, symbol), el('span', null, text));
  };
  add(s.kept, '✓', 'icon good', 'right as they were');
  add(s.edited, '✎', 'icon good', 'edited by you');
  add(s.rewritten, '✦', 'icon claude', 'rewritten by Claude Code');
  add(s.open, '○', '', 'still open');
  return rows;
}

/**
 * @param {import('./types.js').View} v
 * @param {number} n
 * @returns {string}
 */
export function notesWord(v, n) {
  return v.all ? plural(n, 'note', 'notes') : plural(n, 'flagged note', 'flagged notes');
}

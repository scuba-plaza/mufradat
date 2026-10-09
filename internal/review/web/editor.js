import { el } from './dom.js';
import { GENDER_LABEL, POS, POS_LABEL } from './labels.js';

let fieldCount = 0;

/**
 * @param {string} label
 * @param {HTMLElement} control
 * @param {string} [help]
 * @returns {HTMLDivElement}
 */
function field(label, control, help) {
  fieldCount += 1;
  const id = 'field-' + fieldCount;
  const box = el('div', 'field');
  if (control instanceof HTMLInputElement || control instanceof HTMLTextAreaElement || control instanceof HTMLSelectElement) {
    control.id = id;
    const lab = el('label', null, label);
    lab.htmlFor = id;
    box.append(lab, control);
  } else {
    box.append(el('span', 'label', label), control);
  }
  if (help) box.append(el('span', 'help', help));
  return box;
}

/**
 * @template {HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement} T
 * @param {T} node
 * @param {Record<string, unknown>} target
 * @param {string} key
 * @param {() => void} changed
 * @returns {T}
 */
function bind(node, target, key, changed) {
  const current = target[key];
  node.value = typeof current === 'string' ? current : '';
  node.addEventListener('input', () => {
    target[key] = node.value;
    changed();
  });
  return node;
}

/**
 * @param {Record<string, unknown>} target
 * @param {string} key
 * @param {boolean} rtl
 * @param {() => void} changed
 * @returns {HTMLInputElement}
 */
function textInput(target, key, rtl, changed) {
  const node = bind(el('input', rtl ? 'ar-input' : ''), target, key, changed);
  node.type = 'text';
  if (rtl) node.dir = 'rtl';
  return node;
}

/**
 * @param {Record<string, unknown>} target
 * @param {string} key
 * @param {boolean} rtl
 * @param {number} rows
 * @param {() => void} changed
 * @returns {HTMLTextAreaElement}
 */
function textArea(target, key, rtl, rows, changed) {
  const node = el('textarea', rtl ? 'ar-input' : '');
  node.rows = rows;
  if (rtl) node.dir = 'rtl';
  return bind(node, target, key, changed);
}

/**
 * @param {Record<string, unknown>} target
 * @param {string} key
 * @param {[string, string][]} options
 * @param {() => void} changed
 * @returns {HTMLSelectElement}
 */
function choice(target, key, options, changed) {
  const node = el('select');
  for (const [value, label] of options) {
    const option = el('option', null, label);
    option.value = value;
    node.append(option);
  }
  const current = target[key];
  if (typeof current === 'string' && current && !options.some(([value]) => value === current)) {
    const option = el('option', null, current);
    option.value = current;
    node.append(option);
  }
  return bind(node, target, key, changed);
}

/**
 * @param {import('./types.js').Form[]} forms
 * @param {() => void} changed
 * @returns {HTMLDivElement}
 */
function formsEditor(forms, changed) {
  const box = el('div', 'pairs');
  const draw = () => {
    box.replaceChildren();
    forms.forEach((f, i) => {
      const label = el('input');
      label.type = 'text';
      label.value = f.label;
      label.placeholder = 'pl.';
      label.addEventListener('input', () => {
        f.label = label.value;
        changed();
      });
      const word = el('input', 'ar-input');
      word.type = 'text';
      word.dir = 'rtl';
      word.value = f.ar;
      word.addEventListener('input', () => {
        f.ar = word.value;
        changed();
      });
      const remove = el('button', 'small', '×');
      remove.type = 'button';
      remove.title = 'Remove this form';
      remove.addEventListener('click', () => {
        forms.splice(i, 1);
        draw();
        changed();
      });
      box.append(el('div', 'pair', label, word, remove));
    });
    const add = el('button', 'small', '+ Add a form');
    add.type = 'button';
    add.addEventListener('click', () => {
      forms.push({ label: '', ar: '' });
      draw();
    });
    box.append(add);
  };
  draw();
  return box;
}

/**
 * @param {import('./types.js').Note} draft
 * @param {import('./types.js').Parts | null} parts
 * @param {() => void} changed
 * @returns {HTMLDivElement[]}
 */
function sentenceFields(draft, parts, changed) {
  if (!parts) {
    return [field('Sentence', textArea(draft, 'example', true, 2, changed), 'Put the word itself in [square brackets]; the card on the left shows how it will look.')];
  }
  return [
    field('Sentence', textArea(parts, 'sentence', true, 2, changed)),
    field('Word to highlight', textInput(parts, 'word', true, changed), 'The form of the word as it appears in the sentence; the card shows it in colour.'),
  ];
}

/**
 * @param {import('./types.js').Note} draft
 * @param {import('./types.js').Parts | null} parts
 * @param {() => void} changed
 * @param {() => void} submit
 * @returns {HTMLFormElement}
 */
export function editorView(draft, parts, changed, submit) {
  const form = el('form', 'side editor');
  form.addEventListener('submit', (ev) => {
    ev.preventDefault();
    submit();
  });
  form.append(
    el('h2', 'side-title', '✎ Edit the note'),
    field('Word', textInput(draft, 'lemma', true, changed)),
    el('div', 'grid3',
      field('Type', choice(draft, 'pos', POS.map((p) => [p, POS_LABEL[p] || p]), changed)),
      field('Gender', choice(draft, 'gender', [['', '—'], ...Object.keys(GENDER_LABEL).map((g) => /** @type {[string, string]} */ ([g, { m: 'masculine', f: 'feminine', 'm+f': 'both' }[g] || g]))], changed)),
      field('Verb form', textInput(draft, 'verb_form', false, changed))),
    field('Meaning', textInput(draft, 'english', false, changed)),
    field('Hint', textInput(draft, 'hint', false, changed)),
    field('Forms', formsEditor(/** @type {import('./types.js').Form[]} */ (draft.forms), changed)),
    ...sentenceFields(draft, parts, changed),
    field('Translation', textArea(draft, 'example_en', false, 2, changed)),
    field('Comment', textArea(draft, 'comment', false, 2, changed)),
  );
  return form;
}

/**
 * @param {import('./types.js').Note} draft
 * @param {string} example
 * @returns {Record<string, unknown>}
 */
export function editBody(draft, example) {
  /**
   * @param {string | undefined} s
   * @returns {string}
   */
  const trim = (s) => (s || '').trim();
  return {
    lemma: trim(draft.lemma),
    pos: draft.pos,
    gender: trim(draft.gender),
    verb_form: trim(draft.verb_form),
    root: trim(draft.root),
    forms: (draft.forms || []).map((f) => ({ label: trim(f.label), ar: trim(f.ar) })).filter((f) => f.label || f.ar),
    english: trim(draft.english),
    hint: trim(draft.hint),
    example: trim(example),
    example_en: trim(draft.example_en),
    comment: trim(draft.comment),
  };
}

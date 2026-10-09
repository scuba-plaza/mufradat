/**
 * @param {string} id
 * @returns {HTMLElement}
 */
export function $(id) {
  const node = document.getElementById(id);
  if (!node) throw new Error('the page has no #' + id);
  return node;
}

/**
 * @template {keyof HTMLElementTagNameMap} K
 * @param {K} tag
 * @param {string | null} [cls]
 * @param {...(Node | string | null | undefined | false)} kids
 * @returns {HTMLElementTagNameMap[K]}
 */
export function el(tag, cls, ...kids) {
  const node = document.createElement(tag);
  if (cls) node.className = cls;
  for (const kid of kids) {
    if (kid !== null && kid !== undefined && kid !== false) node.append(kid);
  }
  return node;
}

/**
 * @param {import('./types.js').Span[] | undefined} list
 * @returns {DocumentFragment}
 */
export function spans(list) {
  const frag = document.createDocumentFragment();
  for (const s of list || []) {
    frag.append(s.c ? el('span', s.c, s.t) : s.t);
  }
  return frag;
}

/**
 * @param {string} cls
 * @param {import('./types.js').Span[]} list
 * @returns {HTMLSpanElement}
 */
export function arabic(cls, list) {
  const node = el('span', 'ar' + (cls ? ' ' + cls : ''), spans(list));
  node.dir = 'rtl';
  return node;
}

/**
 * @param {number} n
 * @param {string} one
 * @param {string} many
 * @returns {string}
 */
export function plural(n, one, many) {
  return n + ' ' + (n === 1 ? one : many);
}

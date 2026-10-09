/**
 * @param {string | undefined} markup
 * @returns {import('./types.js').Parts | null}
 */
export function splitExample(markup) {
  const m = /^([^[\]]*)\[([^[\]]*)\]([^[\]]*)$/.exec(markup || '');
  return m ? { sentence: m[1] + m[2] + m[3], word: m[2], at: m[1].length } : null;
}

/**
 * @param {import('./types.js').Parts} parts
 * @returns {string | null}
 */
export function composeExample(parts) {
  const word = parts.word.trim();
  if (!word) return parts.sentence;
  const at = parts.sentence.substr(parts.at, word.length) === word ? parts.at : parts.sentence.indexOf(word);
  if (at < 0) return null;
  return parts.sentence.slice(0, at) + '[' + word + ']' + parts.sentence.slice(at + word.length);
}

/**
 * @param {string} text
 * @returns {import('./types.js').Span[]}
 */
export function markupSpans(text) {
  /** @type {import('./types.js').Span[]} */
  const out = [];
  let target = false;
  for (const part of text.split(/(\[|\])/)) {
    if (part === '[') {
      target = true;
    } else if (part === ']') {
      target = false;
    } else if (part) {
      out.push({ t: part, c: target ? 'target' : '' });
    }
  }
  return out;
}

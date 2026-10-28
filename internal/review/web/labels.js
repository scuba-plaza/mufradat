export const POS = ['noun', 'verb', 'adj', 'adv', 'prep', 'conj', 'particle', 'pron', 'num', 'intj', 'phrase', 'det'];

/** @type {Record<string, string>} */
export const POS_LABEL = {
  adj: 'adjective',
  adv: 'adverb',
  prep: 'preposition',
  conj: 'conjunction',
  pron: 'pronoun',
  num: 'number',
  intj: 'interjection',
  det: 'determiner',
};

/** @type {Record<string, string>} */
export const GENDER_LABEL = { m: 'masc.', f: 'fem.', 'm+f': 'masc./fem.' };

/** @type {Record<string, string>} */
export const KEY_LABEL = { enter: 'enter', esc: 'esc', left: '←', right: '→', 'ctrl+enter': 'ctrl+enter' };

/** @type {Record<string, string>} */
export const FIX_LABEL = { catt: "Use CATT's ending", camel: "Use CAMeL's ending", both: 'Add the ending' };

/** @type {Record<string, string>} */
export const PLAY_KEY = { word: 'w', forms: 'f', sentence: 's' };

/** @type {Record<string, string>} */
export const TONES = { info: 'info', good: 'good', bad: 'bad', warn: 'warn', mistral: 'mistral', audio: 'audio' };

/**
 * @param {string | undefined} tone
 * @returns {string}
 */
export function messageTone(tone) {
  return (tone && TONES[tone]) || 'info';
}

/**
 * @param {{ pos: string, gender?: string, verb_form?: string, rank?: number, origin?: string }} note
 * @returns {string}
 */
export function metaOf(note) {
  const parts = [POS_LABEL[note.pos] || note.pos];
  if (note.gender && GENDER_LABEL[note.gender]) parts.push(GENDER_LABEL[note.gender]);
  if (note.verb_form) parts.push('form ' + note.verb_form);
  parts.push(note.rank ? '#' + note.rank : note.origin || '');
  return parts.filter(Boolean).join(' · ');
}

/**
 * @param {{ name: string, label?: string }} v
 * @returns {string}
 */
export function voiceName(v) {
  return v.label || v.name;
}

/**
 * @param {import('./types.js').Voice[]} voices
 * @returns {{ tier: string, voices: import('./types.js').Voice[] }[]}
 */
export function groupVoices(voices) {
  const tiered = voices.some((v) => v.tier);
  /** @type {Map<string, import('./types.js').Voice[]>} */
  const groups = new Map();
  for (const v of voices) {
    const tier = v.tier || (tiered ? 'other' : '');
    groups.set(tier, [...(groups.get(tier) || []), v]);
  }
  return [...groups].map(([tier, members]) => ({ tier, voices: members }));
}

/** @type {Record<string, string>} */
export const PROVIDER_LABEL = { mistral: 'Mistral', google: 'Google' };

/**
 * @param {string | undefined} provider
 * @returns {string}
 */
export function providerLabel(provider) {
  if (!provider) return '';
  return PROVIDER_LABEL[provider] || provider;
}

/**
 * @param {import('./types.js').Voice[]} voices
 * @returns {{ provider: string, voices: import('./types.js').Voice[] }[]}
 */
export function groupProviders(voices) {
  /** @type {Map<string, import('./types.js').Voice[]>} */
  const groups = new Map();
  for (const v of voices) {
    const provider = v.provider || '';
    groups.set(provider, [...(groups.get(provider) || []), v]);
  }
  return [...groups].map(([provider, members]) => ({ provider, voices: members }));
}

/**
 * @param {import('./types.js').Voice} voice
 * @param {{ provider: string, voice: string }} inUse
 * @returns {boolean}
 */
export function isInUse(voice, inUse) {
  return voice.provider === inUse.provider && voice.name === inUse.voice;
}

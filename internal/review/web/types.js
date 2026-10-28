/** @typedef {{ t: string, c?: string }} Span */
/** @typedef {{ label: string, spans: Span[], rtl: boolean, note?: string }} Row */
/** @typedef {{ tone: string, symbol: string, title: string, where: string, rows: Row[], explain?: string, faded: boolean }} Flag */
/** @typedef {{ label: string, ar: string }} Form */
/**
 * @typedef {{
 *   id: string, rank?: number, origin: string, lemma: string, pos: string, gender?: string, verb_form?: string,
 *   root?: string, forms?: Form[], english: string, hint?: string, example: string, example_en: string, comment?: string
 * }} Note
 */
/** @typedef {{ label: string, spans: Span[] }} FormView */
/**
 * @typedef {{
 *   head: Span[], meta: string, english: string, hint?: string, forms: FormView[],
 *   example: Span[], example_en: string, comment?: string
 * }} Card
 */
/** @typedef {{ label: string, rtl: boolean, old: Span[], new: Span[] }} Change */
/** @typedef {{ source: string, words: string[] }} Fix */
/** @typedef {{ kind: string, ready: boolean }} Clip */
/**
 * @typedef {{
 *   id: string, rev: string, state: string, changed?: string, asking: boolean, checking: boolean, seconds: number,
 *   verified: boolean, notice?: string, notice_tone?: string, note: Note, card: Card, flags: Flag[],
 *   proposal?: Note, changes?: Change[], fixes?: Fix[], clips: Clip[]
 * }} Entry
 */
/**
 * @typedef {{
 *   id: string, rank?: number, lemma: string, english: string, state: string, tone: string, flags: number,
 *   asking: boolean, checking: boolean, ready: boolean, gone?: boolean, notice?: string, notice_tone?: string
 * }} ListItem
 */
/** @typedef {{ total: number, kept: number, edited: number, rewritten: number, open: number }} Summary */
/**
 * @typedef {{
 *   summary: Summary, next?: string, history: number, mistral: boolean, audio: boolean, verify: boolean,
 *   voice?: string, all: boolean, list: ListItem[], entry?: Entry
 * }} View
 */
/** @typedef {{ provider?: string, name: string, label?: string, tier?: string, gender?: string }} Voice */
/** @typedef {{ id: string, label: string, text: string }} Sample */
/** @typedef {{ voice: string, voices: Voice[] }} VoiceList */
/** @typedef {{ provider: string, voice: string, samples: Sample[], voices: Voice[], notices: string[] }} Comparison */
/** @typedef {{ message?: string, tone?: string, show?: string, conflict?: boolean }} Result */
/** @typedef {{ sentence: string, word: string, at: number }} Parts */
/** @typedef {{ key: string, label: string, run: () => unknown, primary?: boolean, tone?: string, push?: boolean, nav?: boolean, arabic?: string }} Action */

export {};

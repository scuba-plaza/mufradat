import { $, arabic, el } from './dom.js';
import { groupProviders, groupVoices, isInUse, messageTone, providerLabel, voiceName } from './labels.js';
import * as api from './api.js';

/** @typedef {import('./types.js').Comparison} Comparison */
/** @typedef {{ provider: string, voice: string }} Choice */

const player = new Audio();

/** @type {{ view: Comparison | null, playing: string, used: Choice | null }} */
const state = { view: null, playing: '', used: null };

/**
 * @param {string} text
 * @param {string} [tone]
 */
function say(text, tone) {
  const message = $('message');
  message.className = 'message ' + messageTone(tone);
  message.textContent = text;
}

/**
 * @param {string} provider
 * @param {string} name
 * @returns {string}
 */
function describe(provider, name) {
  return provider ? name + ' (' + providerLabel(provider) + ')' : name;
}

/**
 * @param {import('./types.js').Voice} voice
 * @param {import('./types.js').Sample} sample
 * @param {HTMLButtonElement} button
 */
function play(voice, sample, button) {
  const who = describe(voice.provider || '', voiceName(voice));
  for (const b of document.querySelectorAll('.plays .small.playing')) b.classList.remove('playing');
  player.pause();
  state.playing = (voice.provider || '') + '/' + voice.name + '/' + sample.id;
  button.classList.add('playing');
  say('♪ Making the ' + sample.label + ' with ' + who + '…', 'audio');
  player.src = api.sampleURL(voice.provider || '', voice.name, sample.id);
  player.play().then(
    () => say('▶ ' + who + ' · ' + sample.label, 'audio'),
    (err) => {
      button.classList.remove('playing');
      say('Could not play it: ' + (err && err.message ? err.message : 'the service refused'), 'bad');
    },
  );
}

player.addEventListener('ended', () => {
  state.playing = '';
  for (const b of document.querySelectorAll('.plays .small.playing')) b.classList.remove('playing');
});

/**
 * @param {import('./types.js').Voice} voice
 */
async function use(voice) {
  const provider = voice.provider || '';
  try {
    const res = await api.use(provider, voice.name);
    state.used = { provider, voice: voice.name };
    if (state.view) {
      state.view.provider = provider;
      state.view.voice = voice.name;
    }
    say(res.message || describe(provider, voice.name), messageTone(res.tone));
    render();
  } catch (err) {
    say(err instanceof Error ? err.message : String(err), 'bad');
  }
}

/**
 * @param {import('./types.js').Voice} v
 * @returns {string}
 */
function genderOf(v) {
  return v.gender ? v.gender.toLowerCase() : '';
}

/**
 * @param {Comparison} view
 * @param {import('./types.js').Voice} v
 * @returns {HTMLElement}
 */
function voiceRow(view, v) {
  const current = isInUse(v, view);
  const plays = el('div', 'plays');
  for (const s of view.samples) {
    const button = el('button', 'small', '▶ ' + s.label);
    button.type = 'button';
    button.addEventListener('click', () => play(v, s, button));
    plays.append(button);
  }
  const choose = el('button', 'small use', current ? '✓ In use' : 'Use this voice');
  choose.type = 'button';
  choose.disabled = current;
  choose.addEventListener('click', () => use(v));
  return el('div', 'voice' + (current ? ' current' : ''),
    el('span', 'name', voiceName(v) + (genderOf(v) ? ' · ' + genderOf(v) : '')), plays, choose);
}

function render() {
  const view = state.view;
  if (!view) return;
  $('current').textContent = 'in use: ' + describe(view.provider, view.voice);
  const root = $('voices');
  const texts = el('section', 'sample-text');
  for (const s of view.samples) texts.append(el('span', 'kind', s.label), arabic('', [{ t: s.text }]));
  const children = [texts];
  for (const notice of view.notices || []) children.push(el('p', 'notice', notice));
  for (const { provider, voices } of groupProviders(view.voices)) {
    if (provider) children.push(el('h2', 'provider', providerLabel(provider)));
    for (const { tier, voices: members } of groupVoices(voices)) {
      if (tier) children.push(el('h3', 'tier', tier));
      for (const v of members) children.push(voiceRow(view, v));
    }
  }
  root.replaceChildren(...children);
}

async function finish() {
  try {
    await api.finish();
  } catch {
    state.used = state.used || null;
  }
  player.pause();
  const used = state.used;
  $('voices').replaceChildren(el('section', 'finished',
    el('h2', null, 'Done'),
    el('p', 'next', used ? describe(used.provider, used.voice) + ' speaks every clip made from now on. You can close this tab.' : 'The voice was not changed. You can close this tab.')));
  $('finish').hidden = true;
}

async function start() {
  try {
    state.view = await api.loadComparison();
  } catch (err) {
    say(err instanceof Error ? err.message : String(err), 'bad');
    return;
  }
  render();
  say('Press a ▶ button to hear a voice; clips are made when you ask and kept for next time', 'info');
}

$('finish').addEventListener('click', finish);
start();

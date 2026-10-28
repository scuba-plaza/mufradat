import { $, arabic, el } from './dom.js';
import { groupVoices, messageTone, voiceName } from './labels.js';
import * as api from './api.js';

/** @typedef {{ voice: string, samples: import('./types.js').Sample[], voices: import('./types.js').Voice[] }} Comparison */

const player = new Audio();

/** @type {{ view: Comparison | null, playing: string, used: string }} */
const state = { view: null, playing: '', used: '' };

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
 * @param {string} voice
 * @param {import('./types.js').Sample} sample
 * @param {HTMLButtonElement} button
 */
function play(voice, sample, button) {
  for (const b of document.querySelectorAll('.plays .small.playing')) b.classList.remove('playing');
  player.pause();
  state.playing = voice + '/' + sample.id;
  button.classList.add('playing');
  say('♪ Making the ' + sample.label + ' with ' + voice + '…', 'audio');
  player.src = api.sampleURL(voice, sample.id);
  player.play().then(
    () => say('▶ ' + voice + ' · ' + sample.label, 'audio'),
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
 * @param {string} voice
 */
async function use(voice) {
  try {
    const res = await api.use(voice);
    state.used = voice;
    if (state.view) state.view.voice = voice;
    say(res.message || voice, messageTone(res.tone));
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

function render() {
  const view = state.view;
  if (!view) return;
  $('current').textContent = 'current voice: ' + view.voice;
  const root = $('voices');
  const texts = el('section', 'sample-text');
  for (const s of view.samples) texts.append(el('span', 'kind', s.label), arabic('', [{ t: s.text }]));
  const children = [texts];
  for (const { tier, voices } of groupVoices(view.voices)) {
    if (tier) children.push(el('h2', 'tier', tier));
    for (const v of voices) {
      const plays = el('div', 'plays');
      for (const s of view.samples) {
        const button = el('button', 'small', '▶ ' + s.label);
        button.type = 'button';
        button.addEventListener('click', () => play(v.name, s, button));
        plays.append(button);
      }
      const choose = el('button', 'small use', v.name === view.voice ? '✓ In use' : 'Use this voice');
      choose.type = 'button';
      choose.disabled = v.name === view.voice;
      choose.addEventListener('click', () => use(v.name));
      children.push(el('div', 'voice' + (v.name === view.voice ? ' current' : ''),
        el('span', 'name', voiceName(v) + (genderOf(v) ? ' · ' + genderOf(v) : '')), plays, choose));
    }
  }
  root.replaceChildren(...children);
}

async function finish() {
  try {
    await api.finish();
  } catch {
    state.used = state.used || '';
  }
  player.pause();
  $('voices').replaceChildren(el('section', 'finished',
    el('h2', null, 'Done'),
    el('p', 'next', state.used ? state.used + ' speaks every clip made from now on. You can close this tab.' : 'The voice was not changed. You can close this tab.')));
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

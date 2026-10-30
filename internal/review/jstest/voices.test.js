import assert from 'node:assert/strict';
import { test } from 'node:test';

import { groupVoices, voiceName } from '../web/labels.js';

test('voiceName shows the label the server chose, else the name', () => {
  assert.equal(voiceName({ name: 'ar-XA-Wavenet-A', label: 'Wavenet-A' }), 'Wavenet-A');
  assert.equal(voiceName({ name: 'ar-XA-Wavenet-A' }), 'ar-XA-Wavenet-A');
  assert.equal(voiceName({ name: 'x', label: '' }), 'x');
});

test('groupVoices groups by tier in the order tiers first appear', () => {
  const voices = [
    { name: 'a', tier: 'Chirp3' },
    { name: 'b', tier: 'Wavenet' },
    { name: 'c', tier: 'Chirp3' },
  ];
  assert.deepEqual(groupVoices(voices), [
    { tier: 'Chirp3', voices: [voices[0], voices[2]] },
    { tier: 'Wavenet', voices: [voices[1]] },
  ]);
});

test('groupVoices gives voices without any tier one group with no heading', () => {
  const voices = [{ name: 'ar-XA-A', label: 'ar-XA-A' }, { name: 'ar-XA-B', label: 'ar-XA-B' }];
  assert.deepEqual(groupVoices(voices), [{ tier: '', voices }]);
});

test('groupVoices files untiered voices under other when some voices have a tier', () => {
  const voices = [{ name: 'a', tier: 'Chirp3' }, { name: 'custom' }];
  assert.deepEqual(groupVoices(voices), [
    { tier: 'Chirp3', voices: [voices[0]] },
    { tier: 'other', voices: [voices[1]] },
  ]);
});

test('groupVoices of nothing is nothing', () => {
  assert.deepEqual(groupVoices([]), []);
});

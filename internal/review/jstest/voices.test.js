import assert from 'node:assert/strict';
import { test } from 'node:test';

import { groupProviders, groupVoices, isInUse, providerLabel, voiceName } from '../web/labels.js';

test('voiceName shows the label the server chose, else the name', () => {
  assert.equal(voiceName({ name: 'voice_479', label: 'Maryam' }), 'Maryam');
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
  const voices = [{ name: 'voice_479', label: 'Maryam' }, { name: 'voice_480', label: 'Layla' }];
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

test('groupProviders groups by provider in the order providers first appear', () => {
  const voices = [
    { provider: 'mistral', name: 'a' },
    { provider: 'google', name: 'b' },
    { provider: 'mistral', name: 'c' },
  ];
  assert.deepEqual(groupProviders(voices), [
    { provider: 'mistral', voices: [voices[0], voices[2]] },
    { provider: 'google', voices: [voices[1]] },
  ]);
});

test('groupProviders keeps the same name under two providers apart', () => {
  const voices = [{ provider: 'mistral', name: 'x' }, { provider: 'google', name: 'x' }];
  assert.deepEqual(groupProviders(voices).map((g) => g.provider), ['mistral', 'google']);
});

test('groupProviders gives voices with no provider one group with no heading, and nothing gives nothing', () => {
  const voices = [{ name: 'a' }, { name: 'b' }];
  assert.deepEqual(groupProviders(voices), [{ provider: '', voices }]);
  assert.deepEqual(groupProviders([]), []);
});

test('providerLabel names the providers people know and leaves others as they are', () => {
  assert.equal(providerLabel('mistral'), 'Mistral');
  assert.equal(providerLabel('google'), 'Google');
  assert.equal(providerLabel('azure'), 'azure');
  assert.equal(providerLabel(''), '');
  assert.equal(providerLabel(undefined), '');
});

test('isInUse needs both the provider and the name to match', () => {
  const inUse = { provider: 'google', voice: 'ar-XA-Wavenet-A' };
  assert.equal(isInUse({ provider: 'google', name: 'ar-XA-Wavenet-A' }, inUse), true);
  assert.equal(isInUse({ provider: 'mistral', name: 'ar-XA-Wavenet-A' }, inUse), false);
  assert.equal(isInUse({ provider: 'google', name: 'ar-XA-Wavenet-B' }, inUse), false);
  assert.equal(isInUse({ name: 'ar-XA-Wavenet-A' }, inUse), false);
});

import assert from 'node:assert/strict';
import { test } from 'node:test';

import { composeExample, markupSpans, splitExample } from '../web/example.js';
import { editBody } from '../web/editor.js';
import { messageTone, metaOf } from '../web/labels.js';

test('splitExample finds the highlighted word and where it starts', () => {
  assert.deepEqual(splitExample('قَرَأْتُ [الْكِتَابَ] فِي الْبَيْتِ.'), {
    sentence: 'قَرَأْتُ الْكِتَابَ فِي الْبَيْتِ.',
    word: 'الْكِتَابَ',
    at: 'قَرَأْتُ '.length,
  });
  assert.equal(splitExample('no brackets'), null);
  assert.equal(splitExample('[a] and [b]'), null);
  assert.equal(splitExample('[[a]]'), null);
  assert.equal(splitExample(undefined), null);
});

test('composeExample puts the brackets back, following the word when the sentence moved', () => {
  const parts = splitExample('قَرَأْتُ [الْكِتَابَ] فِي الْبَيْتِ.');
  assert.ok(parts);
  assert.equal(composeExample(parts), 'قَرَأْتُ [الْكِتَابَ] فِي الْبَيْتِ.');

  parts.sentence = 'الْيَوْمَ ' + parts.sentence;
  assert.equal(composeExample(parts), 'الْيَوْمَ قَرَأْتُ [الْكِتَابَ] فِي الْبَيْتِ.');

  parts.word = 'غَيْرُهُ';
  assert.equal(composeExample(parts), null);

  parts.word = '  ';
  assert.equal(composeExample(parts), parts.sentence);
});

test('markupSpans marks the target word and nothing else', () => {
  assert.deepEqual(markupSpans('a [b] c'), [
    { t: 'a ', c: '' },
    { t: 'b', c: 'target' },
    { t: ' c', c: '' },
  ]);
  assert.deepEqual(markupSpans(''), []);
});

test('editBody trims, drops empty forms and sends only editable fields', () => {
  const body = editBody(
    {
      id: 'n_aaaaaaaaaaa2', rank: 5, origin: 'ranked', lemma: ' كِتَابٌ ', pos: 'noun', english: ' book ',
      forms: [{ label: ' pl. ', ar: ' كُتُبٌ ' }, { label: '', ar: ' ' }], example: 'x', example_en: ' I read. ',
    },
    ' قَرَأْتُ [الْكِتَابَ]. ',
  );
  assert.deepEqual(body, {
    lemma: 'كِتَابٌ', pos: 'noun', gender: '', verb_form: '', root: '', forms: [{ label: 'pl.', ar: 'كُتُبٌ' }],
    english: 'book', hint: '', example: 'قَرَأْتُ [الْكِتَابَ].', example_en: 'I read.', comment: '',
  });
});

test('metaOf joins the part of speech, gender, verb form and rank or origin', () => {
  assert.equal(metaOf({ pos: 'adj', gender: 'm', rank: 12 }), 'adjective · masc. · #12');
  assert.equal(metaOf({ pos: 'verb', verb_form: 'II', origin: 'named' }), 'verb · form II · named');
  assert.equal(metaOf({ pos: 'noun', gender: 'f', rank: 3 }), 'noun · fem. · #3');
  assert.equal(metaOf({ pos: 'noun', gender: 'm+f', origin: 'ranked' }), 'noun · masc./fem. · ranked');
  assert.equal(metaOf({ pos: 'noun', gender: 'x', rank: 1 }), 'noun · #1');
  assert.equal(metaOf({ pos: 'banana' }), 'banana');
  assert.equal(metaOf({ pos: 'prep', rank: 0, origin: 'named' }), 'preposition · named');
});

test('messageTone keeps every tone the server sends and shows anything else as info', () => {
  for (const tone of ['info', 'good', 'bad', 'warn', 'mistral', 'audio']) {
    assert.equal(messageTone(tone), tone);
  }
  assert.equal(messageTone('banana'), 'info');
  assert.equal(messageTone(''), 'info');
  assert.equal(messageTone(undefined), 'info');
});

test('composeExample finds a word at the very start of a sentence that moved under it', () => {
  const parts = { sentence: 'الْكِتَابَ جَدِيدٌ.', word: 'الْكِتَابَ', at: 7 };
  assert.equal(composeExample(parts), '[الْكِتَابَ] جَدِيدٌ.');
});

test('editBody keeps half-filled forms and leaves the part of speech to the server to judge', () => {
  const body = editBody(
    { id: 'n_aaaaaaaaaaa2', rank: 0, origin: 'named', lemma: 'كِتَاب', pos: '', english: 'book', forms: [{ label: 'pl.', ar: '' }, { label: '', ar: 'كُتُب' }, { label: ' ', ar: ' ' }], example: '', example_en: '' },
    '',
  );
  assert.deepEqual(body.forms, [{ label: 'pl.', ar: '' }, { label: '', ar: 'كُتُب' }]);
  assert.equal(body.pos, '');
});

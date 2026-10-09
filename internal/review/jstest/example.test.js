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

test('labels', () => {
  assert.equal(metaOf({ pos: 'adj', gender: 'm', rank: 12 }), 'adjective · masc. · #12');
  assert.equal(metaOf({ pos: 'verb', verb_form: 'II', origin: 'named' }), 'verb · form II · named');
  assert.equal(messageTone('good'), 'good');
  assert.equal(messageTone('warn'), 'info');
  assert.equal(messageTone(undefined), 'info');
});

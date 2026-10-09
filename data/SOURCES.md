# Data sources

Everything in this directory is licensed under CC BY-SA 4.0
(https://creativecommons.org/licenses/by-sa/4.0/) because it is derived from
sources under that licence. The code that builds it (`cmd/mkranking`) is MIT.

| File | What it is | Derived from |
|---|---|---|
| `ranking.tsv` | the 5000 most useful Modern Standard Arabic lemmas, in order | Wiktionary, both frequency lists, CAMeL Tools analyses, `curation/` |
| `lexicon.jsonl.gz` | every MSA lemma with its entries, forms of speech and up to 6 senses each; the ranked lemmas come first, in rank order | Wiktionary |
| `forms.tsv.gz` | the inflected spellings of those lemmas, used to recognise words in example sentences | Wiktionary |
| `curation/essentials.tsv` | greetings, numbers and days of the week pinned to an early rank | written for this project |
| `curation/overrides.tsv` | hand decisions for spellings that belong to more than one lemma | written for this project |
| `sources.jsonl` | the URL, size and sha256 of every input the files above were built from | |

## Inputs

### Wiktionary, via kaikki.org

- URL: https://kaikki.org/dictionary/Arabic/kaikki.org-dictionary-Arabic.jsonl
- Licence: Wiktionary text is available under CC BY-SA 4.0
  (https://en.wiktionary.org/wiki/Wiktionary:Copyrights). kaikki.org publishes
  the dump that wiktextract extracts from it and states no further terms.
- Used for: headwords with full vowel marks, parts of speech, glosses, genders,
  plurals, verb forms, roots and the inflection tables behind `forms.tsv.gz`.
- Citation requested by kaikki.org: Tatu Ylonen, *Wiktextract: Wiktionary as
  Machine-Readable Structured Data*, LREC 2022, pp. 1317-1325.
- kaikki.org regenerates the file regularly, so its content changes. The sha256
  in `sources.jsonl` identifies the copy the committed data was built from.
- Senses tagged obsolete, archaic, rare, Classical, dialectal, regional,
  colloquial, slang, poetic or offensive are not counted as Modern Standard
  Arabic. Entries that are only proper nouns are not shipped, except essentials
  such as the days of the week, which ship as common nouns.

### OpenSubtitles frequency list (Hermit Dave)

- URL: https://raw.githubusercontent.com/hermitdave/FrequencyWords/master/content/2018/ar/ar_50k.txt
- Licence: CC BY-SA 4.0 for the content, MIT for the code
  (https://github.com/hermitdave/FrequencyWords). The counts derive from the
  OpenSubtitles corpus (https://www.opensubtitles.org); the subtitles
  themselves are not included in them and are not shipped here.
- Used for: how often a word form occurs in everyday speech. Only the order it
  contributes to is shipped, not the counts.

### CAMeL Arabic Frequency Lists

- URL: https://github.com/CAMeL-Lab/Camel_Arabic_Frequency_Lists/releases/download/v1.0/MSA_freq_lists.tsv.zip
- Licence: CC BY-SA 4.0 (`LICENSE.txt` in the repository; GitHub shows it as
  "other" because the file is not a standard licence file).
- Used for: how often a word form occurs in written Modern Standard Arabic
  (the first 200000 lines, of which the first 100000 word types are used).
  Only the order is shipped.
- Citation requested by the authors: Khalifa, Inoue, Alhafni, Baimukan,
  Bouamor and Habash, *CAMeL Arabic Frequency Lists*, 2021.

### CAMeL Tools

- Licence: MIT (https://github.com/CAMeL-Lab/camel_tools). Its morphology
  database and disambiguator models are installed by `mufradat setup`
  under their own terms and are not shipped here.
- Used for: lemmatising the frequent word forms (`camel-lemmas.tsv`, not
  shipped, its checksum is in `sources.jsonl`) so that each form's frequency
  is credited to the right lemma.

## Left out on purpose

- The Kelly CEFR word list (https://github.com/kotoshu/frequency-list-kelly) is
  CC BY-NC-SA, which the share-alike licence of this data cannot carry. The
  ranking was built without it, so cards carry no CEFR level.

## How the ranking is made

1. Every word form in the two frequency lists is skeletonised (vowel marks
   removed) and matched against all spellings of all Wiktionary lemmas, with
   clitics (و ف ب ك ل س and the pronoun suffixes) stripped where the lemma's
   part of speech allows it.
2. A form that fits several lemmas is shared between them by an iterative
   estimate that favours lemmas with many MSA senses, particles and lemmas the
   data keeps crediting.
3. CAMeL's disambiguator picks a lemma for each form independently. Where it
   and the Wiktionary match agree the form counts in full, where they differ
   it goes to the one whose spelling is exactly the form, and where neither is
   exact the count is split.
4. A lemma's score is the geometric mean of its frequency per million in the two
   corpora, floored at 0.5.
5. `curation/overrides.tsv` is applied, then `curation/essentials.tsv` moves
   its words up to the rank they name.

`forms.tsv.gz` lists, for each spelling, the lemmas it can belong to, as
`spelling<TAB>lemma:class,...`, where the lemma is a zero-based position in
`lexicon.jsonl.gz` and the class is 0 nominal, 1 definite, 2 construct,
3 perfect, 4 imperfect, 5 imperative, 6 particle.

## Rebuilding

```sh
mufradat setup
go run ./cmd/mkranking
```

Run it from the repository root. It downloads the three inputs into
`~/.cache/mufradat/mkranking` the first time, has CAMeL Tools analyse the
frequent word forms (a few minutes), and rewrites `ranking.tsv`,
`lexicon.jsonl.gz`, `forms.tsv.gz` and `sources.jsonl`. The output is
byte-for-byte reproducible from the same inputs. `-refresh` downloads and
analyses everything again, and `rank-report.tsv` in the cache directory lists
the spellings the two methods disagreed on, most frequent first.

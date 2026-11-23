# @mufradat/regex

A regular expression engine that answers one question, whether a pattern occurs in a text. It is built on the principles of RE2 and Go's `regexp`: the pattern is parsed, compiled to a small program, and run by a lazy DFA that falls back to a Pike NFA. It has no backtracking, so its time never grows exponentially, however the pattern is written.

It is the engine behind the `re:` search of the app, as a workspace package written in TypeScript.

```ts
import { compile } from "@mufradat/regex";

const matcher = compile("kit(a|ā)b");
if ("code" in matcher) throw new Error(matcher.code);
matcher.test("al-kitab");
```

`compile(pattern, limits?)` returns a matcher or an error `{ code, position?, detail? }`; it never throws on a bad pattern. `test(text)` answers whether the pattern occurs anywhere in the text. A limit given as `undefined` keeps its default. There are no match positions, no captures and no flags, and no folding: a text is read by code point and matched exactly as given.

## The pattern language

| Piece | Meaning |
| --- | --- |
| a character | itself, any code point; `\` before ASCII punctuation makes it literal |
| `.` | any code point but `\n` |
| `[abc]` `[a-z]` `[^abc]` | a class; inside it literals, ranges, the Perl classes, `\p{..}` and the escapes |
| `\d` `\w` `\s`, `\D` `\W` `\S` | `\p{Nd}`; letters, numbers and `_` of every script; the white space of ECMAScript; and their opposites |
| `\p{Name}` `\P{Name}` | a general category (`L`, `Lu`, `Nd`, `P`, `Z` and the rest) or a script (`Arabic`, `Latin` and the rest), in Go's spelling, Unicode 17.0 |
| `\n \t \r \f \v` `\xHH` `\x{H...}` `\uHHHH` | one code point |
| `^` `$` `\b` `\B` | start and end of the text, a word edge and its opposite; there is no multiline mode |
| `a\|b` `(...)` | alternation, with empty branches allowed, and groups, which never capture |
| `?` `*` `+` `{n}` `{n,}` `{n,m}` | greedy repeats, with a count of at most 100 |

Everything else is refused with a typed error: backreferences, lookaround, every `(?...)` group, lazy and possessive repeats, POSIX classes, inline flags, `\A \z \Z \G`, `\Q...\E`, octal escapes, a repeat of a repeat, an unknown letter or digit after a backslash, a `{` that does not start a repeat, and an unclosed group or class. The codes and where each is raised are in [CHANGES.md](CHANGES.md), and the whole contract is executable: [test/contract.ts](test/contract.ts) holds 635 rows that every implementation is run through.

## Time and size

The time to answer a text is at most proportional to the length of the text times the size of the program, and the program is at most 20,000 instructions. A pattern whose program would be larger is refused as `too_big` before anything is built, by arithmetic on its tree. That bound is what the Pike executor gives; the lazy DFA, which answers in constant time per character once its states are built, is what runs first.

| Limit | Value |
| --- | --- |
| pattern length | 200 characters |
| repeat count | 100 |
| compiled program | 20,000 instructions |
| nesting of groups | 1,000 |
| DFA cache | 2,000,000 stored entries, about 8 MB, flushed when it is spent |

A matcher that builds more than 2,000 DFA states, at more than one state per 8 characters scanned, runs on the Pike executor for the rest of its life. Every limit raises a typed error and never a hang or a crash.

## How it works

1. **Parse** (`src/syntax`): recursive descent over code points to a syntax tree, with the limits and the typed errors.
2. **Compile** (`src/program`): the tree to a program of `ranges`, `split`, `jump`, `assert` and `match`. A bounded repeat is emitted in the nested form, `x{2,5}` as `xx(x(x(x)?)?)?`, in which every optional copy leaves by one shared exit. Character sets are sorted, merged ranges; `\d`, `\w`, `\s` and `\p{...}` come from tables in `src/unicode`.
3. **Run** (`src/exec`): a lazy DFA whose states are sets of program positions plus the side of the previous character, which is how `^ $ \b \B` are resolved; its transitions are cached per character class and shared by every text the matcher is given. A Pike NFA runs the same program with threads in preallocated typed arrays.

The Unicode tables are generated from V8's own data by `pnpm --filter @mufradat/regex generate:unicode`, for the Unicode version that script names. The tests that compare the tables with V8 are skipped, with a warning, on a Node with another Unicode version; the tables themselves are static and stay correct for the version they name.

## Tests

`pnpm test` at the root runs them all.

- the **contract**, through the parser and a derivative matcher written for the tests alone, through the Pike executor, the DFA and `compile()`;
- the **oracle**: 64,000 seeded random patterns and texts, each answered by the reference matcher, the Pike executor and `compile()` and checked against native `RegExp`, which runs under a `vm` timeout;
- an **exhaustive** run of every pattern of up to four tokens against every text of up to three characters;
- the **complexity pins**: eight patterns that make a backtracking engine exponential, and size bombs, each under a 10 second watchdog;
- the **DFA** itself, with a forced bail-out, and against the Pike executor on random patterns with a cache so small that nearly every build flushes it;
- the range sets, the Unicode tables and the compiled programs, each on its own.

The search chain of the app pins the pattern that took 44 seconds before this engine: `re:(.{100,100}){0,100}\u0001` over the whole lexicon is answered through `view()` in about the time of a plain pattern, and the test fails when it takes more than 10 times as long, both measured in the same run (`src/lib/search-time.test.ts`).

## Benchmarks

`pnpm bench:regex` runs the engines over the real lexicon, 45,722 texts and 750,615 characters, warming each pattern up before it times it; `BENCH_ENGINES=pike,dfa` limits it. Medians in milliseconds, in one container on Node 22, which varies by up to 30% between runs. The earlier engine was measured the same way before it was deleted.

| Pattern | Earlier engine | Pike | DFA |
| --- | --- | --- | --- |
| `kitab` | 125 | 47 | 11 |
| `\w+ing` | 172 | 88 | 13 |
| `[a-z]{3,5}ing$` | 235 | 117 | 10 |
| `(.{20,20}){0,20}\u0001` | 6,150 | 746 | 16 |
| `(.{100,100}){0,100}\u0001` | 51,608 | 569 | 18 |
| `a.{18,18}c`, random a/b text, 200,000 characters | 95 | 114 | 73 |

The last row is the pattern for which a plain DFA needs a state for every pattern of the last 18 characters; the DFA gives up on it and finishes on the Pike executor, within 1.5 times its time in every run.

Not built, because no measurement asked for it: a one-pass or bit-state tier, a literal prefilter and a corpus index. Each would need a benchmark and a decision first.

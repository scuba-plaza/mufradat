# Changes from the earlier engine

The rewrite kept every behavior of the earlier `re:` engine except the changes below. While both engines existed, a test ran every row of the contract through the earlier one too, and any row that no declared change covered had to get the same answer from it; the earlier engine and that test were deleted when the app switched to this package.

## Breaking changes the plan lists

| Input | Today | Now |
| --- | --- | --- |
| `\n \t \r \f \v` | the letters n, t, r, f, v | control characters |
| `\b`, `\B` | the letters b, B | word boundaries |
| `\xHH`, `\uHHHH` | the letters x, u followed by digits | escapes for one code point |
| `\q` and other unknown letter or digit escapes | the plain letter | an error, `invalid_escape` |
| `a{2}` | the literal text `a{2}` | exactly two a |
| `a{,3}`, `a{2`, a lone `{` | `a{,3}` is zero to three a; the others are literal text | an error, `invalid_repeat` |
| `.` on a newline | matches | does not match |
| Error output | English sentences | stable codes, which the app maps to text |

## Further changes the comparison with the earlier engine found

These follow from the contract but are not in the plan's table.

- A backslash before a character that is not ASCII (`\é`, `\ك`, `\٥`, `\😀`) was that character; it is now an error, `invalid_escape`. The plan refuses any other letter or digit after a backslash, which covers `\é`, `\ك` and `\٥`; the rest follows Go, which refuses every non-ASCII character there.
- `[[:alpha:]]`, `[a[:digit:]]` and the like were a class of odd characters followed by a literal `]`; they are now refused as `unsupported`, as the plan lists POSIX classes under refused syntax.
- `\p{...}` and `\P{...}` were the letter p followed by literal text; they are now Unicode properties.
- Anything that was already an error stays an error, but its code can differ: `a++` and `a*?` were "a quantifier has nothing before it", and are now `unsupported`; `a**` is `nested_repeat`.

## Choices made where the plan is silent

The plan says to follow RE2 and Go on a detail it does not name, and to record the difference here. Each item below was a choice; any of them can be reversed by a change to the parser and one row of `test/contract.ts`.

### Errors

An error is `{ code, position?, detail? }`. `position` counts code points from 0 and points at where the offending construct starts; `too_long` has none. `detail` is the source text of the construct, given only for `unsupported`, `invalid_escape` and `invalid_property`; for a malformed `\x` or `\u` escape it runs up to and including the character that spoils it, or to the closing brace of `\x{...}`.

| Code | When |
| --- | --- |
| `too_long` | the pattern is over 200 characters (counted in UTF-16 units, as today) |
| `nesting_depth` | groups nest more than 1000 deep. The length limit already allows at most 100, so this applies only when the length limit is raised |
| `missing_paren` | a `(` has no `)`; the position is the `(` |
| `unexpected_paren` | a `)` has no `(` |
| `missing_bracket` | a `[` has no `]` |
| `empty_class` | `[]`, `[^]` and `[]a]`, as today: the first `]` closes the class |
| `reversed_char_range` | `[z-a]` |
| `invalid_char_range` | a range with a set as one end, `[a-\d]` |
| `trailing_backslash` | the pattern ends with `\` |
| `invalid_escape` | any letter, digit or non-ASCII character after a backslash that is not an escape the contract defines; `\1` to `\9`, octal `\0`, `\A`, `\z`, `\Z`, `\G`, `\Q`, `\E`, `\a`, `\e`, a malformed `\x` or `\u`, and `\b` inside a class all land here |
| `invalid_property` | `\p` or `\P` not followed by `{Name}`, including the `\pL` shorthand, and a name that is not in the tables (names are case sensitive) |
| `missing_repeat_argument` | `*`, `+`, `?` or a valid `{n}` form with nothing before it |
| `invalid_repeat` | a `{` that does not start `{n}`, `{n,}` or `{n,m}`, including at the start of a pattern |
| `reversed_repeat` | `{2,1}` |
| `repeat_too_large` | a count over 100; checked before `reversed_repeat` |
| `nested_repeat` | a repeat followed by `*`, or by a valid `{...}` |
| `too_big` | the compiled program would be over 20,000 instructions, the final match included; found from the tree by arithmetic before anything is built |
| `unsupported` | `(?` of any kind; `*?`, `+?`, `??`, `{n,m}?` (lazy) and `*+`, `++`, `?+`, `{n,m}+` (possessive); `[:name:]` inside a class |

### Syntax

- A backslash before any ASCII character that is not a letter or a digit is that character, including a space (Go).
- `\x{...}` takes any number of hex digits as long as the value is at most 10FFFF (Go). `\xHH` takes exactly two digits and `\uHHHH` exactly four. `\u{...}` is not supported.
- `\x{D800}` and `\uD800` are accepted and stand for the surrogate code point (Go).
- A repeat of an assertion (`^*`, `\b+`) or of an empty group (`()*`) is allowed (Go).
- `-` in a class is a literal at the start, at the end, and after a range or a set, as today.
- `[\b]` is refused (Go); it is not a backspace.
- `\p{Name}` takes a name of letters, digits and underscore that is in the tables, and nothing else: no `Script=Arabic`, no long names such as `Letter`, no `Any`, no `Cn`, no `LC`.
- `parse(pattern, limits?)` takes `maxLength`, `maxRepeat` and `maxDepth`, and `compileProgram(tree, limits)` takes `maxInstructions`.

### Unicode

- The tables are for Unicode 17.0. They are generated by `pnpm --filter @mufradat/regex generate:unicode` from the data of the V8 that runs the script, which asserts that its Unicode version is 17.0. The plan leaves the data source open; V8's data is the one the oracle compares with, so the tables agree with native `RegExp` by construction, and nothing has to be vendored or downloaded.
- A test checks that the tables reproduce byte for byte from the script and that every name is the complement of V8's `\P{name}`. Both are skipped, with a warning, on a Node whose V8 has another Unicode version, where they would compare the tables with data of another version; the tables are static and stay correct for the version they name, and the script regenerates them for another.
- The names are the 36 general categories of Go and 174 script names, in Go spelling (long names with underscores).
- Because the data is V8's, `\p{C}` includes the unassigned code points, as in JavaScript and the Unicode standard; Go's `C` leaves them out.
- `\d` is `\p{Nd}`, `\w` is `\p{L}`, `\p{N}` and `_`, and `\s` is the white space and line terminators of ECMAScript.

### Program

- There is no separate simplification pass. The compiler emits a bounded repeat directly in the nested form, `x{2,5}` as `xx(x(x(x)?)?)?`, in which every optional copy leaves by one shared exit; `size(tree)` counts the instructions of the result by arithmetic, and a test checks that it equals the length of the program on thousands of random patterns.
- A program has no loop for an unanchored search: the executors start a thread at every position. The program limit counts the instructions of the pattern and its final match, so a pattern close to the old limit can be accepted or refused where the old engine, which also counted its search loop, decided differently.

### API and executor

- `compile(pattern, limits?)` returns a matcher or an error object `{ code, position?, detail? }`; the two are told apart by `"code" in result`. `limits` takes any of `maxLength`, `maxRepeat`, `maxDepth` and `maxInstructions`; the rest keep their defaults, and so does one that is given as `undefined`. A pattern that nests deeper than the stack of the runtime can hold, which only a raised `maxDepth` allows, is refused as `nesting_depth` like any other.
- `matcher.test(text)` answers whether the pattern occurs anywhere in the text.
- A text is read by code point, so a surrogate pair is one character and a lone surrogate is one character too.
- The package exports `compile` and the types `Matcher`, `RegexError`, `ErrorCode` and `Limits`, and nothing else. The app maps each error code to a sentence in `src/lib/query/errors.ts`, keeping the exact text of every error the earlier engine had.
- `compile()` runs on the lazy DFA, which falls back to the Pike executor. Its cache is shared by every text the matcher is given, which is where most of its speed comes from.
- The cache budget is 2,000,000 stored entries, counting the positions held by each state and one transition entry per character class per state, about 8 MB. When a new state goes over it, every state is dropped but that one. The plan gives no number for the DFA's budget, and the prototype's 400,000 counted positions only.
- A matcher bails out to the Pike executor for the rest of its life once it has built more than 2,000 states and more than one state per 8 characters scanned, the prototype's rule. Characters are counted in UTF-16 units.
- The character classes are the intervals between the range boundaries of the sets in the program, and also between those of `\w` when the program has `\b` or `\B`.


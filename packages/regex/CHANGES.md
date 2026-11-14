# Changes from the current engine

The rewrite keeps every behavior of the current `re:` engine except the changes below. A test enforces that: every row of the contract that no declared change covers is run through the current engine too, and must get the same answer (`test/legacy.test.ts`).

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

## Further changes the comparison with the current engine found

These follow from the contract but are not in the plan's table.

- A backslash before a character that is not ASCII (`\é`, `\ك`, `\٥`, `\😀`) was that character; it is now an error, `invalid_escape`. The plan refuses any other letter or digit after a backslash, which covers `\é`, `\ك` and `\٥`; the rest follows Go, which refuses every non-ASCII character there.
- `[[:alpha:]]`, `[a[:digit:]]` and the like were a class of odd characters followed by a literal `]`; they are now refused as `unsupported`, as the plan lists POSIX classes under refused syntax.
- `\p{...}` and `\P{...}` were the letter p followed by literal text; they are now Unicode properties.
- Anything that was already an error stays an error, but its code can differ: `a++` and `a*?` were "a quantifier has nothing before it", and are now `unsupported`; `a**` is `nested_repeat`.

## Choices made where the plan is silent

The plan says to follow RE2 and Go on a detail it does not name, and to record the difference here. Each item below was a choice; any of them can be reversed by a change to the parser and one row of `test/contract.ts`.

### Errors

An error is `{ code, position?, detail? }`. `position` counts code points from 0 and points at where the offending construct starts; `too_long` has none. `detail` is the source text of the construct, given only for `unsupported`, `invalid_escape` and `invalid_property`.

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
| `invalid_property` | `\p` or `\P` not followed by `{Name}`, including the `\pL` shorthand |
| `missing_repeat_argument` | `*`, `+`, `?` or a valid `{n}` form with nothing before it |
| `invalid_repeat` | a `{` that does not start `{n}`, `{n,}` or `{n,m}`, including at the start of a pattern |
| `reversed_repeat` | `{2,1}` |
| `repeat_too_large` | a count over 100; checked before `reversed_repeat` |
| `nested_repeat` | a repeat followed by `*`, or by a valid `{...}` |
| `unsupported` | `(?` of any kind; `*?`, `+?`, `??`, `{n,m}?` (lazy) and `*+`, `++`, `?+`, `{n,m}+` (possessive); `[:name:]` inside a class |

### Syntax

- A backslash before any ASCII character that is not a letter or a digit is that character, including a space (Go).
- `\x{...}` takes any number of hex digits as long as the value is at most 10FFFF (Go). `\xHH` takes exactly two digits and `\uHHHH` exactly four. `\u{...}` is not supported.
- `\x{D800}` and `\uD800` are accepted and stand for the surrogate code point (Go).
- A repeat of an assertion (`^*`, `\b+`) or of an empty group (`()*`) is allowed (Go).
- `-` in a class is a literal at the start, at the end, and after a range or a set, as today.
- `[\b]` is refused (Go); it is not a backspace.
- `\p{Name}` takes a name of letters, digits and underscore. Whether the name exists is checked when the Unicode tables arrive; until then any such name parses.
- `parse(pattern, limits?)` takes `maxLength`, `maxRepeat` and `maxDepth`; the program size limit joins them with the compiler.

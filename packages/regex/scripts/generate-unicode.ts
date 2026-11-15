import { writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

export const UNICODE_VERSION = "17.0";

const CATEGORIES = [
  "C", "Cc", "Cf", "Co", "Cs", "L", "Ll", "Lm", "Lo", "Lt", "Lu", "M", "Mc", "Me", "Mn", "N", "Nd", "Nl", "No",
  "P", "Pc", "Pd", "Pe", "Pf", "Pi", "Po", "Ps", "S", "Sc", "Sk", "Sm", "So", "Z", "Zl", "Zp", "Zs",
];

const SCRIPTS = [
  "Adlam", "Ahom", "Anatolian_Hieroglyphs", "Arabic", "Armenian", "Avestan", "Balinese", "Bamum", "Bassa_Vah", "Batak", "Bengali",
  "Beria_Erfe", "Bhaiksuki", "Bopomofo", "Brahmi", "Braille", "Buginese", "Buhid", "Canadian_Aboriginal", "Carian",
  "Caucasian_Albanian", "Chakma", "Cham", "Cherokee", "Chorasmian", "Common", "Coptic", "Cuneiform", "Cypriot", "Cypro_Minoan",
  "Cyrillic", "Deseret", "Devanagari", "Dives_Akuru", "Dogra", "Duployan", "Egyptian_Hieroglyphs", "Elbasan", "Elymaic",
  "Ethiopic", "Garay", "Georgian", "Glagolitic", "Gothic", "Grantha", "Greek", "Gujarati", "Gunjala_Gondi", "Gurmukhi",
  "Gurung_Khema", "Han", "Hangul", "Hanifi_Rohingya", "Hanunoo", "Hatran", "Hebrew", "Hiragana", "Imperial_Aramaic", "Inherited",
  "Inscriptional_Pahlavi", "Inscriptional_Parthian", "Javanese", "Kaithi", "Kannada", "Katakana", "Kawi", "Kayah_Li",
  "Kharoshthi", "Khitan_Small_Script", "Khmer", "Khojki", "Khudawadi", "Kirat_Rai", "Lao", "Latin", "Lepcha", "Limbu", "Linear_A",
  "Linear_B", "Lisu", "Lycian", "Lydian", "Mahajani", "Makasar", "Malayalam", "Mandaic", "Manichaean", "Marchen", "Masaram_Gondi",
  "Medefaidrin", "Meetei_Mayek", "Mende_Kikakui", "Meroitic_Cursive", "Meroitic_Hieroglyphs", "Miao", "Modi", "Mongolian",
  "Mro", "Multani", "Myanmar", "Nabataean", "Nag_Mundari", "Nandinagari", "New_Tai_Lue", "Newa", "Nko", "Nushu",
  "Nyiakeng_Puachue_Hmong", "Ogham", "Ol_Chiki", "Ol_Onal", "Old_Hungarian", "Old_Italic", "Old_North_Arabian", "Old_Permic",
  "Old_Persian", "Old_Sogdian", "Old_South_Arabian", "Old_Turkic", "Old_Uyghur", "Oriya", "Osage", "Osmanya", "Pahawh_Hmong",
  "Palmyrene", "Pau_Cin_Hau", "Phags_Pa", "Phoenician", "Psalter_Pahlavi", "Rejang", "Runic", "Samaritan", "Saurashtra", "Sharada",
  "Shavian", "Siddham", "Sidetic", "SignWriting", "Sinhala", "Sogdian", "Sora_Sompeng", "Soyombo", "Sundanese", "Sunuwar",
  "Syloti_Nagri", "Syriac", "Tagalog", "Tagbanwa", "Tai_Le", "Tai_Tham", "Tai_Viet", "Tai_Yo", "Takri", "Tamil", "Tangsa", "Tangut",
  "Telugu", "Thaana", "Thai", "Tibetan", "Tifinagh", "Tirhuta", "Todhri", "Tolong_Siki", "Toto", "Tulu_Tigalari", "Ugaritic", "Vai",
  "Vithkuqi", "Wancho", "Warang_Citi", "Yezidi", "Yi", "Zanabazar_Square",
];

interface Segment {
  text: string;
  code(unit: number): number;
}

function codePoints(from: number, to: number): string {
  const parts: string[] = [];
  for (let start = from; start <= to; start += 8192) {
    const chunk: number[] = [];
    for (let code = start; code <= Math.min(start + 8191, to); code++) chunk.push(code);
    parts.push(String.fromCodePoint(...chunk));
  }
  return parts.join("");
}

const BELOW_SURROGATES: Segment = { text: codePoints(0, 0xd7ff), code: (unit) => unit };
const ABOVE_SURROGATES: Segment = {
  text: codePoints(0xe000, 0x10ffff),
  code: (unit) => (unit < 0x2000 ? 0xe000 + unit : 0x10000 + ((unit - 0x2000) >> 1)),
};

function append(ranges: number[], low: number, high: number): void {
  const last = ranges.length - 1;
  if (last >= 0 && low <= ranges[last] + 1) ranges[last] = Math.max(ranges[last], high);
  else ranges.push(low, high);
}

function appendRuns(ranges: number[], segment: Segment, run: RegExp): void {
  for (const match of segment.text.matchAll(run)) {
    append(ranges, segment.code(match.index), segment.code(match.index + match[0].length) - 1);
  }
}

export function matchRanges(expression: string): number[] {
  const ranges: number[] = [];
  const run = new RegExp(`(?:${expression})+`, "gu");
  appendRuns(ranges, BELOW_SURROGATES, run);
  const single = new RegExp(expression, "u");
  for (let code = 0xd800; code <= 0xdfff; code++) if (single.test(String.fromCharCode(code))) append(ranges, code, code);
  appendRuns(ranges, ABOVE_SURROGATES, run);
  return ranges;
}

export function generate(): string {
  if (process.versions.unicode !== UNICODE_VERSION) {
    throw new Error(`The tables are for Unicode ${UNICODE_VERSION} and this V8 has Unicode ${process.versions.unicode}`);
  }
  const entries = [...CATEGORIES, ...SCRIPTS].map((name) => {
    const property = CATEGORIES.includes(name) ? name : `Script=${name}`;
    return `  ${name}: ${JSON.stringify(matchRanges(`\\p{${property}}`))},`;
  });
  return [
    `export const UNICODE_VERSION = "${UNICODE_VERSION}";`,
    "",
    "export const PROPERTIES: Record<string, readonly number[]> = {",
    ...entries,
    "};",
    "",
    `export const SPACE: readonly number[] = ${JSON.stringify(matchRanges("\\s"))};`,
    "",
  ].join("\n");
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  writeFileSync(new URL("../src/unicode/tables.ts", import.meta.url), generate());
}

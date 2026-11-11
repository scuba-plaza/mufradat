import Database from "better-sqlite3";
import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { audioFileName, slotLabel, type ClipPlace } from "./audio";
import { plainExample, splitExample } from "./validate";
import { POS_LABELS } from "./types";
import type { Note } from "./types";
import { zipStored } from "./zip";

const MODEL_NAME = "Arabic MSA Word";
const FONT_FILE = "ScheherazadeNew-Regular.ttf";
const FONT_MEDIA_NAME = "_" + FONT_FILE;
const PRODUCTION_TOP = 1000;
const PRODUCTION_DELAY = 20;
const UNRANKED_BASE = 5000;
const TEMPLATES_REVISED = Date.UTC(2026, 11, 11, 12, 0, 0);
const DESCRIPTION =
  "The most common Modern Standard Arabic words in frequency order, with fully vowelled examples.";

const FIELDS = [
  "Arabic", "English", "Hint", "HintArabic", "Forms", "Example", "ExampleEnglish", "WordAudio", "FormsAudio",
  "ExampleAudio", "Pos", "Details", "Production", "Check", "Rank", "NoteID",
] as const;
type Field = (typeof FIELDS)[number];

interface ExportOptions {
  deckName: string;
  deckId: number;
  modelId: number;
  /** Where the template files and the font live; defaults to the app's own data. */
  resources?: string;
}

const SCHEMA = `
CREATE TABLE col (id integer primary key, crt integer not null, mod integer not null, scm integer not null, ver integer not null, dty integer not null, usn integer not null, ls integer not null, conf text not null, models text not null, decks text not null, dconf text not null, tags text not null);
CREATE TABLE notes (id integer primary key, guid text not null, mid integer not null, mod integer not null, usn integer not null, tags text not null, flds text not null, sfld integer not null, csum integer not null, flags integer not null, data text not null);
CREATE TABLE cards (id integer primary key, nid integer not null, did integer not null, ord integer not null, mod integer not null, usn integer not null, type integer not null, queue integer not null, due integer not null, ivl integer not null, factor integer not null, reps integer not null, lapses integer not null, left integer not null, odue integer not null, odid integer not null, flags integer not null, data text not null);
CREATE TABLE revlog (id integer primary key, cid integer not null, usn integer not null, ease integer not null, ivl integer not null, lastIvl integer not null, factor integer not null, time integer not null, type integer not null);
CREATE TABLE graves (usn integer not null, oid integer not null, type integer not null);
CREATE INDEX ix_notes_usn on notes (usn);
CREATE INDEX ix_cards_usn on cards (usn);
CREATE INDEX ix_revlog_usn on revlog (usn);
CREATE INDEX ix_cards_nid on cards (nid);
CREATE INDEX ix_cards_sched on cards (did, queue, due);
CREATE INDEX ix_revlog_cid on revlog (cid);
CREATE INDEX ix_notes_csum on notes (csum);
`;

const escapeHtml = (s: string) =>
  s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#39;");

const ARABIC_RUN = /[\u0600-\u06FF\u0750-\u077F]+(?:[\s،]+[\u0600-\u06FF\u0750-\u077F]+)*/g;
const arabicSpan = (escaped: string) => `<span class="ar" lang="ar" dir="rtl">${escaped}</span>`;
const markArabic = (s: string) => escapeHtml(s).replace(ARABIC_RUN, arabicSpan);

function formsHtml(n: Note): string {
  return n.forms
    .map((f) => `<span class="form"><span class="lbl">${escapeHtml(f.label)}</span>${arabicSpan(escapeHtml(f.arabic))}</span>`)
    .join(" ");
}

function exampleHtml(n: Note): string {
  const p = splitExample(n.example);
  if (!p) return escapeHtml(plainExample(n.example));
  return escapeHtml(p.before) + "<b>" + escapeHtml(p.target) + "</b>" + escapeHtml(p.after);
}

function detailsHtml(n: Note): string {
  const parts: string[] = [];
  if (n.gender === "m") parts.push("masc.");
  else if (n.gender === "f") parts.push("fem.");
  else if (n.gender === "m+f") parts.push("masc./fem.");
  if (n.verbForm) parts.push("form " + escapeHtml(n.verbForm));
  if (n.root) parts.push("root " + markArabic(n.root));
  return parts.join(" · ");
}

function rankBucket(rank: number): string {
  if (rank <= 0) return "rank::unranked";
  const lo = Math.floor((rank - 1) / 500) * 500 + 1;
  const pad = (v: number) => String(v).padStart(4, "0");
  return `rank::${pad(lo)}-${pad(lo + 499)}`;
}

const soundTag = (key: string) => `[sound:${audioFileName(key)}]`;

function stableId(...parts: string[]): number {
  const h = createHash("sha256").update(parts.join("\x1f")).digest();
  return 1_700_000_000_000 + Number(h.readBigUInt64BE(0) % 100_000_000_000n);
}

const stripHtml = (s: string) => s.replace(/<[^>]*>/g, "").trim();
const checksum = (s: string) => parseInt(createHash("sha1").update(stripHtml(s)).digest("hex").slice(0, 8), 16);

function studyOrder(a: Note, b: Note): number {
  if (a.rank > 0 && b.rank > 0 && a.rank !== b.rank) return a.rank - b.rank;
  if ((a.rank > 0) !== (b.rank > 0)) return a.rank > 0 ? -1 : 1;
  return a.createdAt.localeCompare(b.createdAt) || a.id.localeCompare(b.id);
}

function modelJson(templates: Record<string, string>, opts: ExportOptions, mod: number) {
  const panels = templates["panels.html"];
  const tmpl = (name: string, ord: number, front: string, back: string) => ({
    name, ord, qfmt: front, afmt: back + panels, did: null, bqfmt: "", bafmt: "",
  });
  return {
    id: String(opts.modelId), name: MODEL_NAME, type: 0, mod, usn: -1, sortf: 0, did: opts.deckId,
    tmpls: [
      tmpl("Recognition", 0, templates["recognition_front.html"], templates["recognition_back.html"]),
      tmpl("Production", 1, templates["production_front.html"], templates["production_back.html"]),
    ],
    flds: FIELDS.map((name, ord) => ({
      name, ord, sticky: false, rtl: name === "Arabic" || name === "Example", font: "Arial", size: 20, media: [],
    })),
    css: templates["card.css"],
    latexPre: "\\documentclass[12pt]{article}\n\\special{papersize=3in,5in}\n\\usepackage[utf8]{inputenc}\n\\usepackage{amssymb,amsmath}\n\\pagestyle{empty}\n\\setlength{\\parindent}{0in}\n\\begin{document}\n",
    latexPost: "\\end{document}", latexsvg: false,
    req: [[0, "all", [0]], [1, "all", [FIELDS.indexOf("Production"), FIELDS.indexOf("English")]]],
    tags: [], vers: [],
  };
}

function deckJson(id: number, name: string, desc: string, mod: number) {
  return {
    id, name, mod, usn: -1, desc, dyn: 0, conf: 1, collapsed: false, browserCollapsed: true, extendNew: 0, extendRev: 0,
    lrnToday: [0, 0], revToday: [0, 0], newToday: [0, 0], timeToday: [0, 0],
  };
}

const DECK_CONFIG = {
  "1": {
    id: 1, name: "Default", mod: 0, usn: 0, maxTaken: 60, autoplay: true, timer: 0, replayq: true,
    new: { bury: true, delays: [1, 10], initialFactor: 2500, ints: [1, 4, 7], order: 1, perDay: 20, separate: true },
    rev: { bury: true, ease4: 1.3, fuzz: 0.05, ivlFct: 1, maxIvl: 36500, minSpace: 1, perDay: 200 },
    lapse: { delays: [10], leechAction: 0, leechFails: 8, minInt: 1, mult: 0 },
  },
};

const TEMPLATE_FILES = [
  "card.css", "panels.html", "production_back.html", "production_front.html",
  "recognition_back.html", "recognition_front.html",
];

/**
 * Builds an .apkg from approved notes. `readAudio` returns the bytes of a clip by key.
 * The same notes in any order give the same bytes.
 */
export function buildApkg(notes: Note[], readAudio: (key: string, place: ClipPlace) => Uint8Array, opts: ExportOptions, everyCard: Note[] = notes): Buffer<ArrayBuffer> {
  if (notes.length === 0) throw new Error("There are no approved cards to export.");
  const resources = opts.resources ?? path.join(process.cwd(), "data");
  const templates = Object.fromEntries(
    TEMPLATE_FILES.map((f) => [f, fs.readFileSync(path.join(resources, "template", f), "utf8")]),
  );
  const font = fs.readFileSync(path.join(process.cwd(), "public", FONT_FILE));

  const ordered = [...notes].sort(studyOrder);
  const unranked = new Map(
    [...new Map([...everyCard, ...notes].map((n) => [n.id, n])).values()]
      .filter((n) => n.rank <= 0)
      .sort(studyOrder)
      .map((n, i) => [n.id, i + 1]),
  );
  const media = new Map<string, Uint8Array>([[FONT_MEDIA_NAME, font]]);
  const rows = ordered.map((n) => {
    const v = Object.fromEntries(FIELDS.map((f) => [f, ""])) as Record<Field, string>;
    v.Arabic = escapeHtml(n.lemma);
    v.English = escapeHtml(n.english);
    v.Hint = markArabic(n.hint);
    v.HintArabic = markArabic(n.hintAr);
    v.Forms = formsHtml(n);
    v.Example = n.example.trim() ? exampleHtml(n) : "";
    v.ExampleEnglish = escapeHtml(n.exampleEn);
    v.Pos = POS_LABELS[n.pos];
    v.Details = detailsHtml(n);
    v.Rank = String(n.rank);
    v.NoteID = n.id;

    const use = (slot: string) => {
      const key = n.audio[slot]?.key;
      if (!key) return "";
      media.set(audioFileName(key), readAudio(key, { lemma: n.lemma, slotName: slotLabel(n.forms, slot) }));
      return soundTag(key);
    };
    v.WordAudio = use("word");
    v.FormsAudio = n.forms.map((f) => use(f.id)).join("");
    v.ExampleAudio = n.example.trim() ? use("example") : "";

    const due = n.rank > 0 ? n.rank : UNRANKED_BASE + unranked.get(n.id)!;
    const cards = [{ ord: 0, due }];
    if (n.rank > 0 && n.rank <= PRODUCTION_TOP) {
      v.Production = "y";
      cards.push({ ord: 1, due: due + PRODUCTION_DELAY });
    }
    return { n, v, cards, mod: Math.floor(Date.parse(n.updatedAt) / 1000), tags: [...new Set(["pos::" + n.pos, rankBucket(n.rank), ...n.tags])] };
  });

  const created = opts.deckId;
  const latest = Math.max(created, TEMPLATES_REVISED, ...rows.map((r) => r.mod * 1000));
  const sec = Math.floor(latest / 1000);
  const createdDate = new Date(created);
  const crt = Date.UTC(createdDate.getUTCFullYear(), createdDate.getUTCMonth(), createdDate.getUTCDate(), 4) / 1000;

  const db = new Database(":memory:");
  db.exec(SCHEMA);
  db.prepare("INSERT INTO col VALUES (1, ?, ?, ?, 11, 0, 0, 0, ?, ?, ?, ?, '{}')").run(
    crt, latest, latest,
    JSON.stringify({
      activeDecks: [1], curDeck: 1, newSpread: 0, collapseTime: 1200, timeLim: 0, estTimes: true, dueCounts: true,
      curModel: String(opts.modelId), nextPos: 1, sortType: "noteFld", sortBackwards: false, addToCur: true,
    }),
    JSON.stringify({ [opts.modelId]: modelJson(templates, opts, sec) }),
    JSON.stringify({ "1": deckJson(1, "Default", "", sec), [opts.deckId]: deckJson(opts.deckId, opts.deckName, DESCRIPTION, sec) }),
    JSON.stringify(DECK_CONFIG),
  );
  const insertNote = db.prepare("INSERT INTO notes VALUES (?, ?, ?, ?, -1, ?, ?, ?, ?, 0, '')");
  const insertCard = db.prepare("INSERT INTO cards VALUES (?, ?, ?, ?, ?, -1, 0, 0, ?, 0, 0, 0, 0, 0, 0, 0, 0, '')");
  const seen = new Map<number, string>();
  db.transaction(() => {
    for (const r of rows) {
      const nid = stableId("note", r.n.id);
      if (seen.has(nid)) throw new Error(`Cards ${seen.get(nid)} and ${r.n.id} collide on id ${nid}.`);
      seen.set(nid, r.n.id);
      insertNote.run(nid, r.n.id, opts.modelId, r.mod, ` ${r.tags.join(" ")} `, FIELDS.map((f) => r.v[f]).join("\x1f"),
        stripHtml(r.v.Arabic), checksum(r.v.Arabic));
      for (const c of r.cards) insertCard.run(stableId("card", r.n.id, String(c.ord)), nid, opts.deckId, c.ord, r.mod, c.due);
    }
  })();
  const collection = db.serialize();
  db.close();

  const names = [...media.keys()].sort();
  return zipStored([
    { name: "collection.anki2", data: collection },
    ...names.map((name, i) => ({ name: String(i), data: media.get(name)! })),
    { name: "media", data: Buffer.from(JSON.stringify(Object.fromEntries(names.map((name, i) => [String(i), name])))) },
  ]);
}

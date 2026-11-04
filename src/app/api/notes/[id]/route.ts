import { NextResponse } from "next/server";
import { context, fail, handle, readJson } from "@/lib/server";
import { POS_VALUES, type NoteEdit, type POS } from "@/lib/types";

export const dynamic = "force-dynamic";
type Ctx = { params: Promise<{ id: string }> };

const TEXT_FIELDS = ["lemma", "english", "gender", "root", "verbForm", "hint", "example", "exampleEn"] as const;

export const PUT = (req: Request, { params }: Ctx) =>
  handle(async () => {
    const body = await readJson(req);
    const edit: NoteEdit = {};
    for (const key of TEXT_FIELDS) if (typeof body[key] === "string") edit[key] = body[key];
    if (typeof body.pos === "string") {
      if (!POS_VALUES.includes(body.pos as POS)) return fail("Unknown part of speech.");
      edit.pos = body.pos as POS;
    }
    if (Array.isArray(body.forms)) {
      edit.forms = body.forms.map((f: { id?: unknown; label?: unknown; arabic?: unknown }) => ({
        id: typeof f.id === "string" ? f.id : undefined,
        label: String(f.label ?? ""),
        arabic: String(f.arabic ?? ""),
      }));
    }
    return NextResponse.json({ note: context().store.updateNote((await params).id, edit) });
  });

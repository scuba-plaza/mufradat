# Possible future improvements

## Tags changed after the first import do not reach Anki on re-import

Setting tags does not change a card's edit time, so that tags never take an approval back or count as an edit of the card. Anki's re-import compares the note's modification time, which the export takes from that edit time. Tags changed after a card was first imported into Anki are therefore ignored when the deck is imported again, unless the note is deleted in Anki first.

A fix: keep a separate `tags_updated_at` column on `notes` (a schema migration to version 4, with the restore and version-guard tests extended), set it in `setTags`, and export `max(updated_at, tags_updated_at)` as the note's modification time.

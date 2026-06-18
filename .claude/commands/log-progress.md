---
description: Append a curated entry to the gowallet prompt/decision timeline via the doc-updater subagent.
argument-hint: <optional one-line note on what just happened>
---

Update the gowallet paper trail. **Note:** $ARGUMENTS

Dispatch the **`doc-updater`** subagent to:
1. Append a new entry to `docs/PROMPT_LOG.md` (above the `<!-- New entries go below this line -->`
   marker), incrementing the Entry number, in the established format
   (🧑 Asked · 🤖 Did · ✅/✏️ · 💡 Why · 📚 sources). Summarize the recent exchange/work — curated,
   not verbatim. If `$ARGUMENTS` is empty, summarize what we did since the last entry.
2. If a real decision changed, sync the `SOLUTION.md` decisions table / AI-workflow summary.
3. If invariants changed state, update the `docs/ACCEPTANCE.md` status flags.

When it returns, show me a tiny frame: which files changed + the new entry number. Don't commit
unless I ask.

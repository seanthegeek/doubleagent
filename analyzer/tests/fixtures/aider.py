"""Synthetic Aider state for the analyzer tests."""

from __future__ import annotations

from pathlib import Path

# Written the way aider/io.py writes them: user and blockquote lines end in
# two spaces (markdown line breaks), assistant output is framed by blanks.
AIDER_CHAT = "".join(
    [
        "\n# aider chat started at 2026-10-02 12:00:00\n\n",
        # the startup announcement (base_coder.py get_announcements)
        "> Aider v0.86.1  \n",
        "> Main model: anthropic/claude-sonnet-4-5 with diff edit format, prompt cache  \n",
        "> Weak model: anthropic/claude-haiku-4-5  \n",
        "> Git repo: .git with 3 files  \n",
        "\n#### rename foo to bar  \n",
        "\nHere is the change:\n\na.py\n<<<<<<< SEARCH\nfoo\n=======\nbar\n>>>>>>> REPLACE\n\n",
        "> Applied edit to a.py  \n",
        "> Commit abc1234 refactor: rename foo to bar  \n",
        # confirm_ask: the subject, then the question with the answer (io.py)
        "> pytest -q  \n",
        "> Run shell command? (Y)es/(N)o/(D)on't ask again [Yes]: n  \n",
        "\n# aider chat started at 2026-10-02 13:00:00\n\n",
        "> Aider v0.86.1  \n",
        "> Model: gpt-4o with diff edit format  \n",
        "> Git repo: .git with 3 files  \n",
        "\n#### /add b.py  \n",
        "> Added b.py to the chat  \n",
        "\n#### <blank>  \n",
        # a /model switch re-announces; architect mode names an editor model
        "\n#### /model claude-opus-4-5  \n",
        "> Aider v0.86.1  \n",
        "> Main model: claude-opus-4-5 with architect edit format  \n",
        "> Editor model: claude-sonnet-4-5 with editor-diff edit format  \n",
        "> Weak model: claude-haiku-4-5  \n",
        "> Git repo: .git with 3 files  \n",
        "> https://example.com/doc  \n",
        "> Add URL to the chat? (Y)es/(N)o/(A)ll/(S)kip all/(D)on't ask again [Yes]: d  \n",
    ]
)

# The last entry has no chat-history twin (a prompt whose chat line is gone),
# so it survives deduplication; the other two pair with the chat rows.
AIDER_INPUT = (
    "\n# 2026-10-02 12:00:05.123456\n+rename foo to bar\n\n# 2026-10-02 13:00:02.000000\n+/add b.py\n"
    "\n# 2026-10-02 13:05:00.000000\n+delete the migrations folder\n"
)

AIDER_LLM = (
    "TO LLM 2026-10-02T12:00:05\n-------\nSYSTEM Act as an expert\nSYSTEM software developer\n-------\n"
    "USER rename foo to bar\nLLM RESPONSE 2026-10-02T12:00:09\nASSISTANT Here is the change:\nASSISTANT \n"
    "ASSISTANT a.py\n"
)


def build_aider(base: Path) -> None:
    """Aider's history files in `base`, a home or a repository root."""
    base.mkdir(parents=True, exist_ok=True)
    (base / ".aider.chat.history.md").write_text(AIDER_CHAT, encoding="utf-8")
    (base / ".aider.input.history").write_text(AIDER_INPUT, encoding="utf-8")
    (base / ".aider.llm.history").write_text(AIDER_LLM, encoding="utf-8")
    (base / ".aider.conf.yml").write_text("model: gpt-4o\n", encoding="utf-8")

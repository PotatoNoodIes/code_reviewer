#!/usr/bin/env python3
"""Embed the fonts/ woff2 files into index.template.html as @font-face data
URIs, producing the self-contained index.html.

Run this after editing index.template.html:

    python3 site/build.py
"""
import base64
import pathlib

SITE_DIR = pathlib.Path(__file__).parent
TEMPLATE = SITE_DIR / "index.template.html"
OUTPUT = SITE_DIR / "index.html"

FONT_MAP = {
    "__FONT_MONO_400__": "fonts/jetbrains-mono-latin-400-normal.woff2",
    "__FONT_MONO_700__": "fonts/jetbrains-mono-latin-700-normal.woff2",
    "__FONT_SERIF_400__": "fonts/source-serif-4-latin-400-normal.woff2",
    "__FONT_SERIF_400I__": "fonts/source-serif-4-latin-400-italic.woff2",
    "__FONT_SERIF_600__": "fonts/source-serif-4-latin-600-normal.woff2",
}


def main():
    html = TEMPLATE.read_text(encoding="utf-8")

    for placeholder, rel_path in FONT_MAP.items():
        font_path = SITE_DIR / rel_path
        b64 = base64.b64encode(font_path.read_bytes()).decode("ascii")
        count = html.count(placeholder)
        if count != 1:
            raise SystemExit(f"expected exactly 1 occurrence of {placeholder}, found {count}")
        html = html.replace(placeholder, b64)

    OUTPUT.write_text(html, encoding="utf-8")
    print(f"wrote {OUTPUT} ({OUTPUT.stat().st_size} bytes)")


if __name__ == "__main__":
    main()

import json
import re

import pdfplumber
import requests

OLLAMA_URL = "http://localhost:11434/api/generate"
MODEL = "mistral"

KNOWN_TAGS = [
    "include",
    "remove",
    "comment",
    "vars",
    "assign",
    "eval",
    "choose",
    "when",
    "otherwise",
    "try",
    "attempt",
    "except",
    "inline",
    "function",
    "text",
]


def extract_pdf_text(path):
    text = ""
    with pdfplumber.open(path) as pdf:
        for page in pdf.pages:
            extracted = page.extract_text()
            if extracted:
                text += extracted + "\n"
    return text


def split_by_tags(text):
    chunks = {}
    for i, tag in enumerate(KNOWN_TAGS):
        start = re.search(rf"\n.*?esi:{tag}.*?\n", text, re.IGNORECASE)
        if not start:
            print(f"Warning: could not find section for {tag}")
            continue
        next_tag = KNOWN_TAGS[i + 1] if i + 1 < len(KNOWN_TAGS) else None
        if next_tag:
            end = re.search(
                rf"\n.*?esi:{next_tag}.*?\n", text[start.start() :], re.IGNORECASE
            )
            chunk = (
                text[start.start() : start.start() + end.start()]
                if end
                else text[start.start() :]
            )
        else:
            chunk = text[start.start() :]
        chunks[tag] = chunk
    return chunks


def build_prompt(tag, content):
    return f"""You are extracting ESI tag metadata from Akamai documentation.
Return ONLY a JSON object. No explanation, no markdown, no preamble.

Schema:
{{
  "tag": "esi:{tag}",
  "summary": "one sentence description",
  "requiredAttrs": ["attr1"],
  "allowedAttrs": ["attr1", "attr2"],
  "attrDocs": {{
    "attr1": "short description"
  }}
}}

Rules:
- Do NOT hallucinate attributes not present in the text
- requiredAttrs must be a subset of allowedAttrs
- Keep summary under 15 words
- Keep attr descriptions under 10 words
- If no attributes, use empty arrays and object

Documentation for esi:{tag}:
{content[:3000]}"""


def call_llm_with_retry(prompt, retries=3):
    for attempt in range(retries):
        try:
            resp = requests.post(
                OLLAMA_URL,
                json={"model": MODEL, "prompt": prompt, "stream": False},
                timeout=60,
            )
            out = resp.json()["response"]
            cleaned = re.sub(r"```json|```", "", out).strip()
            return json.loads(cleaned)
        except (json.JSONDecodeError, KeyError) as e:
            print(f"  Attempt {attempt + 1}/{retries} failed: {e}")
    return None


def main():
    print("Extracting PDF text...")
    text = extract_pdf_text("esi.pdf")

    print("Splitting by tags...")
    chunks = split_by_tags(text)
    print(f"Found sections for: {list(chunks.keys())}")

    results = {}
    for tag, content in chunks.items():
        print(f"Processing esi:{tag}...")
        prompt = build_prompt(tag, content)
        data = call_llm_with_retry(prompt)
        if data:
            results[f"esi:{tag}"] = data
            print(f"  ✓ done")
        else:
            print(f"  ✗ failed after retries")

    with open("tags.json", "w") as f:
        json.dump(results, f, indent=2)

    print(f"\nDone. Extracted {len(results)}/{len(chunks)} tags → tags.json")


if __name__ == "__main__":
    main()

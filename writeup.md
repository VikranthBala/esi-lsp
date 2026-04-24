so this is my first time writing an lsp, and this is what I have understood.

Basically, lsp is a langauge server, which knows hows, and whats of the langauage, and our editors, are basically like clients, to the server. This communication is done on json-rpc. That's the whole game. Everything in an LSP server is just: receive a JSON message → do something smart → send JSON back.

ex message that vs code sends, and lsp resonds with is:

request:
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "textDocument/didOpen",
  "params": {
    "textDocument": {
      "uri": "file:///home/you/page.html",
      "text": "<esi:include src=\"/header.html\" />"
    }
  }
}

response:
{
  "jsonrpc": "2.0",
  "method": "textDocument/publishDiagnostics",
  "params": {
    "uri": "file:///home/you/page.html",
    "diagnostics": []
  }
}


lifecycle of the lsp session
Client                          Server
  |                               |
  |--- initialize  -------------->|   "here are my capabilities"
  |<-- initializeResult ----------|   "here are mine"
  |--- initialized (notify) ----->|   "ok let's go"
  |                               |
  |--- textDocument/didOpen ----->|   "user opened a file"
  |--- textDocument/didChange --->|   "user typed something"
  |<-- publishDiagnostics --------|   "here are the errors I found"
  |                               |
  |--- textDocument/completion -->|   "user pressed Ctrl+Space"
  |<-- CompletionList ------------|   "here are the suggestions"
  |                               |
  |--- shutdown  ---------------->|   "user closed the editor"
  |--- exit (notify) ------------>|   "goodbye"


We use sourcegraph/jsonrpc2 purely for message framing and routing. The ESI intelligence is entirely our own code.


Step1: Lets setup a simple server
mention about stderr/stdout -> for log

------------------------------------------------------

Question answer:

Before I walk you through the server, I want you to think about one thing and tell me your instinct:
When VS Code sends us a textDocument/didChange notification (user typed something), should we:
(A) Parse the document immediately, synchronously, before reading the next message
(B) Parse it in a goroutine, so we go back to reading messages immediately
Think about what happens in each case if the user is typing fast — a keystroke every 100ms — and parsing takes 50ms. What breaks?
Tell me your answer and we'll write server.go based on your reasoning.

Answer


Your understanding is exactly right, and you've independently identified a real problem that LSP servers in production actually solve. Let me sharpen it a bit:

You're right on both counts
Problem with pure sync (A):
If parsing takes 50ms and the user types every 100ms, you're spending half your time blocked on parsing instead of reading new messages. More critically — didChange is a notification, meaning the client doesn't wait for a response. But if your message loop is blocked parsing, you're not reading the next notification. Messages start queuing up in the stdin buffer. The editor feels laggy.
Problem with naive goroutines (B):
Exactly what you said — stale results. User types "hello", goroutine 1 starts. User types "hello w", goroutine 2 starts. Goroutine 2 finishes first (shorter parse), pushes clean diagnostics. Goroutine 1 finishes, pushes diagnostics for "hello" — now the editor shows errors for a state that no longer exists. This is a classic race condition.

The actual solution — debouncing with cancellation
The pattern used by real LSP servers (rust-analyzer, gopls, etc.) is:
Each time didChange arrives:
  1. Cancel the previous parse goroutine if it's still running
  2. Start a new goroutine for the latest content
In Go, you do this with a context.CancelFunc. Visually:
keystroke 1  →  start parse (ctx1)
keystroke 2  →  cancel ctx1, start parse (ctx2)
keystroke 3  →  cancel ctx2, start parse (ctx3)
user stops   →  ctx3 runs to completion → diagnostics published ✓
Only the last goroutine ever finishes. All earlier ones get cancelled mid-flight. Zero stale results, zero wasted CPU on old content.
Some servers add a small debounce delay (e.g. 100ms) — don't even start parsing until the user has paused for 100ms. This avoids spawning goroutines at all for fast typing. We'll add that later; cancellation is the important foundation.
For your blog: This is a great insight to write up. The headline could be "Why your LSP needs debouncing" — it's a pattern most developers don't encounter until they build something real-time like this.

Why sync.RWMutex and not a plain sync.Mutex?
Think about what reads and writes look like here. Multiple LSP requests can arrive concurrently — a hover and a completion request could come in at the same time, both needing to read docs. But a didChange writes to docs.
RWMutex lets multiple readers run simultaneously, but a writer gets exclusive access. A plain Mutex would serialise even two concurrent reads, which is unnecessary. You call mu.RLock() for reads, mu.Lock() for writes.

For your blog: This is worth noting — LSP's wire format is nested JSON, not flat. A common beginner mistake is writing flat Go structs and getting silent empty-string bugs that are hard to track down because the unmarshaller doesn't error, it just gives you zero values.


Yes — docs is a map of every file the editor currently has open, keyed by URI. When VS Code opens a file, didOpen fires and you add it. When the user closes the tab, didClose fires and you remove it. didChange fires on every keystroke for any open file.
So at any point, s.docs represents the current in-memory state of every open tab. This is important — you never read from disk. The editor is the source of truth, and it tells you the full content on every change.
"Missing the lock in handleChange?"
Exactly right. You caught it yourself. handleChange reads and writes s.docs but has no lock. Same pattern as handleOpen — wrap the map write in mu.Lock().

The bigger issue — parser.ParseDocument returning an error
This is a design decision worth thinking about. Should your parser return an error when it finds bad ESI?
The answer is no — and this is important for your blog.
A compiler can refuse to compile invalid code. But a language server must never refuse to process a file. The user is mid-edit. The file is always in an invalid state while they're typing. If your parser returns an error and you bail out, you'd clear all diagnostics the moment the user types an incomplete tag — exactly when they need help most.
So ParseDocument should return *Document always, and the document itself carries any errors it found as data:
// Parser never fails — it always returns a document.
// Errors found during parsing are stored inside the document.
func ParseDocument(uri, text string) *parser.Document
This means your handlers never need to check a parse error — they always get a document back, possibly with doc.Errors populated.

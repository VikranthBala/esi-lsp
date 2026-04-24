package main

import (
	"context"
	"log"
	"os"

	"github.com/vikranthBala/esi-lsp/internal/server"
)

func main() {
	//Critical. Our stdout is reserved for JSON-RPC messages to the editor.
	// If any log line goes to stdout it corrupts the protocol stream and
	// the editor will crash or disconnect.
	// Stderr is safe — VS Code captures it separately and shows it in the Output panel.
	log.SetOutput(os.Stderr) // set the output to stderr

	//Adds timestamps and file:line to every log line.
	// When debugging an LSP you're reading a stream of JSON in a log file — this context is invaluable.
	log.SetFlags(log.Ltime | log.Lshortfile) // this is for log formatting

	log.Println("Starting akamai-esi-lsp server...")

	// We pass a context through the whole server so we can cancel it cleanly on shutdown.
	// Right now it's Background (never cancels), but later we can hook OS signals into it.
	srv := server.New()
	if err := srv.Run(context.Background()); err != nil {
		log.Fatalf("server err: %v", err)
	}
}

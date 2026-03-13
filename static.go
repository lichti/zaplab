package main

import (
	"embed"
	"io/fs"
	"os"
)

//go:embed pb_public
var embeddedFiles embed.FS

// getStaticFS returns the filesystem used to serve the dashboard UI.
// When ZAPLAB_DEV=1 the files are read live from ./pb_public (no rebuild needed).
// In all other cases the files embedded at compile time are used.
func getStaticFS() fs.FS {
	if os.Getenv("ZAPLAB_DEV") == "1" {
		return os.DirFS("./pb_public")
	}
	sub, err := fs.Sub(embeddedFiles, "pb_public")
	if err != nil {
		// Should never happen — pb_public is always embedded.
		panic("embed: failed to sub pb_public: " + err.Error())
	}
	return sub
}

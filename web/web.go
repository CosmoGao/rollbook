package web

import "embed"

//go:embed index.html app.js i18n.js style.css topology.js dagre.min.js dagre.LICENSE marked.umd.js purify.min.js marked.LICENSE dompurify.LICENSE manifest.webmanifest icon.svg icon-192.png icon-512.png
var Files embed.FS

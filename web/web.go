package web

import "embed"

//go:embed index.html app.js i18n.js style.css topology.js dagre.min.js dagre.LICENSE
var Files embed.FS

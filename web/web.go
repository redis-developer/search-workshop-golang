// Package web embeds the workshop's search UI, adapted from the
// "Building Hybrid Search Apps with Redis" workshop
// (https://github.com/redis-developer/building-hybrid-search-apps-with-redis).
//
// The UI is served by cmd/searchd from the same port as the API, so the
// fetch calls in scripts/apis.js use relative paths — no CORS, no second
// process.
package web

import "embed"

// FS holds the static UI assets.
//
//go:embed index.html style.css scripts images
var FS embed.FS

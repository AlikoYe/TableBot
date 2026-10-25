// Package static embeds files served by the API, such as venue floor plans.
package static

import "embed"

// Plans holds the floor plan SVGs; all use viewBox 0 0 1000 700.
//
//go:embed plans/*.svg
var Plans embed.FS

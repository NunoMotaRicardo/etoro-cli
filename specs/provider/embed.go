// Package provider embeds the committed eToro provider snapshot files.
package provider

import _ "embed"

// RoutesSnapshot is routes-snapshot.json.
//
//go:embed routes-snapshot.json
var RoutesSnapshot []byte

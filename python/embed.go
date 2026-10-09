package python

import "embed"

//go:embed mufradat_sidecar/*.py requirements.lock
var Files embed.FS

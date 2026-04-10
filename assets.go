package main

import "embed"

// BuildAssets contains the Dockerfile and scripts needed by `aoa build`.
// Embedding them in the binary means `aoa build` works correctly when
// installed via Homebrew or any other method — no source tree required.
//
//go:embed images scripts
var BuildAssets embed.FS

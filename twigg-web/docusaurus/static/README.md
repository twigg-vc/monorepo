This file exists because Go requires ./build to exist at compile time.

The Go server embeds the ./build directory using go:embed, which has two
requirements:
1. The embedded path must exist at compile time
2. The embedded path must contain at least one file

To satisfy this, ./build/README.md is tracked in twigg using:
  /build/*
  !/build/README.md

However, Docusaurus deletes and regenerates the ./build directory on every
build, which would normally remove this file.

To prevent that, this README.md is mirrored from ./static into ./build on
every Docusaurus build.
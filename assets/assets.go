package assets

import _ "embed"

//go:embed config.example.toml
var DefaultConfigTOML []byte

//go:embed Messages.example.txt
var DefaultMessagesTXT []byte

package templates

import "embed"

//go:embed configs/*
var Configs embed.FS

//go:embed scripts/*
var Scripts embed.FS

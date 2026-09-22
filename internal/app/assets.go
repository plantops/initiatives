package app

import "embed"

//go:embed migrations/*.sql web/templates/*.html web/static/*
var assets embed.FS

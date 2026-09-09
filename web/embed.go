package web

import "embed"

//go:embed client/*
var Client embed.FS

//go:embed controller/*
var Controller embed.FS

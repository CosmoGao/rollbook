package skills

import "embed"

// Rollbook 固定嵌入发布时的 Skill，不依赖运行时工作目录。
//
//go:embed rollbook/*
var Rollbook embed.FS

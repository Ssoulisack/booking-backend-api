package service

import "backend/internal/platform/kit"

func badRequest(code string) error { return kit.BadRequest(code) }
func notFound(code string) error   { return kit.NotFound(code) }
func conflict(code string) error   { return kit.Conflict(code) }

package service

import (
	"backend/internal/platform/kit"
)

// The shared helpers live in platform/kit; these keep the short names this service already uses.

func conflict(code string) error   { return kit.Conflict(code) }
func badRequest(code string) error { return kit.BadRequest(code) }
func forbidden(code string) error  { return kit.Forbidden(code) }
func notFound(code string) error   { return kit.NotFound(code) }
func isNotFound(err error) bool    { return kit.IsNotFound(err) }

func orBadRequest(err error, code string) error { return kit.OrBadRequest(err, code) }

func mapSlice[M, D any](items []M, convert func(M) D) []D { return kit.MapSlice(items, convert) }

func newToken() (raw, hash string, err error) { return kit.NewToken() }
func hashToken(raw string) string             { return kit.HashToken(raw) }

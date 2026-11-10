package service

import (
	"backend/internal/core/port/output"
	"context"
	"log"
	"strings"
	"time"
)

var _ output.TeamNews = (*Service)(nil)

// Announce sends news to the branch's group in the background (see output.TeamNews)
func (s *Service) Announce(ctx context.Context, branchID uint, topic, title string, lines []string, customerName, customerPhone string) {
	ctx = context.WithoutCancel(ctx)
	go func() {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := s.announce(ctx, branchID, topic, title, lines, customerName, customerPhone); err != nil {
			log.Printf("⚠️  telegram news for branch %d failed: %v", branchID, err)
		}
	}()
}

func (s *Service) announce(ctx context.Context, branchID uint, topic, title string, lines []string, name, phone string) error {
	cur, err := s.Repo.Get(ctx, branchID)
	if err != nil || cur == nil || !cur.Enabled || cur.TokenEnc == "" || cur.ChatIDEnc == "" {
		return err
	}
	if (topic == output.NewsPayment && !cur.OnPayment) || (topic == output.NewsCancel && !cur.OnCancel) || (topic == output.NewsStaffCancel && !cur.OnStaffCancel) {
		return nil
	}
	token, chat, err := s.open(cur)
	if err != nil {
		_ = s.Repo.Result(ctx, branchID, "The server secret changed: connect the bot again")
		return err
	}
	if cur.HidePhone {
		phone = ""
	}
	out := append([]string{title, ""}, lines...)
	if who := strings.TrimSpace(name + " " + phone); who != "" {
		out = append(out, "ລູກຄ້າ: "+who)
	}
	if err := s.Bot.Send(ctx, token, chat, truncate(strings.Join(out, "\n"), 3500)); err != nil {
		_ = s.Repo.Result(ctx, branchID, errorText(err))
		return err
	}
	return s.Repo.Result(ctx, branchID, "")
}

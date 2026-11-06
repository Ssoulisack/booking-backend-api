package service

import (
	"backend/internal/core/domain"
	"backend/internal/core/port/output"
	pkgdomain "backend/internal/modules/packages/domain"
	"context"
	"fmt"
	"strings"
)

// tell sends news about a package to the team: the package, its price, and who. Lao, like every team message.
func (s *Service) tell(ctx context.Context, b *domain.Branch, cp *pkgdomain.CustomerPackage, title string, extra ...string) {
	if s.News == nil {
		return
	}
	var name, phone string
	if u, err := s.Users.FindByID(ctx, cp.CustomerID); err == nil && u != nil {
		name, phone = u.Name, u.Phone
	}
	unit := "ເທື່ອ"
	if cp.Kind == pkgdomain.OptionMonths {
		unit = "ເດືອນ"
	}
	lines := []string{"ສາຂາ: " + b.Name, "ລະຫັດ: " + cp.Code(),
		fmt.Sprintf("ແພັກເກັດ: %s · %d %s", cp.Name, cp.Amount, unit), "ລວມ: " + moneyText(cp.Price, cp.Currency)}
	lines = append(lines, extra...)
	s.News.Announce(ctx, b.ID, output.NewsPayment, title, lines, name, phone)
}

func method(m string) string {
	if m == domain.MethodOnline {
		return "ອອນລາຍ"
	}
	return strings.TrimSpace(m)
}

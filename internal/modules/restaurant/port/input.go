// Package port: what the restaurant module offers to the rest of the app (input ports)
package port

import (
	"backend/internal/core/domain"
	"backend/internal/core/dto"
	"context"
)

// TableService is how customers see a restaurant's floor plans and free tables
type TableService interface {
	// FloorPlans returns every bookable area with its tables and objects, lowest floor first
	FloorPlans(ctx context.Context, slug string) ([]dto.PublicAreaDTO, error)
	// TableAvailability answers "which times and tables are free for this many guests on this day"
	TableAvailability(ctx context.Context, slug string, q dto.TableAvailabilityQuery) (*dto.TableAvailabilityDTO, error)
}

// DeskTables is the restaurant team's work: the group came in, other tables, a group without
// booking, and the job that settles late groups (no-show) and evenings that are over (bill closed)
type DeskTables interface {
	MarkArrived(ctx context.Context, branchID, id uint) (*dto.BookingDTO, error)
	MoveTables(ctx context.Context, branchID, id uint, req dto.MoveTablesRequest) (*dto.BookingDTO, error)
	WalkIn(ctx context.Context, branchID uint, actor *domain.User, req dto.WalkInRequest) (*dto.BookingDTO, error)
	// DeskTableAvailability: the free times and tables for a table booking made by the team (a customer who phoned)
	DeskTableAvailability(ctx context.Context, branchID uint, q dto.TableAvailabilityQuery) (*dto.TableAvailabilityDTO, error)
	SettleTables(ctx context.Context) error
}

// Package service implements the ConnectRPC service handlers generated from
// proto/fitter/v1: UserService, CampaignService, GoalService, FeedService,
// CatalogService.
package service

import (
	"database/sql"
	"time"

	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/strava"
)

// Deps holds everything a service handler needs. Each handler type embeds
// *Deps rather than duplicating these fields.
//
// DB is the raw *sql.DB backing Queries — needed anywhere a handler must
// wrap multiple statements in one transaction (BeginTx + Queries.WithTx),
// which Queries alone cannot do.
type Deps struct {
	DB        *sql.DB
	Queries   *db.Queries
	Strava    *strava.Client
	JWTSecret []byte
	JWTTTL    time.Duration
}

type UserService struct{ *Deps }
type CampaignService struct{ *Deps }
type GoalService struct{ *Deps }
type FeedService struct{ *Deps }
type CatalogService struct{ *Deps }
type AdminService struct{ *Deps }

func NewUserService(d *Deps) *UserService         { return &UserService{d} }
func NewCampaignService(d *Deps) *CampaignService { return &CampaignService{d} }
func NewGoalService(d *Deps) *GoalService         { return &GoalService{d} }
func NewFeedService(d *Deps) *FeedService         { return &FeedService{d} }
func NewCatalogService(d *Deps) *CatalogService   { return &CatalogService{d} }
func NewAdminService(d *Deps) *AdminService       { return &AdminService{d} }

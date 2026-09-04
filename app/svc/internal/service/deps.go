// Package service implements the ConnectRPC service handlers generated from
// proto/fitter/v1: UserService, CampaignService, GoalService, FeedService.
package service

import (
	"time"

	"github.com/fitter/svc/internal/db"
	"github.com/fitter/svc/internal/strava"
)

// Deps holds everything a service handler needs. Each handler type embeds
// *Deps rather than duplicating these fields.
type Deps struct {
	Queries   *db.Queries
	Strava    *strava.Client
	JWTSecret []byte
	JWTTTL    time.Duration
}

type UserService struct{ *Deps }
type CampaignService struct{ *Deps }
type GoalService struct{ *Deps }
type FeedService struct{ *Deps }

func NewUserService(d *Deps) *UserService         { return &UserService{d} }
func NewCampaignService(d *Deps) *CampaignService { return &CampaignService{d} }
func NewGoalService(d *Deps) *GoalService         { return &GoalService{d} }
func NewFeedService(d *Deps) *FeedService         { return &FeedService{d} }

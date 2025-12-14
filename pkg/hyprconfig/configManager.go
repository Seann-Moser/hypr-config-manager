package hyprconfig

import (
	"context"

	"github.com/Seann-Moser/mserve"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ConfigManager interface {
	CreateConfig(ctx context.Context, cfg *HyprConfig) (*HyprConfig, error)
	GetConfig(ctx context.Context, id string) (*HyprConfig, error)
	UpdateConfig(ctx context.Context, id string, updates bson.M) error
	DeleteConfig(ctx context.Context, id string) error
	ListConfigs(
		ctx context.Context,
		page, limit int,
		findOpts *options.FindOptions,
	) (mserve.Page[HyprConfig], error)
	ListMyConfigs(
		ctx context.Context,
		page, limit int,
		findOpts *options.FindOptions,
	) (mserve.Page[HyprConfig], error)
	ListConfigsWithFilters(
		ctx context.Context,
		page, limit int,
		filters ConfigSearchFilters,
		findOpts *options.FindOptions,
	) (mserve.Page[HyprConfig], error)
	FavoriteConfig(ctx context.Context, configID string) error
	UnfavoriteConfig(ctx context.Context, configID string) error
	ListFavorites(
		ctx context.Context,
		page, limit int,
	) (mserve.Page[HyprConfig], error)
	ApplyConfig(ctx context.Context, configID string) error
	GetAppliedConfig(
		ctx context.Context,
	) (*HyprConfig, error)
	CountUsersUsingConfig(
		ctx context.Context,
		configID string,
	) (int64, error)
	AddProgramConfig(
		ctx context.Context,
		configID string,
		newProg HyprProgramConfig,
		parentID *string, // nil means insert at top-level
	) error
	RemoveProgramConfig(
		ctx context.Context,
		configID string,
		progID string,
	) error
	MoveProgramConfig(
		ctx context.Context,
		configID string,
		progID string,
		newParentID *string, // nil = move to top-level
	) error
	UpdateProgramConfig(
		ctx context.Context,
		configID string,
		progID string,
		updates HyprProgramConfig,
	) error
	AddAllowedProgram(ctx context.Context, programName string) (*AllowedPrograms, error)
	GetAllowedProgram(ctx context.Context, programName string) (*AllowedPrograms, error)
	ListAllowedPrograms(ctx context.Context) ([]AllowedPrograms, error)
	RemoveAllowedProgram(ctx context.Context, programName string) error
}

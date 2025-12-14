package hyprconfig

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Seann-Moser/mserve"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
)

type ConfigManagerMongo struct {
	Collection          *mongo.Collection // configs
	FavoritesCollection *mongo.Collection // user_favorites
	StateCollection     *mongo.Collection // user_hypr_state
	ProgramsCollection  *mongo.Collection // allowed_programs
}

func NewConfigManager(
	configs *mongo.Collection,
	favorites *mongo.Collection,
	state *mongo.Collection,
	programs *mongo.Collection, // NEW parameter
) (ConfigManager, error) {

	if configs == nil || favorites == nil || state == nil {
		return nil, errors.New("config manager: all collections must be non-nil")
	}

	m := &ConfigManagerMongo{
		Collection:          configs,
		FavoritesCollection: favorites,
		StateCollection:     state,
		ProgramsCollection:  programs,
	}

	// Create all required indexes
	if err := m.ensureIndexes(context.Background()); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *ConfigManagerMongo) ensureIndexes(ctx context.Context) error {

	// ---------------------------
	// CONFIGS COLLECTION INDEXES
	// ---------------------------
	_, err := m.ProgramsCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		// Ensure program names are unique
		{
			Keys:    bson.D{{"program_name", 1}},
			Options: options.Index().SetUnique(true).SetName("uid_program_name"),
		},
	})

	if err != nil {
		return fmt.Errorf("programs index error: %w", err)
	}

	_, err = m.Collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		// Sort by likes
		{
			Keys:    bson.D{{"likes", -1}},
			Options: options.Index().SetName("idx_likes_desc"),
		},
		// Sort by updated time
		{
			Keys:    bson.D{{"updated_timestamp", -1}},
			Options: options.Index().SetName("idx_updated_desc"),
		},
		// Text search support (title, description, tags)
		{
			Keys: bson.D{
				{"title", "text"},
				{"description", "text"},
				{"tags", "text"},
			},
			Options: options.Index().SetName("idx_text_search"),
		},
	})
	if err != nil {
		return fmt.Errorf("config index error: %w", err)
	}

	// -------------------------------------
	// FAVORITES COLLECTION INDEXES
	// -------------------------------------

	_, err = m.FavoritesCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		// Prevent duplicate favorites: (user_id, config_id)
		{
			Keys: bson.D{
				{"user_id", 1},
				{"config_id", 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("uid_config_unique"),
		},
		// Lookup favorites by config (for like rebuild)
		{
			Keys:    bson.D{{"config_id", 1}},
			Options: options.Index().SetName("config_id_idx"),
		},
	})

	if err != nil {
		return fmt.Errorf("favorites index error: %w", err)
	}

	// -------------------------------------
	// USER STATE COLLECTION INDEXES
	// -------------------------------------

	_, err = m.StateCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		// Each user can have only ONE applied config
		{
			Keys: bson.D{
				{"user_id", 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("user_unique"),
		},
		// Lookup who has a config applied
		{
			Keys:    bson.D{{"config_id", 1}},
			Options: options.Index().SetName("config_id_idx"),
		},
	})

	if err != nil {
		return fmt.Errorf("state index error: %w", err)
	}

	return nil
}

func (m *ConfigManagerMongo) CreateConfig(ctx context.Context, cfg *HyprConfig) (*HyprConfig, error) {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	cfg.ID = uuid.New().String()
	cfg.OwnerID = user.UserID
	cfg.CreatedTimestamp = time.Now()
	cfg.UpdatedTimestamp = time.Now()
	// --- NEW VALIDATION STEP ---
	if err := cfg.Validate(m.checkProgramExists); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	// ---------------------------
	_, err = m.Collection.InsertOne(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (m *ConfigManagerMongo) GetConfig(ctx context.Context, id string) (*HyprConfig, error) {
	user, _ := getUserFromContext(ctx) // user may be nil for public configs

	var cfg HyprConfig
	err := m.Collection.FindOne(ctx, bson.M{"_id": id}).Decode(&cfg)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	// PRIVATE CONFIG CHECK
	if cfg.Private {
		if user == nil || (cfg.OwnerID != user.UserID && !isAdmin(user.Roles)) {
			return nil, ErrForbidden
		}
	}
	return &cfg, nil
}

func (m *ConfigManagerMongo) UpdateConfig(ctx context.Context, id string, updates bson.M) error {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}

	// Fetch existing config
	var existing HyprConfig
	err = m.Collection.FindOne(ctx, bson.M{"_id": id}).Decode(&existing)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotFound
		}
		return err
	}

	// Ownership check
	if existing.OwnerID != user.UserID && !isAdmin(user.Roles) {
		return ErrForbidden
	}

	// Determine semantic version bump
	newVersion := bumpPatchVersion(existing.Version)
	updates["version"] = newVersion
	updates["updated_timestamp"] = time.Now()

	// Remove immutable fields if present in updates
	delete(updates, "_id")
	delete(updates, "owner_id")
	delete(updates, "likes")
	delete(updates, "created_timestamp")
	// WARNING: Assuming program_configs are updated via separate endpoints
	delete(updates, "program_configs")

	// --- NEW VALIDATION STEP ---
	// 1. Create a merged config for validation
	mergedCfg := existing

	// Convert the existing struct to a BSON map
	existingBSON, err := bson.Marshal(existing)
	if err != nil {
		return fmt.Errorf("failed to marshal existing config: %w", err)
	}

	var mergedMap bson.M
	if err := bson.Unmarshal(existingBSON, &mergedMap); err != nil {
		return fmt.Errorf("failed to unmarshal existing BSON: %w", err)
	}

	// 2. Apply updates to the map
	for k, v := range updates {
		mergedMap[k] = v
	}

	// 3. Convert the merged map back into a HyprConfig struct
	mergedBSON, err := bson.Marshal(mergedMap)
	if err != nil {
		return fmt.Errorf("failed to marshal merged map: %w", err)
	}
	if err := bson.Unmarshal(mergedBSON, &mergedCfg); err != nil {
		return fmt.Errorf("failed to unmarshal merged BSON into struct: %w", err)
	}

	// 4. Validate the resulting merged struct
	if err := mergedCfg.Validate(m.checkProgramExists); err != nil {
		return fmt.Errorf("merged config failed validation: %w", err)
	}
	// ---------------------------

	// Proceed with the update if validation passes
	_, err = m.Collection.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": updates},
	)
	return err
}

// bumpPatchVersion increases the PATCH number of a semantic version string (e.g., 1.2.3 -> 1.2.4)
func bumpPatchVersion(v string) string {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		// fallback if version is malformed
		return "0.0.1"
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		patch = 0
	}

	patch++
	return fmt.Sprintf("%s.%s.%d", parts[0], parts[1], patch)
}

func (m *ConfigManagerMongo) DeleteConfig(ctx context.Context, id string) error {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}

	var cfg HyprConfig
	err = m.Collection.FindOne(ctx, bson.M{"_id": id}).Decode(&cfg)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotFound
		}
		return err
	}

	if cfg.OwnerID != user.UserID && !isAdmin(user.Roles) {
		return ErrForbidden
	}

	_, err = m.Collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (m *ConfigManagerMongo) ListConfigs(
	ctx context.Context,
	page, limit int,
	findOpts *options.FindOptions,
) (mserve.Page[HyprConfig], error) {

	user, _ := getUserFromContext(ctx) // user may be nil

	// Filter:
	// Public configs OR configs owned by the user.
	filter := bson.M{
		"$or": []bson.M{
			{"private": false},
		},
	}

	if user != nil {
		filter["$or"] = append(filter["$or"].([]bson.M),
			bson.M{"owner_id": user.UserID},
		)
	}

	// Default sort if none provided: newest first
	if findOpts == nil {
		findOpts = options.Find().SetSort(bson.M{
			"updated_timestamp": -1,
		})
	}

	// Use your pagination helper
	return mserve.PaginateMongo[HyprConfig](
		ctx,
		m.Collection,
		filter,
		page,
		limit,
		findOpts,
	)
}

func (m *ConfigManagerMongo) ListMyConfigs(
	ctx context.Context,
	page, limit int,
	findOpts *options.FindOptions,
) (mserve.Page[HyprConfig], error) {

	user, err := getUserFromContext(ctx)
	if err != nil {
		return mserve.Page[HyprConfig]{}, err
	}

	filter := bson.M{
		"owner_id": user.UserID,
	}

	// Default: newest updated first
	if findOpts == nil {
		findOpts = options.Find().SetSort(bson.M{"updated_timestamp": -1})
	}

	return mserve.PaginateMongo[HyprConfig](
		ctx,
		m.Collection,
		filter,
		page,
		limit,
		findOpts,
	)
}

func (m *ConfigManagerMongo) ListConfigsWithFilters(
	ctx context.Context,
	page, limit int,
	filters ConfigSearchFilters,
	findOpts *options.FindOptions,
) (mserve.Page[HyprConfig], error) {

	user, _ := getUserFromContext(ctx) // user may be nil

	filter := buildSearchFilter(filters, user)

	if findOpts == nil {
		findOpts = options.Find().SetSort(bson.M{"updated_timestamp": -1})
	}

	return mserve.PaginateMongo[HyprConfig](
		ctx,
		m.Collection,
		filter,
		page,
		limit,
		findOpts,
	)
}

func (m *ConfigManagerMongo) FavoriteConfig(ctx context.Context, configID string) error {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}

	// Check if already favorited
	exists := m.FavoritesCollection.FindOne(ctx, bson.M{
		"user_id":   user.UserID,
		"config_id": configID,
	})

	if exists.Err() == nil {
		return nil // already favorited, ignore
	}

	// Insert new favorite entry
	_, err = m.FavoritesCollection.InsertOne(ctx, UserFavorite{
		UserID:      user.UserID,
		ConfigID:    configID,
		FavoritedAt: time.Now(),
	})
	if err != nil {
		return err
	}

	// Increment config's like count
	_, err = m.Collection.UpdateByID(ctx, configID, bson.M{
		"$inc": bson.M{"likes": 1},
	})
	return err
}

func (m *ConfigManagerMongo) UnfavoriteConfig(ctx context.Context, configID string) error {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}

	// Remove favorite entry
	res, err := m.FavoritesCollection.DeleteOne(ctx, bson.M{
		"user_id":   user.UserID,
		"config_id": configID,
	})
	if err != nil {
		return err
	}

	// Not favorited before → nothing to do
	if res.DeletedCount == 0 {
		return nil
	}

	// Decrement like count
	_, err = m.Collection.UpdateByID(ctx, configID, bson.M{
		"$inc": bson.M{"likes": -1},
	})

	return err
}

func (m *ConfigManagerMongo) ListFavorites(
	ctx context.Context,
	page, limit int,
) (mserve.Page[HyprConfig], error) {

	user, err := getUserFromContext(ctx)
	if err != nil {
		return mserve.Page[HyprConfig]{}, err
	}

	// first find config ids they have favorited
	cursor, err := m.FavoritesCollection.Find(ctx, bson.M{
		"user_id": user.UserID,
	})
	if err != nil {
		return mserve.Page[HyprConfig]{}, err
	}

	var favs []UserFavorite
	if err := cursor.All(ctx, &favs); err != nil {
		return mserve.Page[HyprConfig]{}, err
	}

	// Extract config IDs
	var ids []string
	for _, f := range favs {
		ids = append(ids, f.ConfigID)
	}

	filter := bson.M{"_id": bson.M{"$in": ids}}

	return mserve.PaginateMongo[HyprConfig](
		ctx,
		m.Collection,
		filter,
		page,
		limit,
		nil,
	)
}

func (m *ConfigManagerMongo) ApplyConfig(ctx context.Context, configID string) error {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}

	// Upsert the user’s applied config
	_, err = m.StateCollection.UpdateOne(
		ctx,
		bson.M{"user_id": user.UserID},
		bson.M{
			"$set": bson.M{
				"config_id":  configID,
				"applied_at": time.Now(),
			},
		},
		options.Update().SetUpsert(true),
	)

	return err
}

func (m *ConfigManagerMongo) GetAppliedConfig(
	ctx context.Context,
) (*HyprConfig, error) {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var state UserHyprState
	err = m.StateCollection.FindOne(ctx, bson.M{
		"user_id": user.UserID,
	}).Decode(&state)
	if err != nil {
		return nil, ErrNotFound
	}

	return m.GetConfig(ctx, state.ConfigID)
}

func (m *ConfigManagerMongo) CountUsersUsingConfig(
	ctx context.Context,
	configID string,
) (int64, error) {

	return m.StateCollection.CountDocuments(ctx, bson.M{
		"config_id": configID,
	})
}

func (m *ConfigManagerMongo) AddProgramConfig(
	ctx context.Context,
	configID string,
	newProg HyprProgramConfig,
	parentID *string, // nil means insert at top-level
) error {

	user, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}

	// Fetch the config to check permissions and modify in memory
	var cfg HyprConfig
	if err := m.Collection.FindOne(ctx, bson.M{"_id": configID}).Decode(&cfg); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotFound
		}
		return err
	}

	// Owner or Admin required
	if cfg.OwnerID != user.UserID && !isAdmin(user.Roles) {
		return ErrForbidden
	}

	// Ensure ID exists
	if newProg.ID == "" {
		newProg.ID = uuid.NewString()
	}

	now := time.Now()
	newProg.CreatedTimestamp = now
	newProg.UpdatedTimestamp = now

	// ----------------------
	// Top-level insert
	// ----------------------
	if parentID == nil || *parentID == "" {
		cfg.ProgramConfigs = append(cfg.ProgramConfigs, newProg)

		_, err = m.Collection.UpdateByID(ctx, configID, bson.M{
			"$set": bson.M{
				"program_configs":   cfg.ProgramConfigs,
				"updated_timestamp": now,
			},
		})
		return err
	}

	// ----------------------
	// Insert into a parent sub-config (recursive)
	// ----------------------
	inserted := insertIntoSubConfig(cfg.ProgramConfigs, newProg, *parentID)
	if !inserted {
		return fmt.Errorf("parent program config with ID %s not found", *parentID)
	}

	// Write back
	_, err = m.Collection.UpdateByID(ctx, configID, bson.M{
		"$set": bson.M{
			"program_configs":   cfg.ProgramConfigs,
			"updated_timestamp": now,
		},
	})
	return err
}

// insertIntoSubConfig recursively searches for parentID and inserts newProg into its SubConfigs.
// Returns true if inserted, false otherwise.
func insertIntoSubConfig(
	list []HyprProgramConfig,
	newProg HyprProgramConfig,
	parentID string,
) bool {

	for i := range list {
		// Match parent
		if list[i].ID == parentID {
			list[i].SubConfigs = append(list[i].SubConfigs, &newProg)
			return true
		}

		// Check nested subconfigs
		if len(list[i].SubConfigs) > 0 {
			if insertIntoNested(list[i].SubConfigs, newProg, parentID) {
				return true
			}
		}
	}

	return false
}

// Same logic but for []*HyprProgramConfig
func insertIntoNested(
	list []*HyprProgramConfig,
	newProg HyprProgramConfig,
	parentID string,
) bool {

	for i := range list {
		if list[i].ID == parentID {
			list[i].SubConfigs = append(list[i].SubConfigs, &newProg)
			return true
		}

		if len(list[i].SubConfigs) > 0 {
			if insertIntoNested(list[i].SubConfigs, newProg, parentID) {
				return true
			}
		}
	}

	return false
}

func (m *ConfigManagerMongo) RemoveProgramConfig(
	ctx context.Context,
	configID string,
	progID string,
) error {

	user, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}

	// Load full config (needed for nested removal)
	var cfg HyprConfig
	if err := m.Collection.FindOne(ctx, bson.M{"_id": configID}).Decode(&cfg); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotFound
		}
		return err
	}

	// Owner/Admin validation
	if cfg.OwnerID != user.UserID && !isAdmin(user.Roles) {
		return ErrForbidden
	}

	// --------
	// Attempt top-level removal
	// --------
	res, err := m.Collection.UpdateByID(ctx, configID, bson.M{
		"$pull": bson.M{
			"program_configs": bson.M{"id": progID},
		},
	})
	if err != nil {
		return err
	}

	if res.ModifiedCount > 0 {
		// Found and removed at top-level, just update timestamp
		_, _ = m.Collection.UpdateByID(ctx, configID, bson.M{
			"$set": bson.M{
				"updated_timestamp": time.Now(),
			},
		})
		return nil
	}

	// Otherwise, must remove from nested SubConfigs
	updatedList := removeNestedProgramConfig(cfg.ProgramConfigs, progID)

	// Write updated ProgramConfigs back
	_, err = m.Collection.UpdateByID(ctx, configID, bson.M{
		"$set": bson.M{
			"program_configs":   updatedList,
			"updated_timestamp": time.Now(),
		},
	})
	return err
}

func removeNestedProgramConfig(
	list []HyprProgramConfig,
	targetID string,
) []HyprProgramConfig {

	newList := make([]HyprProgramConfig, 0, len(list))

	for _, item := range list {
		// Remove matching subconfigs recursively
		if len(item.SubConfigs) > 0 {
			item.SubConfigs = filterSubConfigs(item.SubConfigs, targetID)
		}

		// Keep this item if it’s NOT the target
		if item.ID != targetID {
			newList = append(newList, item)
		}
	}

	return newList
}

func filterSubConfigs(
	sub []*HyprProgramConfig,
	targetID string,
) []*HyprProgramConfig {

	var result []*HyprProgramConfig
	for _, sc := range sub {
		// Recursively fix sub-subconfigs
		if len(sc.SubConfigs) > 0 {
			sc.SubConfigs = filterSubConfigs(sc.SubConfigs, targetID)
		}

		// If this subconfig IS the one being removed → skip it
		if sc.ID == targetID {
			continue
		}

		result = append(result, sc)
	}
	return result
}

func (m *ConfigManagerMongo) MoveProgramConfig(
	ctx context.Context,
	configID string,
	progID string,
	newParentID *string, // nil = move to top-level
) error {

	user, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}

	// Load config
	var cfg HyprConfig
	if err := m.Collection.FindOne(ctx, bson.M{"_id": configID}).Decode(&cfg); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotFound
		}
		return err
	}

	// Permission check
	if cfg.OwnerID != user.UserID && !isAdmin(user.Roles) {
		return ErrForbidden
	}

	// 1. Remove program config
	var removed *HyprProgramConfig
	cfg.ProgramConfigs, removed = extractProgramConfig(cfg.ProgramConfigs, progID)
	if removed == nil {
		return fmt.Errorf("program config with ID %s not found", progID)
	}

	// Cleanup nested timestamps
	now := time.Now()
	removed.UpdatedTimestamp = now

	// 2. Insert program config into new parent or top-level
	if newParentID == nil || *newParentID == "" {
		// Move to top-level
		cfg.ProgramConfigs = append(cfg.ProgramConfigs, *removed)
	} else {
		if !insertIntoSubConfig(cfg.ProgramConfigs, *removed, *newParentID) {
			return fmt.Errorf("parent program config with ID %s not found", *newParentID)
		}
	}

	// 3. Write changes back to Mongo
	_, err = m.Collection.UpdateByID(ctx, configID, bson.M{
		"$set": bson.M{
			"program_configs":   cfg.ProgramConfigs,
			"updated_timestamp": now,
		},
	})
	return err
}

func extractProgramConfig(
	list []HyprProgramConfig,
	progID string,
) ([]HyprProgramConfig, *HyprProgramConfig) {

	newList := make([]HyprProgramConfig, 0, len(list))

	for _, item := range list {
		if item.ID == progID {
			return newList, &item
		}

		// Search nested subconfigs
		if len(item.SubConfigs) > 0 {
			subNew, removed := extractProgramConfigNested(item.SubConfigs, progID)
			if removed != nil {
				item.SubConfigs = subNew
				newList = append(newList, item)
				return newList, removed
			}
		}

		newList = append(newList, item)
	}

	return newList, nil
}

func extractProgramConfigNested(
	list []*HyprProgramConfig,
	progID string,
) ([]*HyprProgramConfig, *HyprProgramConfig) {

	newList := make([]*HyprProgramConfig, 0, len(list))

	for _, sc := range list {
		if sc.ID == progID {
			return newList, sc
		}

		if len(sc.SubConfigs) > 0 {
			subNew, removed := extractProgramConfigNested(sc.SubConfigs, progID)
			if removed != nil {
				sc.SubConfigs = subNew
				newList = append(newList, sc)
				return newList, removed
			}
		}

		newList = append(newList, sc)
	}

	return newList, nil
}

func (m *ConfigManagerMongo) UpdateProgramConfig(
	ctx context.Context,
	configID string,
	progID string,
	updates HyprProgramConfig,
) error {

	user, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}

	// Load config
	var cfg HyprConfig
	if err := m.Collection.FindOne(ctx, bson.M{"_id": configID}).Decode(&cfg); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotFound
		}
		return err
	}

	// Check permissions
	if cfg.OwnerID != user.UserID && !isAdmin(user.Roles) {
		return ErrForbidden
	}

	now := time.Now()

	// Perform recursive update
	updated, ok := updateProgramConfigRecursive(cfg.ProgramConfigs, progID, updates, now)
	if !ok {
		return fmt.Errorf("program config with ID %s not found", progID)
	}

	// Write back
	_, err = m.Collection.UpdateByID(ctx, configID, bson.M{
		"$set": bson.M{
			"program_configs":   updated,
			"updated_timestamp": now,
		},
	})
	return err
}

func updateProgramConfigRecursive(
	list []HyprProgramConfig,
	progID string,
	updates HyprProgramConfig,
	now time.Time,
) ([]HyprProgramConfig, bool) {

	for i := range list {
		if list[i].ID == progID {

			// Preserve immutable fields
			updates.ID = progID
			updates.CreatedTimestamp = list[i].CreatedTimestamp

			// Force updated timestamp
			updates.UpdatedTimestamp = now

			// Preserve existing subconfigs
			updates.SubConfigs = list[i].SubConfigs

			list[i] = updates
			return list, true
		}

		// Search in nested
		if len(list[i].SubConfigs) > 0 {
			done := false
			list[i].SubConfigs, done = updateSubConfigRecursive(list[i].SubConfigs, progID, updates, now)
			if done {
				return list, true
			}
		}
	}

	return list, false
}

func updateSubConfigRecursive(
	list []*HyprProgramConfig,
	progID string,
	updates HyprProgramConfig,
	now time.Time,
) ([]*HyprProgramConfig, bool) {

	for i := range list {
		if list[i].ID == progID {

			// Keep immutable fields
			updates.ID = progID
			updates.CreatedTimestamp = list[i].CreatedTimestamp
			updates.SubConfigs = list[i].SubConfigs

			// Replace
			list[i] = &updates
			list[i].UpdatedTimestamp = now

			return list, true
		}

		// Check sub-sub configs
		if len(list[i].SubConfigs) > 0 {
			done := false
			list[i].SubConfigs, done = updateSubConfigRecursive(list[i].SubConfigs, progID, updates, now)
			if done {
				return list, true
			}
		}
	}

	return list, false
}

// checkProgramExists queries the database to see if a program name is currently allowed.
func (m *ConfigManagerMongo) checkProgramExists(ctx context.Context, programName string) error {
	var allowedProgram AllowedPrograms
	err := m.ProgramsCollection.FindOne(ctx, bson.M{"program_name": programName}).Decode(&allowedProgram)

	if errors.Is(err, mongo.ErrNoDocuments) {
		// Program not found in the AllowedPrograms collection
		return fmt.Errorf("program '%s' is not in the list of allowed programs", programName)
	}
	if err != nil {
		// Database error during lookup
		return fmt.Errorf("database error checking program '%s': %w", programName, err)
	}

	// Program found
	return nil
}

// AddAllowedProgram inserts a new program name into the allowed list.
func (m *ConfigManagerMongo) AddAllowedProgram(ctx context.Context, programName string) (*AllowedPrograms, error) {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Admin check is crucial for managing the allowed program list
	if !isAdmin(user.Roles) {
		return nil, ErrForbidden
	}

	programName = strings.ToLower(strings.TrimSpace(programName))
	if programName == "" {
		return nil, errors.New("program name cannot be empty")
	}

	newProgram := AllowedPrograms{
		ProgramName: programName,
	}

	_, err = m.ProgramsCollection.InsertOne(ctx, newProgram)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, fmt.Errorf("program '%s' is already allowed", programName)
		}
		return nil, fmt.Errorf("failed to insert allowed program: %w", err)
	}

	return &newProgram, nil
}

// GetAllowedProgram retrieves a single allowed program definition by its name.
func (m *ConfigManagerMongo) GetAllowedProgram(ctx context.Context, programName string) (*AllowedPrograms, error) {
	programName = strings.ToLower(strings.TrimSpace(programName))
	if programName == "" {
		return nil, errors.New("program name cannot be empty")
	}

	var program AllowedPrograms
	err := m.ProgramsCollection.FindOne(ctx, bson.M{"program_name": programName}).Decode(&program)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch allowed program: %w", err)
	}

	return &program, nil
}

// ListAllowedPrograms retrieves all program names in the allowed list.
func (m *ConfigManagerMongo) ListAllowedPrograms(ctx context.Context) ([]AllowedPrograms, error) {
	// No admin check here, as this list is often public for config creation.

	cursor, err := m.ProgramsCollection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to list allowed programs: %w", err)
	}
	defer cursor.Close(ctx)

	var programs []AllowedPrograms
	if err := cursor.All(ctx, &programs); err != nil {
		return nil, fmt.Errorf("failed to decode allowed programs: %w", err)
	}

	return programs, nil
}

// RemoveAllowedProgram deletes a program name from the allowed list.
func (m *ConfigManagerMongo) RemoveAllowedProgram(ctx context.Context, programName string) error {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return err
	}

	// Admin check is required to delete an allowed program
	if !isAdmin(user.Roles) {
		return ErrForbidden
	}

	programName = strings.ToLower(strings.TrimSpace(programName))
	if programName == "" {
		return errors.New("program name cannot be empty")
	}

	res, err := m.ProgramsCollection.DeleteOne(ctx, bson.M{"program_name": programName})
	if err != nil {
		return fmt.Errorf("failed to delete allowed program: %w", err)
	}

	if res.DeletedCount == 0 {
		return ErrNotFound
	}

	// NOTE: Deleting an allowed program should ideally trigger a warning or cleanup
	// process for any existing HyprConfigs that rely on this program.
	// This is a complex cascading logic step that you might implement later.

	return nil
}

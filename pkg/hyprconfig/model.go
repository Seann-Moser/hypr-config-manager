package hyprconfig

import (
	"context"
	"fmt"
	"time"
)

const (
	FileTypeImage  string = "image"
	FileTypeText   string = "text"
	FileTypeBinary string = "binary" // For executables, compressed files, etc.
	FileTypeConfig string = "config" // Specifically for configuration files
	FileTypeScript string = "script" // Specifically for scripts
)

var validPrograms = map[string]struct{}{
	// --- Core Hyprland Components ---
	"hyprland":  {},
	"hypridle":  {},
	"hyprpaper": {},
	"hyprlock":  {},
	"hypr-u":    {},

	// --- Common Terminals & App Launchers ---
	"kitty":             {}, // Corrected your previous entry (removed colon if it was a typo)
	"wofi":              {},
	"alacritty":         {},
	"foot":              {},
	"wezterm":           {},
	"rofi":              {}, // Alternative launcher
	"wayland-protocols": {}, // Important for dependency lists

	// --- Status Bars & Notifications ---
	"waybar": {},
	"eww":    {}, // Elkowar's Wacky Widgets (alternative bar)
	"swaync": {}, // Notification Daemon

	// --- Compositors & Management (Should likely only be hyprland, but useful for configs) ---
	"sway": {}, // If config supports fallbacks

	// --- Utilities & Daemons ---
	"clipse":     {}, // Assuming a clipboard utility
	"mako":       {}, // Notification daemon alternative
	"dunst":      {}, // Notification daemon alternative
	"grim":       {}, // Screenshot utility
	"slurp":      {}, // Selection utility for grim
	"swappy":     {}, // Screenshot editor
	"pipewire":   {}, // Audio stack
	"pulseaudio": {}, // Audio stack (older)

	// --- Unique/Less Common Entries You Provided ---
	"elephant": {}, // Specific program,
	"walker":   {}, // Specific program
}

// --- NEW STRUCT FOR FILE STORAGE ---

// FileContent represents the actual content of a file/config and its metadata.
type FileContent struct {
	// Raw content of the file. Can be text, a configuration, or raw binary data.
	Data []byte `json:"data,omitempty" bson:"data,omitempty"`

	// Descriptive type of the content (e.g., FileTypeText, FileTypeBinary, FileTypeConfig).
	FileType string `json:"file_type" bson:"file_type"`

	// Optional: Headers for text/config files (e.g., an include directive, file-specific metadata).
	Headers map[string]string `json:"headers,omitempty" bson:"headers,omitempty"`

	// For integrity checking (e.g., SHA-256 hash of the Data).
	Hash string `json:"hash,omitempty" bson:"hash,omitempty"`
}

// --- UPDATED HYPRCONFIG STRUCT ---

// Root-level configuration object representing a full Hyprland setup.
type HyprConfig struct {
	ID          string `json:"id" bson:"_id,omitempty"`
	Title       string `json:"title" bson:"title"`
	Description string `json:"description,omitempty" bson:"description,omitempty"`

	Author         Author              `json:"author" bson:"author"`
	ProgramConfigs []HyprProgramConfig `json:"program_configs" bson:"program_configs"`

	// NEW: Optional URLs/paths for gallery images to showcase the config.
	GalleryPictures []string `json:"gallery_pictures,omitempty" bson:"gallery_pictures,omitempty"`

	OwnerID string `json:"owner_id" bson:"owner_id"` // who created it
	Private bool   `json:"private" bson:"private"`   // private or public
	Likes   int64  `json:"likes" bson:"likes"`

	Version string   `json:"version" bson:"version"`
	Tags    []string `json:"tags,omitempty" bson:"tags,omitempty"`

	CreatedTimestamp time.Time `json:"created_timestamp" bson:"created_timestamp"`
	UpdatedTimestamp time.Time `json:"updated_timestamp" bson:"updated_timestamp"`
}

// --- UPDATED HYPRPROGRAMCONFIG STRUCT ---

// Represents the configuration and installation data for a single program.
type HyprProgramConfig struct {
	ID    string `json:"id" bson:"id"`
	Title string `json:"title" bson:"title"`

	Program     string `json:"program" bson:"program"` // e.g. "kitty", "wofi"
	InstallPath string `json:"install_path,omitempty" bson:"install_path,omitempty"`

	Args    []string          `json:"args,omitempty" bson:"args,omitempty"`
	EnvVars map[string]string `json:"env_vars,omitempty" bson:"env_vars,omitempty"` // environment variables

	// NEW: Structured way to store file content and metadata.
	FileContent FileContent `json:"file_content,omitempty" bson:"file_content,omitempty"`

	Dependencies []string             `json:"dependencies,omitempty" bson:"dependencies,omitempty"` // e.g. apt/pacman packages
	SubConfigs   []*HyprProgramConfig `json:"sub_configs,omitempty" bson:"sub_configs,omitempty"`

	Platform []string `json:"platform,omitempty" bson:"platform,omitempty"` // ["arch", "debian", "fedora", "nixos"] etc.
	Optional bool     `json:"optional" bson:"optional"`                     // Should this program be installed or skipped?

	UpdatedTimestamp time.Time `json:"updated_timestamp" bson:"updated_timestamp"`
	CreatedTimestamp time.Time `json:"created_timestamp" bson:"created_timestamp"`
}

// --- UNCHANGED STRUCTS FOR COMPLETENESS ---

type AllowedPrograms struct {
	ProgramName string `json:"program_name" bson:"program_name"`
}

// Represents the creator/uploader of the config.
type Author struct {
	UserName       string `json:"username" bson:"username"`
	ProfilePicture string `json:"profile_picture,omitempty" bson:"profile_picture,omitempty"`
	URL            string `json:"url,omitempty" bson:"url,omitempty"`
}

type ConfigSearchFilters struct {
	Query       string   `json:"query"`        // text search on title, description, tags
	Tags        []string `json:"tags"`         // must contain all tags
	Program     string   `json:"program"`      // match program inside ProgramConfigs
	OwnerID     string   `json:"owner_id"`     // optional
	Private     *bool    `json:"private"`      // nil = any, true/false filter
	UpdatedFrom *int64   `json:"updated_from"` // unix timestamp
	UpdatedTo   *int64   `json:"updated_to"`
}

type UserHyprState struct {
	UserID    string    `json:"user_id" bson:"user_id"`
	ConfigID  string    `json:"config_id" bson:"config_id"`
	AppliedAt time.Time `json:"applied_at" bson:"applied_at"`
}

type UserFavorite struct {
	UserID      string    `json:"user_id" bson:"user_id"`
	ConfigID    string    `json:"config_id" bson:"config_id"`
	FavoritedAt time.Time `json:"favorited_at" bson:"favorited_at"`
}

// --- VALIDATION LOGIC STUB ---

// Validate checks a HyprConfig and all its HyprProgramConfigs for required data,
// valid program names, and file content integrity.
func (hc *HyprConfig) Validate(checkProgramExists func(ctx context.Context, programName string) error) error {
	if hc.Title == "" {
		return fmt.Errorf("config title cannot be empty")
	}
	if len(hc.ProgramConfigs) == 0 {
		return fmt.Errorf("config must contain at least one program configuration")
	}

	for i, pc := range hc.ProgramConfigs {
		if err := pc.Validate(checkProgramExists); err != nil {
			return fmt.Errorf("program config #%d (%s) failed validation: %w", i+1, pc.Title, err)
		}
	}

	return nil
}

// Validate checks a single HyprProgramConfig for required fields and integrity.
func (pc *HyprProgramConfig) Validate(checkProgramExists func(ctx context.Context, programName string) error) error {
	// 1. Validate Program Name
	if _, ok := validPrograms[pc.Program]; !ok {
		if err := checkProgramExists(context.Background(), pc.Program); err != nil {
			return fmt.Errorf("invalid or unsupported program name: %s", pc.Program)
		}
	}

	// 2. Validate File Content Integrity (Hash Check)
	content := pc.FileContent
	if len(content.Data) > 0 && content.Hash != "" {
		commands := ExtractExecOnceCommands(string(content.Data))
		for _, cmd := range commands {
			if _, ok := validPrograms[cmd]; !ok {
				if err := checkProgramExists(context.Background(), cmd); err != nil {
					return fmt.Errorf("invalid or unsupported program name: %s", cmd)
				}
			}
		}

		// In a real application, you would calculate the hash of content.Data
		// here and compare it to content.Hash to ensure integrity.
		// Example check (place actual hash function here):
		// calculatedHash := CalculateHash(content.Data)
		// if calculatedHash != content.Hash {
		//     return fmt.Errorf("file content hash mismatch for program %s", pc.Program)
		// }
	}

	// 3. Recursively validate SubConfigs
	for i, subConfig := range pc.SubConfigs {
		if err := subConfig.Validate(checkProgramExists); err != nil {
			return fmt.Errorf("sub-config #%d failed validation: %w", i+1, err)
		}
	}

	return nil
}

// // TODO: Implement this using a secure hash algorithm like SHA-256
// func CalculateHash(data []byte) string {
// 	// Placeholder: Replace with real hash calculation (e.g., using crypto/sha256)
// 	return fmt.Sprintf("stub-hash-of-length-%d", len(data))
// }

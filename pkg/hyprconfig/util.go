package hyprconfig

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Seann-Moser/credentials/session"
	"github.com/Seann-Moser/hypr-config-manager/pkg/utils"
	"go.mongodb.org/mongo-driver/bson"
)

func getUserFromContext(ctx context.Context) (*session.UserSessionData, error) {
	user, err := session.GetSession(ctx)
	if err != nil {
		return nil, ErrUnauthorized
	}
	if !user.SignedIn {
		return nil, ErrUnauthorized
	}
	return user, nil
}

func isAdmin(roles []string) bool {
	for _, r := range roles {
		if r == "admin" {
			return true
		}
	}
	return false
}

func buildSearchFilter(filters ConfigSearchFilters, user *session.UserSessionData) bson.M {
	andParts := []bson.M{}

	// 🔍 Text Search (title, description, tags)
	if filters.Query != "" {
		q := filters.Query
		andParts = append(andParts, bson.M{
			"$or": []bson.M{
				{"title": bson.M{"$regex": q, "$options": "i"}},
				{"description": bson.M{"$regex": q, "$options": "i"}},
				{"tags": bson.M{"$regex": q, "$options": "i"}},
			},
		})
	}

	// 🏷 Tags Filter (must contain all tags)
	if len(filters.Tags) > 0 {
		andParts = append(andParts, bson.M{
			"tags": bson.M{"$all": filters.Tags},
		})
	}

	// 🖥 Program filter
	if filters.Program != "" {
		andParts = append(andParts, bson.M{
			"program_configs.program": filters.Program,
		})
	}

	// 👤 Owner filter
	if filters.OwnerID != "" {
		andParts = append(andParts, bson.M{
			"owner_id": filters.OwnerID,
		})
	}

	// 🔐 Private filter
	if filters.Private != nil {
		andParts = append(andParts, bson.M{
			"private": *filters.Private,
		})
	}

	// 🕒 Date Range Filter
	if filters.UpdatedFrom != nil || filters.UpdatedTo != nil {
		rangeFilter := bson.M{}
		if filters.UpdatedFrom != nil {
			rangeFilter["$gte"] = time.Unix(*filters.UpdatedFrom, 0)
		}
		if filters.UpdatedTo != nil {
			rangeFilter["$lte"] = time.Unix(*filters.UpdatedTo, 0)
		}
		andParts = append(andParts, bson.M{"updated_timestamp": rangeFilter})
	}

	// 🔒 Respect visibility rules:
	// Private configs only visible to owners or admins
	orClause := []bson.M{
		{"private": false},
	}

	if user != nil {
		orClause = append(orClause, bson.M{
			"owner_id": user.UserID,
		})
	}

	// Final Filter
	finalFilter := bson.M{
		"$or": []bson.M(orClause),
	}

	if len(andParts) > 0 {
		finalFilter["$and"] = andParts
	}

	return finalFilter
}

// StringSlicesEqual checks if two slices contain the same set of strings,
// ignoring order and duplicates.
func StringSlicesEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}

	// Convert first slice to a map
	mapA := make(map[string]int)
	for _, s := range a {
		mapA[s]++
	}

	// Convert second slice to a map
	mapB := make(map[string]int)
	for _, s := range b {
		mapB[s]++
	}

	// Compare both maps
	if len(mapA) != len(mapB) {
		return false
	}

	for key, countA := range mapA {
		if countB, ok := mapB[key]; !ok || countB != countA {
			return false
		}
	}

	return true
}

func ExtractLines(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var sourceLines []string
	var customStartReached bool
	var customSection []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Check if the line starts with "source="
		if strings.HasPrefix(line, "source=") {
			sourceLines = append(sourceLines, line)
		}

		// Start of CUSTOM section
		if line == "### CUSTOM START" {
			customStartReached = true
			customSection = append(customSection, line)
			continue
		}

		// End of CUSTOM section
		if line == "### CUSTOM END" && customStartReached {
			customSection = append(customSection, line)
			break
		}

		// Collect lines inside CUSTOM section
		if customStartReached {
			customSection = append(customSection, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Print or return the extracted lines
	fmt.Println("Source Lines:")
	for _, line := range sourceLines {
		fmt.Println(line)
	}

	fmt.Println("\nCustom Section:")
	for _, line := range customSection {
		fmt.Println(line)
	}

	return sourceLines, nil
}

// ParseKeyValuePairs takes a string and returns a map of key-value pairs
func ParseKeyValuePairs(input string) map[string]string {
	// Define a regular expression to match the pattern "$key = value"
	re := regexp.MustCompile(`\$(\w+)\s*=\s*(\S+)`)

	// Create a map to store the key-value pairs
	result := make(map[string]string)

	// Find all matches in the input string
	matches := re.FindAllStringSubmatch(input, -1)

	// Loop through each match and populate the map
	for _, match := range matches {
		// match[1] is the key (e.g., "terminal"), and match[2] is the value (e.g., "kitty")
		result["$"+match[1]] = match[2]
	}

	return result
}

var ignore = map[string]struct{}{
	"va11-popup":   {},
	"va11-confirm": {},
}

// ExtractExecOnceCommands takes a multi-line string and returns a list of commands and arguments, separated
func ExtractExecOnceCommands(input string) []string {
	// Regular expression to match lines with exec or exec-once
	pairs := ParseKeyValuePairs(input)
	reList := []*regexp.Regexp{
		regexp.MustCompile(`#*\s*exec-once\s*=\s*([^\n]+)`),
		regexp.MustCompile(`#*\s*exec\s*[=,]\s*([^\n]+)`),
	}

	var commands []string
	for _, re := range reList {
		// Find all matches for the exec or exec-once pattern
		matches := re.FindAllStringSubmatch(input, -1)

		for _, match := range matches {
			// match[1] contains the command and its arguments (after exec= or exec-once=)
			if strings.Contains(match[0], "#") {
				continue
			}
			commandLine := match[1]

			// Split by '&' or '&&' to handle both simple background execution and sequential execution
			parts := strings.FieldsFunc(commandLine, func(c rune) bool {
				return c == '&' || c == '\n' || c == ';'
			})

			for _, part := range parts {
				// Trim whitespace and split by spaces to handle command with arguments
				pts := strings.Fields(strings.TrimSpace(part))
				if len(pts) > 0 {
					if v, ok := pairs[pts[0]]; ok {
						if _, ok := ignore[strings.TrimSpace(v)]; ok {
							continue
						}
						commands = append(commands, strings.TrimSpace(v)) // Get only the main command
					} else {
						if _, ok := ignore[strings.TrimSpace(pts[0])]; ok {
							continue
						}
						commands = append(commands, strings.TrimSpace(pts[0])) // Get only the main command
					}
				}
			}
		}
	}

	return utils.DeduplicateStrings(commands)
}

// ExtractExecOnceCommands takes a multi-line string and returns a list of commands and arguments, separated
func ExtractExecOnceCommandsFile(input string) ([]string, error) {
	// Regular expression to match "exec-once" lines
	data, err := os.ReadFile(input)
	if err != nil {
		return nil, err
	}
	return ExtractExecOnceCommands(string(data)), nil
}

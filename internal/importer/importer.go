package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/registry/internal/service"
	"github.com/modelcontextprotocol/registry/internal/storage"
	"github.com/modelcontextprotocol/registry/internal/validators"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
)

// Service handles importing seed data into the registry
type Service struct {
	registry service.RegistryService
	storage  storage.Storage
}

// NewService creates a new importer service
func NewService(registry service.RegistryService, st storage.Storage) *Service {
	return &Service{
		registry: registry,
		storage:  st,
	}
}

// ImportFromPath imports seed data from various sources with full database reload:
// 1. Local file paths (*.json files) - expects ServerJSON array format
// 2. Direct HTTP URLs to seed.json files - expects ServerJSON array format
// 3. Registry root URLs (automatically appends /v0/servers and paginates)
//
// This performs a full reload: clears all existing servers and imports the seed data
func (s *Service) ImportFromPath(ctx context.Context, path string) error {
	servers, err := readSeedFile(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to read seed data: %w", err)
	}

	log.Printf("Starting full storage reload with %d servers from seed file", len(servers))

	// Step 1: Clear all existing servers
	log.Printf("Clearing existing servers from storage...")
	if err := s.storage.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear existing servers: %w", err)
	}
	log.Printf("Successfully cleared existing servers")

	// Step 2: Import all servers from seed
	var successfullyCreated []string
	var failedCreations []string

	for _, server := range servers {
		// Use the service's CreateServer which handles all the business logic
		_, err := s.registry.CreateServer(ctx, server)
		if err != nil {
			failedCreations = append(failedCreations, fmt.Sprintf("%s: %v", server.Name, err))
			log.Printf("Failed to import server %s: %v", server.Name, err)
		} else {
			successfullyCreated = append(successfullyCreated, server.Name)
		}
	}

	// Report import results
	log.Printf("Import completed: %d servers created successfully, %d servers failed",
		len(successfullyCreated), len(failedCreations))

	if len(failedCreations) > 0 {
		log.Printf("Failed servers: %v", failedCreations)
		// Don't fail the operation - just log warnings
	}

	log.Printf("Full storage reload completed successfully")
	return nil
}

// readSeedFile reads seed data from various sources
func readSeedFile(ctx context.Context, path string) ([]*apiv0.ServerJSON, error) {
	var data []byte
	var err error

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		// Handle HTTP URLs
		if strings.HasSuffix(path, "/v0/servers") || strings.Contains(path, "/v0/servers") {
			// This is a registry API endpoint - fetch paginated data
			return fetchFromRegistryAPI(ctx, path)
		}
		// This is a direct file URL
		data, err = fetchFromHTTP(ctx, path)
	} else {
		// Handle local file paths
		data, err = os.ReadFile(path)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read seed data from %s: %w", path, err)
	}

	// Parse ServerJSON array format
	var serverResponses []apiv0.ServerJSON
	if err := json.Unmarshal(data, &serverResponses); err != nil {
		return nil, fmt.Errorf("failed to parse seed data as ServerJSON array format: %w", err)
	}

	if len(serverResponses) == 0 {
		return []*apiv0.ServerJSON{}, nil
	}

	// Validate servers and collect warnings instead of failing the whole batch
	var validRecords []*apiv0.ServerJSON
	var invalidServers []string
	var validationFailures []string

	for _, response := range serverResponses {
		// ValidateServerJSON returns all validation results; using FirstError() to preserve existing behavior
		// In future, consider logging all issues from result.Issues for better diagnostics
		result := validators.ValidateServerJSON(&response, validators.ValidationSchemaVersionAndSemantic)
		if err := result.FirstError(); err != nil {
			// Log warning and track invalid server instead of failing
			invalidServers = append(invalidServers, response.Name)
			validationFailures = append(validationFailures, fmt.Sprintf("Server '%s': %v", response.Name, err))
			log.Printf("Warning: Skipping invalid server '%s': %v", response.Name, err)
			continue
		}

		// Add valid ServerJSON to records
		validRecords = append(validRecords, &response)
	}

	// Print summary of validation results
	if len(invalidServers) > 0 {
		log.Printf("Validation summary: %d servers passed validation, %d invalid servers skipped", len(validRecords), len(invalidServers))
		log.Printf("Invalid servers: %v", invalidServers)
		for _, failure := range validationFailures {
			log.Printf("  - %s", failure)
		}
	} else {
		log.Printf("Validation summary: All %d servers passed validation", len(validRecords))
	}

	return validRecords, nil
}

func fetchFromHTTP(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func fetchFromRegistryAPI(ctx context.Context, baseURL string) ([]*apiv0.ServerJSON, error) {
	var allRecords []*apiv0.ServerJSON
	cursor := ""

	for {
		url := baseURL
		if cursor != "" {
			if strings.Contains(url, "?") {
				url += "&cursor=" + cursor
			} else {
				url += "?cursor=" + cursor
			}
		}

		data, err := fetchFromHTTP(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch page from registry API: %w", err)
		}

		var response struct {
			Servers  []apiv0.ServerResponse `json:"servers"`
			Metadata *struct {
				NextCursor string `json:"nextCursor,omitempty"`
			} `json:"metadata,omitempty"`
		}

		if err := json.Unmarshal(data, &response); err != nil {
			return nil, fmt.Errorf("failed to parse registry API response: %w", err)
		}

		// Extract ServerJSON from each ServerResponse
		for _, serverResponse := range response.Servers {
			allRecords = append(allRecords, &serverResponse.Server)
		}

		// Check if there's a next page
		if response.Metadata == nil || response.Metadata.NextCursor == "" {
			break
		}
		cursor = response.Metadata.NextCursor
	}

	return allRecords, nil
}

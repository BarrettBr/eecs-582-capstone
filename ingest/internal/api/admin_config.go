package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BarrettBr/eecs-582-capstone/internal/config"
	ingestvalidation "github.com/BarrettBr/eecs-582-capstone/internal/ingest/validation"
)

type adminConfigDocument struct {
	SourceConfigPath string                                     `json:"source_config_path"`
	SourceProfile    string                                     `json:"source_profile,omitempty"`
	Catalog          config.SourceCatalog                       `json:"catalog"`
	ValidationSpecs  map[string]ingestvalidation.ValidationSpec `json:"validation_specs"`
}

// description: Returns the current editable source catalog plus referenced validation specs.
// input: HTTP response writer and request for the admin config endpoint.
// output: Writes one structured admin config document as JSON.
func (cfg *apiConfig) adminConfigHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		doc, err := cfg.loadAdminConfigDocument()
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "failed to load admin config", err)
			return
		}
		respondWithJSON(w, http.StatusOK, doc)
	case http.MethodPut:
		var req adminConfigDocument
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "invalid admin config payload", err)
			return
		}
		if decoder.More() {
			respondWithError(w, http.StatusBadRequest, "invalid admin config payload", fmt.Errorf("multiple JSON documents"))
			return
		}
		doc, err := cfg.saveAdminConfigDocument(req)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "failed to save admin config", err)
			return
		}
		respondWithJSON(w, http.StatusOK, doc)
	default:
		respondWithError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

func (cfg *apiConfig) loadAdminConfigDocument() (adminConfigDocument, error) {
	if strings.TrimSpace(cfg.sourceConfigPath) == "" {
		return adminConfigDocument{}, fmt.Errorf("source config path unavailable")
	}

	content, err := os.ReadFile(cfg.sourceConfigPath)
	if err != nil {
		return adminConfigDocument{}, fmt.Errorf("read source config %s: %w", cfg.sourceConfigPath, err)
	}
	catalog, err := config.ParseSourceCatalogWithProfile(content, "")
	if err != nil {
		return adminConfigDocument{}, fmt.Errorf("parse source config %s: %w", cfg.sourceConfigPath, err)
	}

	for i := range catalog.Sources {
		catalog.Sources[i].ValidationFile = normalizeConfigPath(catalog.Sources[i].ValidationFile)
	}

	specPaths := make([]string, 0)
	seen := make(map[string]struct{}, len(catalog.Sources))
	for _, source := range catalog.Sources {
		specPath := normalizeConfigPath(source.ValidationFile)
		if specPath == "" {
			continue
		}
		if _, ok := seen[specPath]; ok {
			continue
		}
		seen[specPath] = struct{}{}
		specPaths = append(specPaths, specPath)
	}
	slices.Sort(specPaths)

	specs := make(map[string]ingestvalidation.ValidationSpec, len(specPaths))
	for _, specPath := range specPaths {
		specContent, err := os.ReadFile(specPath)
		if err != nil {
			return adminConfigDocument{}, fmt.Errorf("read validation spec %s: %w", specPath, err)
		}
		spec, err := ingestvalidation.ParseValidationSpec(specContent)
		if err != nil {
			return adminConfigDocument{}, fmt.Errorf("parse validation spec %s: %w", specPath, err)
		}
		specs[specPath] = spec
	}

	return adminConfigDocument{
		SourceConfigPath: normalizeConfigPath(cfg.sourceConfigPath),
		SourceProfile:    strings.TrimSpace(cfg.sourceProfile),
		Catalog:          *catalog,
		ValidationSpecs:  specs,
	}, nil
}

func (cfg *apiConfig) saveAdminConfigDocument(req adminConfigDocument) (adminConfigDocument, error) {
	if strings.TrimSpace(cfg.sourceConfigPath) == "" {
		return adminConfigDocument{}, fmt.Errorf("source config path unavailable")
	}

	req.Catalog.Sources = append([]config.SourceDefinition(nil), req.Catalog.Sources...)
	for i := range req.Catalog.Sources {
		req.Catalog.Sources[i].ValidationFile = normalizeConfigPath(req.Catalog.Sources[i].ValidationFile)
	}

	normalizedSpecs := make(map[string]ingestvalidation.ValidationSpec, len(req.ValidationSpecs))
	for path, spec := range req.ValidationSpecs {
		normalizedPath := normalizeConfigPath(path)
		if normalizedPath == "" {
			return adminConfigDocument{}, fmt.Errorf("validation spec path is required")
		}
		if err := ingestvalidation.NormalizeValidationSpec(&spec); err != nil {
			return adminConfigDocument{}, fmt.Errorf("validation spec %s: %w", normalizedPath, err)
		}
		normalizedSpecs[normalizedPath] = spec
	}
	req.ValidationSpecs = normalizedSpecs

	for _, source := range req.Catalog.Sources {
		if source.ValidationFile == "" {
			return adminConfigDocument{}, fmt.Errorf("source %q missing validation_file", source.Name)
		}
		if _, ok := req.ValidationSpecs[source.ValidationFile]; !ok {
			return adminConfigDocument{}, fmt.Errorf("source %q references missing validation spec %q", source.Name, source.ValidationFile)
		}
	}

	if err := config.ValidateSourceCatalogWithProfile(&req.Catalog, ""); err != nil {
		return adminConfigDocument{}, err
	}
	if profileName := strings.TrimSpace(cfg.sourceProfile); profileName != "" {
		catalogContent, err := json.Marshal(req.Catalog)
		if err != nil {
			return adminConfigDocument{}, fmt.Errorf("marshal source config for profile validation: %w", err)
		}
		if _, err := config.ParseSourceCatalogWithProfile(catalogContent, profileName); err != nil {
			return adminConfigDocument{}, err
		}
	}

	previousDoc, err := cfg.loadAdminConfigDocument()
	if err != nil {
		return adminConfigDocument{}, err
	}

	for path, spec := range req.ValidationSpecs {
		if err := writeJSONFile(path, spec); err != nil {
			return adminConfigDocument{}, fmt.Errorf("write validation spec %s: %w", path, err)
		}
	}
	if err := writeJSONFile(cfg.sourceConfigPath, req.Catalog); err != nil {
		return adminConfigDocument{}, fmt.Errorf("write source config %s: %w", cfg.sourceConfigPath, err)
	}

	for path := range previousDoc.ValidationSpecs {
		if _, ok := req.ValidationSpecs[path]; ok {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return adminConfigDocument{}, fmt.Errorf("remove validation spec %s: %w", path, err)
		}
	}

	return cfg.loadAdminConfigDocument()
}

func normalizeConfigPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	return filepath.Clean(trimmed)
}

func writeJSONFile(path string, payload any) error {
	normalizedPath := normalizeConfigPath(path)
	if normalizedPath == "" {
		return fmt.Errorf("path is required")
	}
	if err := os.MkdirAll(filepath.Dir(normalizedPath), 0o755); err != nil {
		return err
	}

	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')

	tempFile, err := os.CreateTemp(filepath.Dir(normalizedPath), filepath.Base(normalizedPath)+".*.tmp")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.Write(content); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, normalizedPath)
}

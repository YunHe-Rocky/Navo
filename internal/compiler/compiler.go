package compiler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"navo/internal/fsatomic"
	"navo/internal/winprocess"
)

// ConfigCompiler manages the transformation and deployment pipeline
// from Navo domain models to sing-box configuration files.
type ConfigCompiler interface {
	// Compile generates and validates a sing-box config from the domain model.
	Compile(ctx context.Context, cfg *Config) (*CompileResult, error)

	// Check invokes "sing-box check" on the compiled config.
	Check(ctx context.Context, configPath string) error

	// Apply deploys a compiled config as the active configuration,
	// creating a new revision and archiving the old one.
	Apply(ctx context.Context, result *CompileResult) (*Revision, error)

	// Rollback reverts to a previous revision.
	Rollback(ctx context.Context, toVersion int) (*Revision, error)

	// GetActiveRevision returns the currently active configuration revision.
	GetActiveRevision() *Revision

	// ListRevisions returns all configuration revisions, newest first.
	ListRevisions() []*Revision
}

// CompileResult wraps a compiled configuration.
type CompileResult struct {
	Config     *Config   `json:"config"`
	JSON       []byte    `json:"json"`
	ConfigHash string    `json:"config_hash"`
	CompiledAt time.Time `json:"compiled_at"`
}

// DefaultCompiler implements ConfigCompiler with a local filesystem backend.
type DefaultCompiler struct {
	mu          sync.RWMutex
	binaryPath  string // path to sing-box.exe for check
	configDir   string // directory for storing config files
	revisions   []*Revision
	activeRev   *Revision
	nextVersion int
}

// NewDefaultCompiler creates a new DefaultCompiler.
func NewDefaultCompiler(binaryPath string, configDir string) *DefaultCompiler {
	if configDir == "" {
		configDir = filepath.Join(os.TempDir(), "navo", "configs")
	}
	return &DefaultCompiler{
		binaryPath:  binaryPath,
		configDir:   configDir,
		nextVersion: 1,
	}
}

// Compile validates the domain model, generates sing-box JSON,
// and optionally runs "sing-box check".
func (c *DefaultCompiler) Compile(ctx context.Context, cfg *Config) (*CompileResult, error) {
	// Step 1: Validate
	vr := Validate(cfg)
	if !vr.Valid {
		return nil, fmt.Errorf("config validation failed: %d error(s)", len(vr.Errors))
	}

	// Step 2: Resolve outbound tags
	if err := ResolveOutboundTags(cfg); err != nil {
		return nil, fmt.Errorf("tag resolution failed: %w", err)
	}

	// Step 3: Generate JSON
	jsonData, err := Generate(cfg)
	if err != nil {
		return nil, fmt.Errorf("generation failed: %w", err)
	}

	// Step 4: Compute hash
	hash := sha256.Sum256(jsonData)
	configHash := fmt.Sprintf("%x", hash)[:16]

	return &CompileResult{
		Config:     cfg,
		JSON:       jsonData,
		ConfigHash: configHash,
		CompiledAt: time.Now(),
	}, nil
}

// Check runs "sing-box check" on a compiled config file.
func (c *DefaultCompiler) Check(ctx context.Context, configPath string) error {
	if c.binaryPath == "" {
		return fmt.Errorf("sing-box binary not configured")
	}

	cmd := exec.CommandContext(ctx, c.binaryPath, "check", "-c", configPath)
	winprocess.ConfigureHidden(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sing-box check failed: %s: %w", string(output), err)
	}
	return nil
}

// Apply writes the compiled config to disk and activates it as a revision.
func (c *DefaultCompiler) Apply(ctx context.Context, result *CompileResult) (*Revision, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Ensure config dir exists
	if err := os.MkdirAll(c.configDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create config dir: %w", err)
	}

	version := c.nextVersion
	c.nextVersion++

	// Write config file
	configFilename := fmt.Sprintf("config_v%d.json", version)
	configPath := filepath.Join(c.configDir, configFilename)

	if err := fsatomic.WriteFile(configPath, result.JSON, 0600); err != nil {
		return nil, fmt.Errorf("cannot write config: %w", err)
	}

	// Run sing-box check if binary is available
	if c.binaryPath != "" {
		if err := c.Check(ctx, configPath); err != nil {
			// Config is written but failed check — mark as rejected
			rev := &Revision{
				ID:         fmt.Sprintf("rev-%d", version),
				Version:    version,
				Status:     RevisionRejected,
				ConfigHash: result.ConfigHash,
				ConfigPath: configPath,
				CreatedAt:  time.Now(),
			}
			c.revisions = append(c.revisions, rev)
			return rev, fmt.Errorf("config check failed: %w", err)
		}
	}

	now := time.Now()

	// Mark previous active as rollback
	if c.activeRev != nil {
		c.activeRev.Status = RevisionRollback
	}

	rev := &Revision{
		ID:          fmt.Sprintf("rev-%d", version),
		Version:     version,
		Status:      RevisionActive,
		ConfigHash:  result.ConfigHash,
		ConfigPath:  configPath,
		CreatedAt:   now,
		ActivatedAt: &now,
	}
	c.revisions = append(c.revisions, rev)
	c.activeRev = rev

	return rev, nil
}

// Rollback reverts to a previous revision.
func (c *DefaultCompiler) Rollback(ctx context.Context, toVersion int) (*Revision, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if toVersion < 1 {
		return nil, fmt.Errorf("invalid version: %d", toVersion)
	}

	var target *Revision
	for _, r := range c.revisions {
		if r.Version == toVersion && (r.Status == RevisionActive || r.Status == RevisionRollback) {
			target = r
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("version %d not found or not rollback-able", toVersion)
	}

	// Verify config file still exists
	if _, err := os.Stat(target.ConfigPath); err != nil {
		return nil, fmt.Errorf("config file for version %d no longer exists: %w", toVersion, err)
	}

	// Check that the config is still valid
	if c.binaryPath != "" {
		if err := c.Check(ctx, target.ConfigPath); err != nil {
			return nil, fmt.Errorf("rollback config failed check: %w", err)
		}
	}

	version := c.nextVersion
	c.nextVersion++

	now := time.Now()
	if c.activeRev != nil {
		c.activeRev.Status = RevisionRollback
	}

	rev := &Revision{
		ID:           fmt.Sprintf("rev-%d", version),
		Version:      version,
		Status:       RevisionActive,
		ConfigHash:   target.ConfigHash,
		ConfigPath:   target.ConfigPath,
		CreatedAt:    now,
		ActivatedAt:  &now,
		RollbackFrom: toVersion,
	}
	c.revisions = append(c.revisions, rev)
	c.activeRev = rev

	return rev, nil
}

// GetActiveRevision returns the currently active revision.
func (c *DefaultCompiler) GetActiveRevision() *Revision {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.activeRev
}

// ListRevisions returns all revisions, newest first.
func (c *DefaultCompiler) ListRevisions() []*Revision {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*Revision, len(c.revisions))
	// Copy in reverse order
	for i := len(c.revisions) - 1; i >= 0; i-- {
		result[len(c.revisions)-1-i] = c.revisions[i]
	}
	return result
}

// ── Helper to write a CompileResult to a temp file for testing ──

// WriteTempConfig writes compiled JSON to a temporary file and returns the path.
func WriteTempConfig(result *CompileResult) (string, error) {
	dir := filepath.Join(os.TempDir(), "navo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "config_*.json")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(result.JSON); err != nil {
		f.Close()
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

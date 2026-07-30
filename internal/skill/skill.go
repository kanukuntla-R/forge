// Package skill installs forge's Claude Code skill file to the user's
// ~/.claude/skills/ directory.
package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	forge "github.com/kanukuntla-r/forge"
)

// InstallResult describes what happened during install.
type InstallResult struct {
	InstalledPath string
	BackupPath    string // empty if no backup was needed
	Created       bool   // true if the skills directory was created
}

// Install copies the embedded skill file to the user's Claude Code skills
// directory (~/.claude/skills/forge.skill.md). If a file already exists at
// the destination, it is backed up with a timestamp suffix. If the skills
// directory doesn't exist, it is created.
func Install() (*InstallResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolving home directory: %w", err)
	}

	skillsDir := filepath.Join(home, ".claude", "skills")
	targetPath := filepath.Join(skillsDir, "forge.skill.md")

	result := &InstallResult{InstalledPath: targetPath}

	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(skillsDir, 0755); err != nil {
			return nil, fmt.Errorf("creating %s: %w", skillsDir, err)
		}
		result.Created = true
	}

	if _, err := os.Stat(targetPath); err == nil {
		backupPath := targetPath + ".backup-" + time.Now().Format("2006-01-02-150405")
		if err := os.Rename(targetPath, backupPath); err != nil {
			return nil, fmt.Errorf("backing up existing skill file: %w", err)
		}
		result.BackupPath = backupPath
	}

	if err := os.WriteFile(targetPath, forge.SkillContent(), 0644); err != nil {
		return nil, fmt.Errorf("writing %s: %w", targetPath, err)
	}

	return result, nil
}

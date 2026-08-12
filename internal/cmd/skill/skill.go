// Package skill provides skill management subcommands.
package skill

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var skillsDir = ".lana/skills"

// SkillManifest represents a skill's manifest file.
type SkillManifest struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	Path        string   `json:"path"`
	Enabled     bool     `json:"enabled"`
	Triggers    []string `json:"triggers,omitempty"`
}

// NewCommand creates the skill command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Skill management",
	}
	cmd.AddCommand(skillListCommand())
	cmd.AddCommand(skillInstallCommand())
	cmd.AddCommand(skillEnableCommand())
	cmd.AddCommand(skillDisableCommand())
	cmd.AddCommand(skillRemoveCommand())
	cmd.AddCommand(skillInfoCommand())
	return cmd
}

func skillListCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			skills, err := loadSkills()
			if err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "No skills installed.")
				return nil
			}

			if len(skills) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No skills installed.")
				fmt.Fprintln(cmd.OutOrStdout(), "To install: lana skill install <local-path>")
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(skills, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Installed skills (%d):\n\n", len(skills))
			for _, s := range skills {
				status := "on"
				if !s.Enabled {
					status = "off"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s v%s\n", status, s.Name, s.Version)
				if s.Description != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "      %s\n", s.Description)
				}
				if len(s.Triggers) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "      Triggers: %s\n", strings.Join(s.Triggers, ", "))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "      Path: %s\n\n", s.Path)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	return cmd
}

func skillInstallCommand() *cobra.Command {
	var pName, pVersion string

	cmd := &cobra.Command{
		Use:   "install <source>",
		Short: "Install a skill from a local directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]
			absSource, err := filepath.Abs(source)
			if err != nil {
				return fmt.Errorf("resolve source: %w", err)
			}

			info, err := os.Stat(absSource)
			if err != nil {
				return fmt.Errorf("stat source: %w", err)
			}
			if !info.IsDir() {
				return fmt.Errorf("source must be a directory: %s", source)
			}

			if pName == "" {
				pName = filepath.Base(absSource)
			}
			if pVersion == "" {
				pVersion = "0.0.1"
			}

			manifest := SkillManifest{
				Name:        pName,
				Version:     pVersion,
				Description: "Auto-detected skill",
				Path:        absSource,
				Enabled:     true,
			}
			manifestData, _ := json.MarshalIndent(manifest, "", "  ")

			destDir := filepath.Join(skillsDir, manifest.Name)
			if err := os.MkdirAll(destDir, 0755); err != nil {
				return fmt.Errorf("create skill directory: %w", err)
			}
			if err := os.WriteFile(filepath.Join(destDir, "skill.json"), manifestData, 0644); err != nil {
				return fmt.Errorf("save manifest: %w", err)
			}

			if err := copyDir(absSource, destDir); err != nil {
				return fmt.Errorf("copy skill files: %w", err)
			}

			skills, _ := loadSkills()
			newSkills := make([]SkillManifest, 0)
			for _, s := range skills {
				if s.Name != manifest.Name {
					newSkills = append(newSkills, s)
				}
			}
			manifest.Path = destDir
			newSkills = append(newSkills, manifest)
			if err := saveSkills(newSkills); err != nil {
				return fmt.Errorf("save skills: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Skill installed: %s v%s\n", manifest.Name, manifest.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "  Path: %s\n", destDir)
			return nil
		},
	}

	cmd.Flags().StringVarP(&pName, "name", "n", "", "Skill name")
	cmd.Flags().StringVarP(&pVersion, "version", "v", "0.0.1", "Skill version")
	return cmd
}

func skillEnableCommand() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name = args[0]
			return toggleSkill(cmd.OutOrStdout(), name, true)
		},
	}
	return cmd
}

func skillDisableCommand() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name = args[0]
			return toggleSkill(cmd.OutOrStdout(), name, false)
		},
	}
	return cmd
}

func skillRemoveCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			skills, err := loadSkills()
			if err != nil {
				return err
			}

			found := false
			var newSkills []SkillManifest
			for _, s := range skills {
				if s.Name == name {
					found = true
					if !force {
						return fmt.Errorf("skill %s found. Use --force to remove", name)
					}
				} else {
					newSkills = append(newSkills, s)
				}
			}

			if !found {
				return fmt.Errorf("skill not found: %s", name)
			}

			os.RemoveAll(filepath.Join(skillsDir, name))
			if err := saveSkills(newSkills); err != nil {
				return fmt.Errorf("save skills: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Skill removed: %s\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}

func skillInfoCommand() *cobra.Command {
	var name string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "info <name>",
		Short: "Show skill information",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name = args[0]

			skills, err := loadSkills()
			if err != nil {
				return err
			}

			for _, s := range skills {
				if s.Name == name {
					if jsonOutput {
						data, _ := json.MarshalIndent(s, "", "  ")
						fmt.Fprintln(cmd.OutOrStdout(), string(data))
						return nil
					}

					fmt.Fprintf(cmd.OutOrStdout(), "Name:        %s\n", s.Name)
					fmt.Fprintf(cmd.OutOrStdout(), "Version:     %s\n", s.Version)
					fmt.Fprintf(cmd.OutOrStdout(), "Enabled:     %v\n", s.Enabled)
					fmt.Fprintf(cmd.OutOrStdout(), "Path:        %s\n", s.Path)
					if s.Description != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "Description: %s\n", s.Description)
					}
					if len(s.Triggers) > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "Triggers:    %s\n", strings.Join(s.Triggers, ", "))
					}
					return nil
				}
			}
			return fmt.Errorf("skill not found: %s", name)
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output in JSON format")
	return cmd
}

func loadSkills() ([]SkillManifest, error) {
	skillsPath := filepath.Join(skillsDir, "skills.json")
	data, err := os.ReadFile(skillsPath)
	if err != nil {
		return nil, fmt.Errorf("no skills found")
	}

	var skills []SkillManifest
	if err := json.Unmarshal(data, &skills); err != nil {
		return nil, fmt.Errorf("parse skills: %w", err)
	}
	return skills, nil
}

func saveSkills(skills []SkillManifest) error {
	data, err := json.MarshalIndent(skills, "", "  ")
	if err != nil {
		return err
	}

	skillsDirAbs := skillsDir
	if err := os.MkdirAll(skillsDirAbs, 0755); err != nil {
		return fmt.Errorf("create skills directory: %w", err)
	}

	return os.WriteFile(filepath.Join(skillsDirAbs, "skills.json"), data, 0644)
}

func toggleSkill(w io.Writer, name string, enabled bool) error {
	skills, err := loadSkills()
	if err != nil {
		return err
	}

	found := false
	for i, s := range skills {
		if s.Name == name {
			skills[i].Enabled = enabled
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("skill not found: %s", name)
	}

	if err := saveSkills(skills); err != nil {
		return fmt.Errorf("save skills: %w", err)
	}

	action := "disabled"
	if enabled {
		action = "enabled"
	}
	fmt.Fprintf(w, "Skill %s %s\n", name, action)
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) || strings.HasPrefix(rel, ".git/") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dstPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

// Package knowledge provides queryable local knowledge-store subcommands.
package knowledge

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/deagy/lana/internal/app"
	store "github.com/deagy/lana/internal/knowledge"
)

// NewCommand creates the local-only knowledge command group. When wired below
// Lana's root command it uses knowledge_store.path from resolved configuration;
// direct embedders can use --store (or retain the legacy relative default).
func NewCommand() *cobra.Command {
	var storePath string
	cmd := &cobra.Command{Use: "knowledge", Short: "Manage a local knowledge store"}
	cmd.PersistentFlags().StringVar(&storePath, "store", "", "Local knowledge store directory")
	cmd.AddCommand(ingestCommand(&storePath))
	cmd.AddCommand(searchCommand(&storePath))
	cmd.AddCommand(listCommand(&storePath))
	cmd.AddCommand(sourcesCommand(&storePath))
	cmd.AddCommand(removeCommand(&storePath))
	return cmd
}

func open(cmd *cobra.Command, override string) (*store.Store, error) {
	if override == "" {
		if application, ok := app.FromContext(cmd.Context()); ok {
			if configured := application.Config().Config().KnowledgeStore; configured != nil {
				override = configured.Path
			}
		}
	}
	if override == "" {
		// Preserve direct command/fixture behavior for callers not using root.
		override = "knowledge-store"
	}
	return store.New(store.Options{Dir: override})
}

func ingestCommand(storePath *string) *cobra.Command {
	var source string
	var tags []string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "ingest <path>",
		Short: "Register and ingest local text files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			knowledgeStore, err := open(cmd, *storePath)
			if err != nil {
				return err
			}
			result, err := knowledgeStore.Ingest(args[0], source, tags)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(cmd, result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Ingested source %s: %d added, %d updated, %d unchanged, %d removed, %d skipped\n", terminalText(result.Source), result.Added, result.Updated, result.Unchanged, result.Removed, result.Skipped)
			return nil
		},
	}
	cmd.Flags().StringVarP(&source, "source", "s", "", "Stable local source name (defaults to canonical path)")
	cmd.Flags().StringArrayVarP(&tags, "tag", "g", nil, "Tag to attach (repeatable)")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output JSON")
	return cmd
}

func searchCommand(storePath *string) *cobra.Command {
	var top int
	var source string
	var tags []string
	var mode string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search locally indexed content with citations",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			knowledgeStore, err := open(cmd, *storePath)
			if err != nil {
				return err
			}
			searchMode := store.SearchModeTokens
			switch mode {
			case "semantic":
				searchMode = store.SearchModeSemantic
			case "hybrid":
				searchMode = store.SearchModeHybrid
			case "tokens", "":
				searchMode = store.SearchModeTokens
			default:
				return fmt.Errorf("invalid search mode: %s (valid: tokens, semantic, hybrid)", mode)
			}
			results, err := knowledgeStore.Search(args[0], store.SearchOptions{Top: top, Source: source, Tags: tags, Mode: searchMode})
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(cmd, struct {
					Query   string         `json:"query"`
					Count   int            `json:"count"`
					Results []store.Result `json:"results"`
				}{args[0], len(results), results})
			}
			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No results found for %s\n", terminalText(args[0]))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Search results for %s (%d found):\n\n", terminalText(args[0]), len(results))
			for _, result := range results {
				semanticStr := ""
				if result.SemanticScore > 0 {
					semanticStr = fmt.Sprintf(" semantic=%.3f", result.SemanticScore)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] score=%d%s\n  citation: source=%s path=%s document=%s hash=%s\n  tags: %s\n  content: %s\n\n",
					terminalText(result.Citation.ChunkID),
					result.Score,
					semanticStr,
					terminalText(result.Citation.Source),
					terminalText(result.Citation.Path),
					terminalText(result.Citation.DocumentID),
					terminalText(result.Citation.ContentHash),
					terminalText(strings.Join(result.Tags, ",")),
					terminalText(strings.TrimSpace(result.Content)),
				)
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&top, "top", "t", 10, "Maximum results (1-100)")
	cmd.Flags().StringVarP(&source, "source", "s", "", "Source filter")
	cmd.Flags().StringArrayVarP(&tags, "tag", "g", nil, "Required tag (repeatable)")
	cmd.Flags().StringVarP(&mode, "mode", "m", "tokens", "Search mode: tokens, semantic, hybrid")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output JSON")
	return cmd
}

func listCommand(storePath *string) *cobra.Command {
	var source string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use: "list", Short: "List indexed documents",
		RunE: func(cmd *cobra.Command, args []string) error {
			knowledgeStore, err := open(cmd, *storePath)
			if err != nil {
				return err
			}
			documents, err := knowledgeStore.ListDocuments(source)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(cmd, documents)
			}
			if len(documents) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No indexed documents.")
				return nil
			}
			for _, document := range documents {
				fmt.Fprintf(cmd.OutOrStdout(), "%s source=%s path=%s hash=%s tags=%s\n", terminalText(document.ID), terminalText(document.Source), terminalText(document.Path), terminalText(document.ContentHash), terminalText(strings.Join(document.Tags, ",")))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&source, "source", "s", "", "Source filter")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output JSON")
	return cmd
}

func sourcesCommand(storePath *string) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use: "sources", Short: "List registered local sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			knowledgeStore, err := open(cmd, *storePath)
			if err != nil {
				return err
			}
			sources, err := knowledgeStore.ListSources()
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(cmd, sources)
			}
			if len(sources) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No registered sources.")
				return nil
			}
			for _, source := range sources {
				fmt.Fprintf(cmd.OutOrStdout(), "%s kind=%s root=%s updated=%s\n", terminalText(source.ID), terminalText(source.Kind), terminalText(source.Root), source.Updated.Format("2006-01-02T15:04:05Z07:00"))
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output JSON")
	return cmd
}

func removeCommand(storePath *string) *cobra.Command {
	var source string
	var force bool
	cmd := &cobra.Command{
		Use: "remove <document-id>", Short: "Remove one document or an entire source",
		Args: func(cmd *cobra.Command, args []string) error {
			if source != "" && len(args) != 0 {
				return fmt.Errorf("document id and --source cannot be used together")
			}
			if source == "" && len(args) != 1 {
				return fmt.Errorf("provide a document id or --source")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return fmt.Errorf("removal requires --force")
			}
			knowledgeStore, err := open(cmd, *storePath)
			if err != nil {
				return err
			}
			if source != "" {
				removed, err := knowledgeStore.RemoveSource(source)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Removed source %s and %d documents\n", terminalText(source), removed)
				return nil
			}
			if err := knowledgeStore.RemoveDocument(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed document %s\n", terminalText(args[0]))
			return nil
		},
	}
	cmd.Flags().StringVarP(&source, "source", "s", "", "Remove this source and all its documents")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Confirm removal")
	return cmd
}

func printJSON(cmd *cobra.Command, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode JSON output: %w", err)
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}

// terminalText is used only for human-readable output. JSON retains exact
// stored values for machine consumers, while this form renders every control
// or format character visibly so indexed content cannot execute terminal
// escape sequences or disguise text direction.
func terminalText(value string) string {
	var output strings.Builder
	for _, r := range value {
		switch {
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&output, `\x%02X`, r)
		case unicode.IsControl(r) || unicode.In(r, unicode.Cf):
			fmt.Fprintf(&output, `\u%04X`, r)
		default:
			output.WriteRune(r)
		}
	}
	return output.String()
}

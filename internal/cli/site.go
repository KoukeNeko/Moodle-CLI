package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// siteSummary is one row of `site list`.
type siteSummary struct {
	Name     string   `json:"name"`
	ID       string   `json:"id"`
	BaseURL  string   `json:"base_url"`
	Backend  string   `json:"backend"`
	Current  bool     `json:"current"`
	Accounts []string `json:"accounts"`
}

func newSiteCommand(r *Renderer, deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "site",
		Short: "Manage the Moodle sites this tool knows about",
	}
	cmd.AddCommand(
		newSiteAddCommand(r, deps),
		newSiteListCommand(r, deps),
		newSiteUseCommand(r, deps),
		newSiteRemoveCommand(r, deps),
		newSiteAcademicCommand(r, deps),
	)
	return cmd
}

func newSiteAcademicCommand(r *Renderer, deps Deps) *cobra.Command {
	cmd := &cobra.Command{Use: "academic", Short: "Configure institution-specific workload fields"}
	cmd.AddCommand(newSiteAcademicConfigureCommand(r, deps))
	return cmd
}

func newSiteAcademicConfigureCommand(r *Renderer, deps Deps) *cobra.Command {
	var siteFlag, creditsField, levelField, termField string
	var undergraduate, graduate float64
	var yes, dryRun bool
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Map course custom fields to credits, academic level and term",
		Args:  cobra.NoArgs,
		Annotations: map[string]string{
			annotationKind: "site.academic.configure", annotationMutates: "true",
			// It rewrites the local configuration, nothing on Moodle.
			annotationSafety: safetyLocal,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			resolved, err := file.Resolve(siteFlag, "")
			if err != nil {
				return err
			}
			academic := config.Academic{
				CreditsField: creditsField, LevelField: levelField, TermField: termField,
				Minimum: config.AcademicMinimum{Undergraduate: undergraduate, Graduate: graduate},
			}
			if err := academic.Validate(); err != nil {
				return err
			}
			planned := dryRun
			if !dryRun {
				if !yes {
					return errs.New(errs.CodeUsage, "changing academic workload settings requires confirmation").
						WithHint("inspect it with --dry-run, then pass --yes")
				}
				resolved.Site.Academic = academic
				if err := file.Save(); err != nil {
					return err
				}
			}
			payload := v1.SiteAcademicConfiguration{
				Site: resolved.SiteName, CreditsField: academic.CreditsField,
				LevelField: academic.LevelField, TermField: academic.TermField,
				UndergraduateMinimum: academic.Minimum.Undergraduate,
				GraduateMinimum:      academic.Minimum.Graduate, DryRun: planned,
			}
			return r.Render(Result{
				Envelope: v1.NewEnvelope("site.academic.configure", payload, v1.NewMeta(v1.SourceLocal)),
				Human: func(w io.Writer) error {
					prefix := "Configured"
					if planned {
						prefix = "Would configure"
					}
					_, err := fmt.Fprintf(w, "%s academic workload fields for %s: credits=%s, level=%s, term=%s; minimum undergraduate=%.2f, graduate=%.2f.\n",
						prefix, resolved.SiteName, academic.CreditsField, academic.LevelField,
						academic.TermField, academic.Minimum.Undergraduate, academic.Minimum.Graduate)
					return err
				},
			})
		},
	}
	cmd.Flags().StringVar(&siteFlag, "site", "", "site to configure")
	cmd.Flags().StringVar(&creditsField, "credits-field", "credits", "course custom-field shortname containing credits")
	cmd.Flags().StringVar(&levelField, "level-field", "academic_level", "course custom-field shortname containing undergraduate or graduate")
	cmd.Flags().StringVar(&termField, "term-field", "academic_term", "course custom-field shortname containing the academic term")
	cmd.Flags().Float64Var(&undergraduate, "undergraduate-minimum", 21, "minimum undergraduate credits per term")
	cmd.Flags().Float64Var(&graduate, "graduate-minimum", 6, "minimum graduate credits per term")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show the configuration without saving it")
	cmd.Flags().BoolVar(&yes, "yes", false, "save the configuration without another prompt")
	return cmd
}

func newSiteAddCommand(r *Renderer, deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:         "add <name> <url>",
		Short:       "Register a Moodle site",
		Args:        cobra.ExactArgs(2),
		Annotations: map[string]string{annotationKind: "site.add", annotationSafety: safetyLocal},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			entry, err := file.AddSite(args[0], args[1])
			if err != nil {
				return err
			}
			if err := file.Save(); err != nil {
				return err
			}
			summary := siteSummary{
				Name: args[0], ID: string(entry.ID), BaseURL: entry.BaseURL,
				Backend: entry.Backend, Current: file.Current.Site == args[0],
				Accounts: []string{},
			}
			return r.Render(Result{
				Envelope: v1.NewEnvelope("site.add", summary, v1.NewMeta(v1.SourceLocal)),
				Human: humanLine("Added site %q (%s).\nSign in with `moodle auth login`.",
					args[0], entry.BaseURL),
			})
		},
	}
}

func newSiteListCommand(r *Renderer, deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List the configured sites",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "site.list"},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			summaries := []siteSummary{}
			for _, name := range file.SiteNames() {
				entry := file.Sites[name]
				summaries = append(summaries, siteSummary{
					Name: name, ID: string(entry.ID), BaseURL: entry.BaseURL,
					Backend: entry.Backend, Current: file.Current.Site == name,
					Accounts: entry.AccountNames(),
				})
			}
			return r.Render(Result{
				Envelope: v1.NewEnvelope("site.list", summaries, v1.NewMeta(v1.SourceLocal)),
				Human:    func(w io.Writer) error { return writeSiteTable(w, summaries) },
			})
		},
	}
}

func writeSiteTable(w io.Writer, summaries []siteSummary) error {
	if len(summaries) == 0 {
		_, err := fmt.Fprintln(w, "No sites configured. Add one with `moodle site add <name> <url>`.")
		return err
	}
	table := newTable(w)
	fmt.Fprintln(table, "  NAME\tURL\tACCOUNTS")
	for _, summary := range summaries {
		marker := " "
		if summary.Current {
			marker = "*"
		}
		accounts := "-"
		if len(summary.Accounts) > 0 {
			accounts = join(summary.Accounts)
		}
		fmt.Fprintf(table, "%s %s\t%s\t%s\n", marker, summary.Name, summary.BaseURL, accounts)
	}
	return table.Flush()
}

func newSiteUseCommand(r *Renderer, deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:         "use <name>",
		Short:       "Make a site the default for later commands",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationKind: "site.use", annotationSafety: safetyLocal},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			if err := file.UseSite(args[0]); err != nil {
				return err
			}
			if err := file.Save(); err != nil {
				return err
			}
			return r.Render(Result{
				Envelope: v1.NewEnvelope("site.use",
					map[string]string{"site": args[0], "account": file.Current.Account},
					v1.NewMeta(v1.SourceLocal)),
				Human: humanLine("Now using site %q.", args[0]),
			})
		},
	}
}

func newSiteRemoveCommand(r *Renderer, deps Deps) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Forget a site and delete its stored credentials",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			annotationKind: "site.remove", annotationSafety: safetyLocal,
			// Local only: it deletes nothing on the Moodle server.
			annotationMutates: "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			entry, ok := file.Sites[args[0]]
			if !ok {
				return errs.New(errs.CodeNotFound, fmt.Sprintf("no site named %q", args[0])).
					WithHint(fmt.Sprintf("configured sites: %v", file.SiteNames()))
			}
			if !yes && len(entry.Accounts) > 0 {
				return errs.New(errs.CodeUsage, fmt.Sprintf(
					"site %q has %d signed-in account(s)", args[0], len(entry.Accounts))).
					WithHint("re-run with --yes to remove the site and its stored credentials")
			}

			siteID := entry.ID
			accounts, err := file.RemoveSite(args[0])
			if err != nil {
				return err
			}
			if err := file.Save(); err != nil {
				return err
			}
			// Only after the configuration is safely written: a credential
			// left behind is recoverable, a lost configuration is not.
			for _, account := range accounts {
				if err := deps.Auth.Forget(siteID, account.ID); err != nil {
					return err
				}
			}
			return r.Render(Result{
				Envelope: v1.NewEnvelope("site.remove",
					map[string]any{"site": args[0], "accounts_removed": len(accounts)},
					v1.NewMeta(v1.SourceLocal)),
				Human: humanLine("Removed site %q and %d account(s).", args[0], len(accounts)),
			})
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "remove without confirmation")
	return cmd
}

func join(items []string) string {
	out := ""
	for i, item := range items {
		if i > 0 {
			out += ", "
		}
		out += item
	}
	return out
}

package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

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
	)
	return cmd
}

func newSiteAddCommand(r *Renderer, deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:         "add <name> <url>",
		Short:       "Register a Moodle site",
		Args:        cobra.ExactArgs(2),
		Annotations: map[string]string{annotationKind: "site.add"},
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
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
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
		Annotations: map[string]string{annotationKind: "site.use"},
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
			annotationKind: "site.remove",
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

package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/doctor"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

func newDoctorCommand(r *Renderer, deps Deps) *cobra.Command {
	var siteFlag, accountFlag string
	cmd := &cobra.Command{
		Use:         "doctor",
		Short:       "Diagnose a site: what works, what does not, and why",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "doctor"},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			resolved, err := file.Resolve(siteFlag, accountFlag)
			if err != nil {
				return err
			}
			target, err := targetSite(resolved.SiteName, resolved.Site)
			if err != nil {
				return err
			}
			input := doctor.Input{
				SiteName:    resolved.SiteName,
				SiteURL:     resolved.Site.BaseURL,
				AccountName: resolved.AccountName,
				Probe:       deps.Auth.Probe(target),
			}
			// No credential yet is a normal state, not a failure: doctor still
			// reports what the site itself offers. A token in the environment
			// counts, even with nothing stored.
			if sessionAccount, token, sessionErr := resolveSession(deps, file, siteFlag, accountFlag); sessionErr == nil {
				input.AccountName = sessionAccount.AccountName
				input.Session = deps.Auth.OpenWithToken(target, sessionAccount.Account.ID, token)
			}

			report := doctor.Run(cmd.Context(), input)

			if err := r.Render(Result{
				Envelope: v1.DoctorReport(report),
				Human:    func(w io.Writer) error { return writeReport(w, report) },
			}); err != nil {
				return err
			}
			if !report.OK() {
				// A failed diagnosis is a real failure: scripts should see a
				// non-zero exit. The report is the output, so no second
				// document may follow it.
				return Reported(v1.ExitUnavailable)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&siteFlag, "site", "", "site to diagnose")
	cmd.Flags().StringVar(&accountFlag, "account", "", "account to diagnose")
	return cmd
}

func writeReport(w io.Writer, report doctor.Report) error {
	fmt.Fprintf(w, "%s  %s\n\n", report.SiteName, report.SiteURL)
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, check := range report.Checks {
		fmt.Fprintf(table, "%s  %s\t%s\n", marker(check.Status), check.Name, check.Detail)
	}
	if err := table.Flush(); err != nil {
		return err
	}
	// Hints go after the table so the columns stay readable.
	for _, check := range report.Checks {
		if check.Hint != "" && check.Status != doctor.StatusOK {
			fmt.Fprintf(w, "\n%s: %s", check.Name, check.Hint)
		}
	}
	fmt.Fprintln(w)
	return nil
}

func marker(status doctor.Status) string {
	switch status {
	case doctor.StatusOK:
		return "OK  "
	case doctor.StatusWarning:
		return "WARN"
	case doctor.StatusFailed:
		return "FAIL"
	default:
		return "SKIP"
	}
}

func newSiteInspectCommand(r *Renderer, deps Deps) *cobra.Command {
	var siteFlag, accountFlag string
	var listFunctions bool
	cmd := &cobra.Command{
		Use:         "inspect",
		Short:       "Show what this account can do on this site",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "site.inspect"},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			resolved, token, err := resolveSession(deps, file, siteFlag, accountFlag)
			if err != nil {
				return err
			}
			target, err := targetSite(resolved.SiteName, resolved.Site)
			if err != nil {
				return err
			}
			capabilities, err := deps.Auth.
				OpenWithToken(target, resolved.Account.ID, token).
				Capabilities(cmd.Context())
			if err != nil {
				return err
			}

			envelope := v1.SiteInspect(resolved.SiteName, resolved.Site.BaseURL,
				capabilities, listFunctions)
			payload, _ := envelope.Data.(v1.SiteCapabilities)

			return r.Render(Result{
				Envelope: envelope,
				Human:    func(w io.Writer) error { return writeInspect(w, payload, capabilities) },
			})
		},
	}
	cmd.Flags().StringVar(&siteFlag, "site", "", "site to inspect")
	cmd.Flags().StringVar(&accountFlag, "account", "", "account to inspect")
	cmd.Flags().BoolVar(&listFunctions, "functions", false, "include the full function list")
	return cmd
}

func writeInspect(w io.Writer, payload v1.SiteCapabilities, capabilities *site.Capabilities) error {
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(table, "Site\t%s\n", payload.SiteName)
	fmt.Fprintf(table, "URL\t%s\n", payload.SiteURL)
	fmt.Fprintf(table, "Moodle\t%s\n", payload.Release)
	fmt.Fprintf(table, "Account\t%s (%s)\n", capabilities.FullName, payload.Username)
	// Not "this site has 429 functions": core_webservice_get_site_info answers
	// for the external service the token belongs to. Measured by dropping one
	// function from moodle_mobile_app while the site kept it installed.
	fmt.Fprintf(table, "Functions offered to this account\t%d\n", payload.FunctionCount)
	fmt.Fprintf(table, "Upload\t%s\n", yesNo(payload.CanUpload))
	fmt.Fprintf(table, "Download\t%s\n", yesNo(payload.CanDownload))
	if err := table.Flush(); err != nil {
		return err
	}
	for _, name := range payload.Functions {
		fmt.Fprintln(w, name)
	}
	return nil
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

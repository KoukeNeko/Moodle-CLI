package cli

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/auth"
	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// newSetupCommand adds a site and signs in, in one pass.
//
// Doing it in two commands is correct and unhelpful: `site add` succeeds
// whatever the site turns out to be, and `auth login` then picks a method
// automatically, which quietly skips everything that needs the user to paste
// something — on an SSO site that is every method that works. The result was a
// student reading "no login method can run without more information" as their
// introduction to the tool.
//
// This asks the site first, then says which methods it actually offers, which
// one is likely to work here and why, and lets the person choose.
func newSetupCommand(r *Renderer, deps Deps) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "setup <url>",
		Short: "Add a Moodle site and sign in, choosing from the methods it offers",
		Long: "Asks the site what it supports, explains which method suits it, and runs\n" +
			"the one you choose. Needs a terminal: with --json or --no-input, use\n" +
			"`moodle site add` and `moodle auth login --method …` instead.",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{annotationKind: "auth.login", annotationMutates: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if r.Format == FormatJSON {
				return errs.New(errs.CodeUsage, "setup is interactive, so it has no JSON form").
					WithHint("use `moodle site add` then `moodle auth login --method …`")
			}
			if r.NoInput || deps.Interactive == nil || !deps.Interactive() {
				return errs.New(errs.CodeUsage, "setup needs a terminal to ask which method to use").
					WithHint("use `moodle site add` then `moodle auth login --method …`")
			}

			base, err := site.ParseBaseURL(args[0])
			if err != nil {
				return err
			}
			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			siteName := strings.TrimSpace(name)
			if siteName == "" {
				siteName = suggestSiteName(base.Host, file)
			}

			// Registered before signing in: the methods need a site to probe,
			// and a site that is already there is reused rather than refused.
			if _, exists := file.Sites[siteName]; !exists {
				if _, err := file.AddSite(siteName, args[0]); err != nil {
					return err
				}
				if err := file.Save(); err != nil {
					return err
				}
			}
			resolved, err := file.Resolve(siteName, "")
			if err != nil {
				return err
			}
			target, err := targetSite(resolved.SiteName, resolved.Site)
			if err != nil {
				return err
			}

			out := r.Streams.Err
			fmt.Fprintf(out, "Asking %s what it supports…\n", base)
			publicConfig, probeErr := deps.Auth.Probe(target).PublicConfig(cmd.Context())
			if publicConfig != nil && publicConfig.SiteName != "" {
				fmt.Fprintf(out, "\n%s\n", publicConfig.SiteName)
			}
			if probeErr != nil {
				// Not fatal: a site that cannot be asked reports every method
				// as "unknown", and an explicit choice still runs.
				fmt.Fprintf(out, "\nThe site did not answer the question every client asks first,\n"+
					"so what follows is what this build can offer rather than what it confirmed:\n  %s\n",
					errs.From(probeErr).Error())
			}
			describeSite(out, publicConfig)

			candidates := deps.Login.Candidates(cmd.Context(), target, publicConfig)
			choice, err := askForMethod(r, out, candidates, publicConfig)
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "\n")
			credential, err := deps.Login.Authenticate(cmd.Context(), auth.Request{
				Site: target,
				In:   cmd.InOrStdin(),
				// Prompts are diagnostics: stdout carries the result only.
				Out: out,
			}, choice)
			if err != nil {
				return err
			}
			return storeLogin(cmd, r, deps, file, resolved, target, credential, "")
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "name for this site (default: from its address)")
	return cmd
}

// describeSite says what the site's own configuration implies, in the terms
// that decide which method can work.
func describeSite(w io.Writer, config *auth.PublicConfig) {
	if config == nil {
		return
	}
	if config.EnableMobileWebService != 1 {
		fmt.Fprintln(w, "\nThis site has mobile web services switched off, so it issues no token.\n"+
			"A browser session can still read, but nothing can hand work in.")
		return
	}
	if config.HasIdentityProviders {
		// The providers' names are not carried this far; what matters for
		// choosing a method is that there is one at all.
		fmt.Fprintln(w, "\nThis site signs in through an external identity provider, so it\n"+
			"probably has no Moodle password of its own for your account.")
	}
}

// recommendation is the method to try first here, and why. It reads the site's
// own configuration rather than ranking the methods in the abstract: what is
// easiest depends on whether the site does its own passwords, and on whether
// this machine can receive a callback.
func recommendation(candidates []auth.Candidate, config *auth.PublicConfig) (string, string) {
	available := map[string]bool{}
	for _, candidate := range candidates {
		if candidate.Probe.Availability == auth.Available {
			available[candidate.Method.Name()] = true
		}
	}
	sso := config != nil && config.HasIdentityProviders

	switch {
	case available["mobilelaunch"]:
		return "mobilelaunch", "your browser signs in and hands the result back on its own"
	case available["browser-session"] && sso:
		// One paste, and it ends in a token that outlives the session it came
		// from — which is what makes it better than import-session here.
		return "browser-session", "you are signed in through " +
			"single sign-on already, and this trades that session for a token that does not expire with it"
	case available["qr"] && sso:
		return "qr", "it works with single sign-on, and the code is one paste"
	case available["password"] && !sso:
		return "password", "this site keeps its own passwords"
	case available["browser-session"]:
		return "browser-session", "it reuses the browser you are already signed in to"
	case available["qr"]:
		return "qr", "the code is one paste and needs no password"
	case available["token"]:
		return "token", "you already have a token to give it"
	case available["manual"]:
		return "manual", "it is the one that works without anything else in place"
	}
	return "", ""
}

// askForMethod prints the methods and reads a choice.
func askForMethod(r *Renderer, out io.Writer, candidates []auth.Candidate, config *auth.PublicConfig) (string, error) {
	suggested, why := recommendation(candidates, config)

	fmt.Fprintln(out, "\nHow would you like to sign in?")
	table := newTable(out)
	numbered := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		name := candidate.Method.Name()
		numbered = append(numbered, name)
		marker := " "
		if name == suggested {
			marker = "→"
		}
		status := string(candidate.Probe.Availability)
		if candidate.Probe.Reason != "" {
			status += " (" + candidate.Probe.Reason + ")"
		}
		fmt.Fprintf(table, "%s %d\t%s\t%s\t%s\n",
			marker, len(numbered), name, status, candidate.Method.Describe())
	}
	if err := table.Flush(); err != nil {
		return "", err
	}
	// Every method is listed, including the ones that cannot run: a person
	// deciding needs to see that a method exists and why it is out, not a
	// filtered list that looks like the whole set.
	if suggested != "" {
		fmt.Fprintf(out, "\nSuggested: %s — %s.\n", suggested, why)
	}
	fmt.Fprintf(out, "Choose a number or a name")
	if suggested != "" {
		fmt.Fprintf(out, " [%s]", suggested)
	}
	fmt.Fprint(out, ": ")

	line, err := bufio.NewReader(r.Streams.In).ReadString('\n')
	answer := strings.TrimSpace(line)
	if err != nil && answer == "" {
		return "", errs.Wrap(errs.CodeUsage, err, "cannot read the choice")
	}
	if answer == "" {
		if suggested == "" {
			return "", errs.New(errs.CodeUsage, "no method chosen")
		}
		return suggested, nil
	}
	if index, convErr := strconv.Atoi(answer); convErr == nil {
		if index < 1 || index > len(numbered) {
			return "", errs.New(errs.CodeUsage,
				fmt.Sprintf("there is no method %d", index)).
				WithHint("choose between 1 and " + strconv.Itoa(len(numbered)))
		}
		return numbered[index-1], nil
	}
	for _, name := range numbered {
		if strings.EqualFold(answer, name) {
			return name, nil
		}
	}
	return "", errs.New(errs.CodeUsage, fmt.Sprintf("no login method named %q", answer)).
		WithHint("one of: " + strings.Join(numbered, ", "))
}

// suggestSiteName turns a host into a short, unused name, so that the common
// case needs no --name. "ecourse2.ccu.edu.tw" becomes "ccu".
func suggestSiteName(host string, file *config.File) string {
	host = strings.ToLower(host)
	if colon := strings.IndexByte(host, ':'); colon >= 0 {
		host = host[:colon]
	}
	labels := strings.Split(host, ".")
	// The institution's own label: the one before the public suffix, skipping
	// the parts that say only which country and which kind of body it is.
	generic := map[string]bool{
		"edu": true, "ac": true, "com": true, "org": true, "net": true, "gov": true,
		"tw": true, "uk": true, "jp": true, "cn": true, "au": true, "nz": true,
	}
	candidate := ""
	for _, label := range labels {
		if label == "" || generic[label] {
			continue
		}
		candidate = label
	}
	if candidate == "" || candidate == "www" {
		candidate = strings.ReplaceAll(host, ".", "-")
	}
	if _, taken := file.Sites[candidate]; !taken {
		return candidate
	}
	// An existing name is never silently reused for a different address.
	for suffix := 2; ; suffix++ {
		name := candidate + "-" + strconv.Itoa(suffix)
		if _, taken := file.Sites[name]; !taken {
			return name
		}
	}
}

package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// The command is not called "completion": that name belongs to Moodle's own
// activity completion, which a student asks about far more often than they
// install a shell script. Cobra skips adding its own generator when a command
// of that name already exists, which is why this one exists — the hidden
// __complete command it also installs is unaffected, so tab completion itself
// has always worked, with no way to switch it on.
func newShellCompletionCommand(r *Renderer, rootOf func() *cobra.Command) *cobra.Command {
	generators := map[string]func(*cobra.Command, io.Writer) error{
		"bash": func(root *cobra.Command, w io.Writer) error {
			return root.GenBashCompletionV2(w, true)
		},
		"zsh":        func(root *cobra.Command, w io.Writer) error { return root.GenZshCompletion(w) },
		"fish":       func(root *cobra.Command, w io.Writer) error { return root.GenFishCompletion(w, true) },
		"powershell": func(root *cobra.Command, w io.Writer) error { return root.GenPowerShellCompletionWithDesc(w) },
	}
	shells := make([]string, 0, len(generators))
	for name := range generators {
		shells = append(shells, name)
	}
	sort.Strings(shells)

	cmd := &cobra.Command{
		Use:   "shell-completion <" + strings.Join(shells, "|") + ">",
		Short: "Print a tab-completion script for your shell",
		Long: "Prints the script on stdout; where to put it differs by shell.\n\n" +
			"  bash:        moodle shell-completion bash > /etc/bash_completion.d/moodle\n" +
			"  zsh:         moodle shell-completion zsh > \"${fpath[1]}/_moodle\"\n" +
			"  fish:        moodle shell-completion fish > ~/.config/fish/completions/moodle.fish\n" +
			"  powershell:  moodle shell-completion powershell | Out-String | Invoke-Expression\n\n" +
			"Site and account names are completed from your configuration, so no\n" +
			"request is made while you are typing. In read-only mode the script\n" +
			"leaves out the commands that mode withholds.",
		Args:      cobra.ExactArgs(1),
		ValidArgs: shells,
		RunE: func(cmd *cobra.Command, args []string) error {
			generate, ok := generators[args[0]]
			if !ok {
				return errs.New(errs.CodeUsage, fmt.Sprintf("unknown shell %q", args[0])).
					WithHint("one of: " + strings.Join(shells, ", "))
			}
			// The script is the requested output, so it goes to stdout even
			// though it is not the JSON contract — the same rule that puts a
			// schema document there.
			return generate(rootOf(), r.Streams.Out)
		},
	}
	return cmd
}

// completeSites and completeAccounts answer from the configuration file. They
// must not reach the network: a shell runs them while the user is still
// typing, and a slow or failing request would look like a broken terminal.
func completeSites(deps Deps) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		file, err := config.Load(deps.ConfigPath)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		names := make([]string, 0, len(file.Sites))
		for name, site := range file.Sites {
			if !strings.HasPrefix(name, toComplete) {
				continue
			}
			// The URL is the one thing that tells two similar names apart.
			names = append(names, name+"\t"+site.BaseURL)
		}
		sort.Strings(names)
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

func completeAccounts(deps Deps) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		file, err := config.Load(deps.ConfigPath)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		// An account name is only unique within its site, so --site decides
		// which ones are offered; without it, the default site's.
		siteName, _ := cmd.Flags().GetString("site")
		resolved, err := file.Resolve(siteName, "")
		if err != nil || resolved.Site == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names := make([]string, 0, len(resolved.Site.Accounts))
		for name := range resolved.Site.Accounts {
			if strings.HasPrefix(name, toComplete) {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

// registerFlagCompletion attaches the site and account completers to every
// command that carries those flags, so a new command gets them by existing
// rather than by remembering to ask.
func registerFlagCompletion(root *cobra.Command, deps Deps) {
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd.Flags().Lookup("site") != nil {
			_ = cmd.RegisterFlagCompletionFunc("site", completeSites(deps))
		}
		if cmd.Flags().Lookup("account") != nil {
			_ = cmd.RegisterFlagCompletionFunc("account", completeAccounts(deps))
		}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)
}

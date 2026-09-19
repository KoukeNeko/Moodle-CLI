package cli

import (
	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
)

func newMCPCommand(deps Deps, mode *safety.Mode) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve Moodle to an AI agent",
	}
	cmd.AddCommand(newMCPServeCommand(deps, mode))
	return cmd
}

func newMCPServeCommand(deps Deps, mode *safety.Mode) *cobra.Command {
	var (
		flags      sessionFlags
		allowWrite bool
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run a Model Context Protocol server on stdin and stdout",
		Long: "Serves the same use cases the commands use, over the Model Context\n" +
			"Protocol. An agent and a person asking the same question get the same\n" +
			"answer, because there is one implementation underneath.\n\n" +
			"Read-only unless you pass --allow-write. Without it, the tools that can\n" +
			"change anything are not offered at all — an agent cannot decide to try a\n" +
			"tool it cannot see. Handing coursework in cannot be undone, so that is\n" +
			"the default worth having.\n\n" +
			"stdout carries the protocol and nothing else; diagnostics go to stderr.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationKind: "mcp.serve"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if allowWrite && mode.ReadOnly {
				// Saying yes to both is a contradiction, and picking one
				// silently would be picking for them.
				return errs.New(errs.CodeUsage,
					"--allow-write and --read-only ask for opposite things").
					WithHint("drop one of them")
			}

			file, err := config.Load(deps.ConfigPath)
			if err != nil {
				return err
			}
			resolved, token, err := resolveSession(deps, file, flags.site, flags.account)
			if err != nil {
				return err
			}
			session := openSessionFor(deps, resolved, token)
			// This process outlives the credential it starts with. Without
			// this, a student who signs in again in another terminal leaves
			// the server holding a dead token for ever: the agent is told
			// correctly that authentication failed and can do nothing about
			// it, because it cannot sign in and the human who can has no way
			// to hand the result over short of a restart.
			if resolved.Site != nil && resolved.Account != nil {
				deps.Auth.KeepCredentialFresh(session, resolved.Site.ID, resolved.Account.ID)
			}
			// The handshake happens before any tool call, so a site that
			// cannot be reached is reported now rather than inside a tool.
			capabilities, err := session.Capabilities(cmd.Context())
			if err != nil {
				return err
			}

			return deps.ServeMCP(cmd.Context(), MCPSession{
				Session:      session,
				Capabilities: capabilities,
				SiteName:     resolved.SiteName,
				AccountName:  resolved.AccountName,
				AllowWrite:   allowWrite,
			})
		},
	}
	flags.bind(cmd, "serve")
	cmd.Flags().BoolVar(&allowWrite, "allow-write", false,
		"offer the tools that can change things on the site")
	return cmd
}

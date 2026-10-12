package cli

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/KoukeNeko/moodle-cli/internal/config"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/safety"
)

// capabilityTTL is how long the site's answer is kept. The environment
// variable exists so the expiry can be exercised against a real site without
// a five-minute test.
func capabilityTTL() time.Duration {
	if raw := os.Getenv("MOODLE_CAPABILITY_TTL"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
	}
	return 5 * time.Minute
}

// EnvHostPassphrase supplies the host passphrase without a file, for a
// container whose environment is the only configuration it has.
const EnvHostPassphrase = "MOODLE_HOST_PASSPHRASE"

func newMCPCommand(deps Deps, mode *safety.Mode) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve Moodle to an AI agent",
	}
	cmd.AddCommand(newMCPServeCommand(deps, mode), newMCPHostCommand(deps, mode))
	return cmd
}

// openMCPSession signs the process in for a server that outlives one command.
//
// Both transports start here, so a site that cannot be reached is reported
// before a client connects rather than inside its first tool call.
func openMCPSession(cmd *cobra.Command, deps Deps, mode *safety.Mode,
	flags sessionFlags, allowWrite bool) (MCPSession, error) {
	if allowWrite && mode.ReadOnly {
		// Saying yes to both is a contradiction, and picking one
		// silently would be picking for them.
		return MCPSession{}, errs.New(errs.CodeUsage,
			"--allow-write and --read-only ask for opposite things").
			WithHint("drop one of them")
	}

	file, err := config.Load(deps.ConfigPath)
	if err != nil {
		return MCPSession{}, err
	}
	resolved, token, err := resolveSession(deps, file, flags.site, flags.account)
	if err != nil {
		return MCPSession{}, err
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
	// Five minutes is a compromise between two wrong answers. Never
	// asking again means a function switched on by an administrator
	// stays refused for the life of the server — measured. Asking on
	// every tool call doubles the traffic this server puts on a
	// university's Moodle for a setting that changes yearly.
	session.ExpireCapabilitiesAfter(capabilityTTL())
	// The handshake happens before any tool call, so a site that
	// cannot be reached is reported now rather than inside a tool.
	capabilities, err := session.Capabilities(cmd.Context())
	if err != nil {
		return MCPSession{}, err
	}

	return MCPSession{
		Session:      session,
		Capabilities: capabilities,
		SiteName:     resolved.SiteName,
		AccountName:  resolved.AccountName,
		AllowWrite:   allowWrite,
	}, nil
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
			session, err := openMCPSession(cmd, deps, mode, flags, allowWrite)
			if err != nil {
				return err
			}
			return deps.ServeMCP(cmd.Context(), session)
		},
	}
	flags.bind(cmd, "serve")
	cmd.Flags().BoolVar(&allowWrite, "allow-write", false,
		"offer the tools that can change things on the site")
	return cmd
}

func newMCPHostCommand(deps Deps, mode *safety.Mode) *cobra.Command {
	var (
		flags          sessionFlags
		allowWrite     bool
		listen         string
		publicURL      string
		passphraseFile string
	)
	cmd := &cobra.Command{
		Use:   "host",
		Short: "Serve Moodle to Claude as a connector, over HTTPS",
		Long: "Serves the same tools as `mcp serve` over HTTP, so a client that cannot\n" +
			"start this program can use them: Claude on the web, on a phone, or in\n" +
			"Cowork, added as a custom connector.\n\n" +
			"Claude connects from Anthropic's servers, not from your machine, so the\n" +
			"host needs an HTTPS address they can reach. Run it somewhere that stays\n" +
			"on, put a tunnel or reverse proxy that terminates HTTPS in front of\n" +
			"--listen, and pass that address as --public-url.\n\n" +
			"Without --public-url the host starts a Cloudflare quick tunnel itself\n" +
			"(cloudflared must be installed), listens on a free loopback port, and\n" +
			"uses the address it is given: a random trycloudflare.com address that is\n" +
			"new on every start and open to anyone until the host stops, with\n" +
			"Cloudflare relaying all traffic. Cloudflare gives quick tunnels no uptime\n" +
			"guarantee, so that is for trying it out.\n\n" +
			"The host prints the connector URL to add in Claude; it ends in /mcp.\n\n" +
			"A client signs in once. The sign-in page asks for the host passphrase, so\n" +
			"only you can admit one. Once admitted, a client acts as this account\n" +
			"until the host stops or the sign-in lapses after 30 days without use,\n" +
			"and the host holds this account's Moodle credential while it runs.\n" +
			"Stopping the host ends every sign-in, and the connector has to be added\n" +
			"again. Each tool call is noted on stderr, without its arguments.\n\n" +
			"Read-only unless you pass --allow-write, exactly as for `mcp serve`. With\n" +
			"writing on, remember that course content is written by other people and\n" +
			"reaches Claude: keep Claude's permission for the writing tools on asking.\n\n" +
			"The passphrase comes from --passphrase-file, then " + EnvHostPassphrase + ".\n" +
			"With neither, one is generated and printed on stderr for this run.",
		Args: cobra.NoArgs,
		// Not "read": it starts a program, binds a port, and — without
		// --public-url — publishes the account on the internet until it stops.
		// An unattended caller should weigh that, not run it freely.
		Annotations: map[string]string{annotationKind: "mcp.host", annotationSafety: safetyLocal},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("public-url") && strings.TrimSpace(publicURL) == "" {
				// Not the same as leaving it out: a script that passes an unset
				// variable would otherwise publish the host through a quick
				// tunnel nobody asked for.
				return errs.New(errs.CodeUsage, "--public-url is empty").
					WithHint("give the HTTPS address Claude will reach, or leave --public-url out to start a Cloudflare quick tunnel")
			}
			passphrase, err := hostPassphrase(passphraseFile)
			if err != nil {
				return err
			}
			session, err := openMCPSession(cmd, deps, mode, flags, allowWrite)
			if err != nil {
				return err
			}
			return deps.ServeMCPHost(cmd.Context(), session, MCPHost{
				Addr:       hostAddress(listen, publicURL),
				PublicURL:  strings.TrimSpace(publicURL),
				Passphrase: passphrase,
			})
		},
	}
	flags.bind(cmd, "serve")
	cmd.Flags().BoolVar(&allowWrite, "allow-write", false,
		"offer the tools that can change things on the site")
	cmd.Flags().StringVar(&listen, "listen", "",
		"address to listen on, kept on loopback for a tunnel or proxy to reach "+
			"(default 127.0.0.1:8787 with --public-url, otherwise a free port)")
	cmd.Flags().StringVar(&publicURL, "public-url", "",
		"HTTPS address Claude connects to, with no path; without it a Cloudflare quick tunnel is started")
	cmd.Flags().StringVar(&passphraseFile, "passphrase-file", "",
		"file holding the host passphrase; a generated one is printed when none is given")
	return cmd
}

// hostAddress is where the host listens when the person did not say. With an
// address of their own a tunnel or proxy has to find it, so it is fixed; with
// the host's own quick tunnel nothing needs to find it, and a free port cannot
// be one that something else holds.
func hostAddress(listen, publicURL string) string {
	switch {
	case listen != "":
		return listen
	case strings.TrimSpace(publicURL) == "":
		return "127.0.0.1:0"
	default:
		return "127.0.0.1:8787"
	}
}

// hostPassphrase reads the passphrase from where the person put it. A file
// beats the environment: a flag is a decision made for this run.
func hostPassphrase(file string) (string, error) {
	if file == "" {
		return strings.TrimSpace(os.Getenv(EnvHostPassphrase)), nil
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return "", errs.Wrap(errs.CodeConfiguration, err, "cannot read the passphrase file").
			WithHint("check the path given to --passphrase-file")
	}
	passphrase := strings.TrimSpace(string(raw))
	if passphrase == "" {
		return "", errs.New(errs.CodeConfiguration, "the passphrase file is empty")
	}
	return passphrase, nil
}

package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	v1 "github.com/KoukeNeko/moodle-cli/internal/contract/v1"
	"github.com/KoukeNeko/moodle-cli/internal/errs"
)

// newAuthRegisterHandlerCommand installs the desktop handler.
//
// It is explicit, and asked for by name, because it is not an ephemeral
// action: it writes files the desktop reads, claims a URL scheme in a
// namespace nobody owns, and leaves both behind until they are removed. Doing
// that quietly during a sign-in would be changing someone's desktop because
// they wanted to read their coursework.
func newAuthRegisterHandlerCommand(r *Renderer, handler CallbackHandler) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register-handler",
		Short: "Let your browser hand sign-ins back to this tool",
		Long: "Installs a per-user handler so that, after you sign in through\n" +
			"your browser, the result comes back here on its own.\n\n" +
			"It writes two files and asks your desktop to associate a URL\n" +
			"scheme with them. Nothing needs administrator rights, nothing\n" +
			"outside your own account is touched, and `unregister-handler`\n" +
			"removes exactly what this wrote.",
		Args: cobra.NoArgs,
		Annotations: map[string]string{
			annotationKind: "auth.handler", annotationMutates: "true", annotationSafety: safetyLocal,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if handler == nil {
				return handlerUnavailable()
			}
			registration, err := handler.Register()
			if err != nil {
				return err
			}
			return r.Render(Result{
				Envelope: handlerEnvelope(registration),
				Human: func(w io.Writer) error {
					return writeRegistration(w, registration)
				},
			})
		},
	}
	return cmd
}

func newAuthUnregisterHandlerCommand(r *Renderer, handler CallbackHandler) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unregister-handler",
		Short: "Remove the browser sign-in handler",
		Args:  cobra.NoArgs,
		Annotations: map[string]string{
			annotationKind: "auth.handler", annotationMutates: "true", annotationSafety: safetyLocal,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if handler == nil {
				return handlerUnavailable()
			}
			status := handler.Status()
			if err := handler.Unregister(); err != nil {
				return err
			}
			removed := handler.Status()
			return r.Render(Result{
				Envelope: handlerEnvelope(removed),
				Human:    humanLine("Removed the handler for %s.", status.Scheme),
			})
		},
	}
	return cmd
}

func newAuthHandlerStatusCommand(r *Renderer, handler CallbackHandler) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "handler-status",
		Short: "Show whether the browser sign-in handler is installed",
		Args:  cobra.NoArgs,
		Annotations: map[string]string{
			annotationKind: "auth.handler",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if handler == nil {
				return handlerUnavailable()
			}
			status := handler.Status()
			return r.Render(Result{
				Envelope: handlerEnvelope(status),
				Human:    func(w io.Writer) error { return writeHandlerStatus(w, status) },
			})
		},
	}
	return cmd
}

func handlerEnvelope(reg HandlerRegistration) v1.Envelope {
	payload := v1.HandlerStatus{Scheme: reg.Scheme, Installed: reg.Installed}
	setString(&payload.DesktopFile, reg.DesktopFile)
	setString(&payload.ServiceFile, reg.ServiceFile)
	setString(&payload.Executable, reg.Executable)
	setString(&payload.MIMEDefault, reg.MIMEDefault)
	return v1.NewEnvelope("auth.handler", payload, v1.NewMeta(v1.SourceLocal))
}

// newAuthCallbackCommand is what the desktop starts. It is not for people.
func newAuthCallbackCommand(handler CallbackHandler) *cobra.Command {
	var scheme string
	if handler != nil {
		scheme = handler.Scheme()
	}
	cmd := &cobra.Command{
		Use:    "callback",
		Short:  "Receive a sign-in result from the browser (started by your desktop)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if handler == nil {
				return handlerUnavailable()
			}
			return handler.Serve(cmd.Context(), scheme)
		},
	}
	cmd.Flags().StringVar(&scheme, "scheme", scheme, "the URL scheme being handled")
	return cmd
}

func writeRegistration(w io.Writer, reg HandlerRegistration) error {
	fmt.Fprintf(w, "Installed the handler for %s://\n\n", reg.Scheme)
	fmt.Fprintf(w, "  %s\n  %s\n\n", reg.DesktopFile, reg.ServiceFile)
	switch {
	case reg.MIMEDefault == "":
		// No answer is not the same as no association: the tool that reports
		// it may simply not be here.
		fmt.Fprintln(w, strings.TrimSpace(`
Your desktop could not be asked what it will open this with, so whether the
handler fires is not something this can confirm. Try a sign-in; if nothing
comes back, `+"`moodle auth login --method manual`"+` always works.`))
	case reg.MIMEDefault != filepath.Base(reg.DesktopFile):
		// Compared against the entry's own file name, which is what the
		// desktop answers with. Matching on the scheme instead called our own
		// entry somebody else's, because the scheme is not in that name.
		fmt.Fprintf(w, "Your desktop says %q will open it, which is not this "+
			"handler.\nAutomatic sign-in will not work until that changes.\n",
			reg.MIMEDefault)
	default:
		fmt.Fprintln(w, "Your desktop confirms it will open this handler.")
	}
	return nil
}

func writeHandlerStatus(w io.Writer, status HandlerRegistration) error {
	if !status.Installed {
		fmt.Fprintf(w, "No handler is installed for %s://\n", status.Scheme)
		fmt.Fprintln(w, "Install one with `moodle auth register-handler`.")
		return nil
	}
	fmt.Fprintf(w, "Handler for %s:// is installed.\n", status.Scheme)
	fmt.Fprintf(w, "  program  %s\n", status.Executable)
	fmt.Fprintf(w, "  entry    %s\n", status.DesktopFile)
	fmt.Fprintf(w, "  service  %s\n", status.ServiceFile)
	if status.MIMEDefault == "" {
		fmt.Fprintln(w, "  desktop  could not be asked which application it would open")
	} else {
		fmt.Fprintf(w, "  desktop  opens %s\n", status.MIMEDefault)
	}
	// The program a desktop entry names is the one that answers, and an
	// upgrade that moves the binary leaves it pointing at nothing.
	if status.Executable != "" {
		if _, err := os.Stat(status.Executable); err != nil {
			fmt.Fprintln(w, "\nThat program is no longer there. Run "+
				"`moodle auth register-handler` again to point at this one.")
		}
	}
	return nil
}

func handlerUnavailable() error {
	return errs.New(errs.CodeUnavailable,
		"browser callback handling is not configured in this build").
		WithHint("use `moodle auth login --method manual`")
}

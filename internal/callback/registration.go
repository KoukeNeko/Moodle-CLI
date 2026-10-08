package callback

// Registration uses the same status contract for a Linux desktop entry or a
// macOS application bundle. MIMEDefault identifies the actual OS association.
type Registration struct {
	Scheme      string
	DesktopFile string
	ServiceFile string
	Executable  string
	MIMEDefault string
}

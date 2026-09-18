package config

import (
	"fmt"
	"sort"

	"github.com/KoukeNeko/moodle-cli/internal/errs"
	"github.com/KoukeNeko/moodle-cli/internal/site"
)

// SiteNames returns the configured site names, sorted.
func (f *File) SiteNames() []string {
	names := make([]string, 0, len(f.Sites))
	for name := range f.Sites {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// AccountNames returns the account names on a site, sorted.
func (s *Site) AccountNames() []string {
	names := make([]string, 0, len(s.Accounts))
	for name := range s.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// AddSite registers a new site and returns it.
func (f *File) AddSite(name string, baseURL string) (*Site, error) {
	if name == "" {
		return nil, errs.New(errs.CodeUsage, "site name is empty")
	}
	if f.Sites == nil {
		f.Sites = map[string]*Site{}
	}
	if _, exists := f.Sites[name]; exists {
		return nil, errs.New(errs.CodeConflict, fmt.Sprintf("site %q already exists", name)).
			WithHint("pick another name, or remove the existing one with `moodle site remove`")
	}
	parsed, err := site.ParseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	entry := &Site{
		ID:       site.NewID(),
		BaseURL:  parsed.String(),
		Backend:  BackendAuto,
		Accounts: map[string]*Account{},
	}
	f.Sites[name] = entry
	// The first site becomes current, so a single-site user never has to run
	// `site use` at all.
	if f.Current.Site == "" {
		f.Current.Site = name
	}
	return entry, nil
}

// RemoveSite drops a site and reports the accounts that went with it, so the
// caller can delete their credentials from the keychain.
func (f *File) RemoveSite(name string) ([]*Account, error) {
	entry, ok := f.Sites[name]
	if !ok {
		return nil, unknownSite(f, name)
	}
	accounts := make([]*Account, 0, len(entry.Accounts))
	for _, accountName := range entry.AccountNames() {
		accounts = append(accounts, entry.Accounts[accountName])
	}
	delete(f.Sites, name)
	if f.Current.Site == name {
		f.Current = Current{}
	}
	return accounts, nil
}

// UseSite makes a site current.
func (f *File) UseSite(name string) error {
	entry, ok := f.Sites[name]
	if !ok {
		return unknownSite(f, name)
	}
	f.Current.Site = name
	f.Current.Account = entry.DefaultAccount
	return nil
}

// Resolved is the site and account a command should act on.
type Resolved struct {
	SiteName    string
	Site        *Site
	AccountName string
	Account     *Account
}

// Resolve picks the target site and account.
//
// Order: explicit flags, then the current selection, then the only candidate
// if there is exactly one. Anything else is a configuration error that names
// the choices rather than guessing.
func (f *File) Resolve(siteName, accountName string) (Resolved, error) {
	name := siteName
	if name == "" {
		name = f.Current.Site
	}
	if name == "" && len(f.Sites) == 1 {
		name = f.SiteNames()[0]
	}
	if name == "" {
		if len(f.Sites) == 0 {
			return Resolved{}, errs.New(errs.CodeConfiguration, "no site is configured").
				WithHint("add one with `moodle site add <name> <url>`")
		}
		return Resolved{}, errs.New(errs.CodeConfiguration, "no site selected").
			WithHint(fmt.Sprintf("choose one with `moodle site use <name>`; configured: %v", f.SiteNames()))
	}
	entry, ok := f.Sites[name]
	if !ok {
		return Resolved{}, unknownSite(f, name)
	}

	resolved := Resolved{SiteName: name, Site: entry}

	account := accountName
	if account == "" && siteName == "" {
		// Only inherit the current account when the site was not overridden,
		// or we would pair an account with a site it does not belong to.
		account = f.Current.Account
	}
	if account == "" {
		account = entry.DefaultAccount
	}
	if account == "" && len(entry.Accounts) == 1 {
		account = entry.AccountNames()[0]
	}
	if account == "" {
		// Not an error: `site add` and `auth login` both run before any
		// account exists.
		return resolved, nil
	}
	accountEntry, ok := entry.Accounts[account]
	if !ok {
		return Resolved{}, errs.New(errs.CodeConfiguration,
			fmt.Sprintf("site %q has no account %q", name, account)).
			WithHint(fmt.Sprintf("known accounts: %v", entry.AccountNames()))
	}
	resolved.AccountName = account
	resolved.Account = accountEntry
	return resolved, nil
}

// RequireAccount is Resolve for commands that cannot work without credentials.
func (f *File) RequireAccount(siteName, accountName string) (Resolved, error) {
	resolved, err := f.Resolve(siteName, accountName)
	if err != nil {
		return Resolved{}, err
	}
	if resolved.Account == nil {
		return Resolved{}, errs.New(errs.CodeAuthentication,
			fmt.Sprintf("no account is configured for site %q", resolved.SiteName)).
			WithReason(errs.ReasonCredentialMissing).
			WithHint("sign in with `moodle auth login`")
	}
	return resolved, nil
}

// UpsertAccount adds or updates an account on a site, keeping its ID stable
// so the stored credential stays reachable.
func (s *Site) UpsertAccount(name string, update Account) *Account {
	if s.Accounts == nil {
		s.Accounts = map[string]*Account{}
	}
	existing, ok := s.Accounts[name]
	if !ok {
		existing = &Account{ID: site.NewID()}
		s.Accounts[name] = existing
	}
	existing.UserID = update.UserID
	existing.Username = update.Username
	existing.DisplayName = update.DisplayName
	existing.AuthMethod = update.AuthMethod
	existing.CredentialKind = update.CredentialKind
	if s.DefaultAccount == "" {
		s.DefaultAccount = name
	}
	return existing
}

// RemoveAccount drops an account and returns it so its credential can be
// deleted from the keychain too.
func (s *Site) RemoveAccount(name string) (*Account, error) {
	account, ok := s.Accounts[name]
	if !ok {
		return nil, errs.New(errs.CodeNotFound, fmt.Sprintf("no account %q on this site", name)).
			WithHint(fmt.Sprintf("known accounts: %v", s.AccountNames()))
	}
	delete(s.Accounts, name)
	if s.DefaultAccount == name {
		s.DefaultAccount = ""
		if remaining := s.AccountNames(); len(remaining) == 1 {
			s.DefaultAccount = remaining[0]
		}
	}
	return account, nil
}

func unknownSite(f *File, name string) error {
	return errs.New(errs.CodeNotFound, fmt.Sprintf("no site named %q", name)).
		WithHint(fmt.Sprintf("configured sites: %v", f.SiteNames()))
}

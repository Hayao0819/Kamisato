package domain

// Features is the capability manifest consumed by Lumine. Optional integrations
// are boolean/string fields; protocol grammar is advertised as data so the web
// client does not maintain a second allowlist.
type Features struct {
	BugReport              bool     `json:"bug_report"`
	Miko                   bool     `json:"miko"`
	GitHubLogin            bool     `json:"github_login"`
	RecaptchaSiteKey       string   `json:"recaptcha_site_key"`
	PackageArchiveSuffixes []string `json:"package_archive_suffixes"`
}

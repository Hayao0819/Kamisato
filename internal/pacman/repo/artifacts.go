package repo

// DatabaseArtifacts is the complete conventional name set for one pacman
// repository database. Alias names are addressed by RepoName; archive names are
// stored under ArchivePrefix. They are normally equal, but a merged or snapshot
// database can be stored under a private prefix while serving the public aliases.
type DatabaseArtifacts struct {
	repoName      string
	archivePrefix string
}

// Artifacts returns the conventional database artifact names for repoName.
func Artifacts(repoName string) DatabaseArtifacts {
	return DatabaseArtifacts{repoName: repoName, archivePrefix: repoName}
}

// WithArchivePrefix returns the same public alias set backed by archives under
// prefix. This models internal variants such as "<repo>.merged.db.tar.gz".
func (a DatabaseArtifacts) WithArchivePrefix(prefix string) DatabaseArtifacts {
	a.archivePrefix = prefix
	return a
}

func (a DatabaseArtifacts) DatabaseAlias() string { return a.repoName + ".db" }
func (a DatabaseArtifacts) FilesAlias() string    { return a.repoName + ".files" }

func (a DatabaseArtifacts) DatabaseAliasSignature() string {
	return a.DatabaseAlias() + ".sig"
}

func (a DatabaseArtifacts) FilesAliasSignature() string {
	return a.FilesAlias() + ".sig"
}

func (a DatabaseArtifacts) DatabaseArchive() string {
	return a.archivePrefix + ".db.tar.gz"
}

func (a DatabaseArtifacts) FilesArchive() string {
	return a.archivePrefix + ".files.tar.gz"
}

func (a DatabaseArtifacts) DatabaseArchiveSignature() string {
	return a.DatabaseArchive() + ".sig"
}

func (a DatabaseArtifacts) FilesArchiveSignature() string {
	return a.FilesArchive() + ".sig"
}

// Aliases returns the uncompressed public database names.
func (a DatabaseArtifacts) Aliases() []string {
	return []string{a.DatabaseAlias(), a.FilesAlias()}
}

// AliasSignatures returns the detached signatures requested beside the public
// aliases.
func (a DatabaseArtifacts) AliasSignatures() []string {
	return []string{a.DatabaseAliasSignature(), a.FilesAliasSignature()}
}

// Archives returns the canonical compressed database objects.
func (a DatabaseArtifacts) Archives() []string {
	return []string{a.DatabaseArchive(), a.FilesArchive()}
}

// ArchiveSignatures returns the detached signatures stored beside the archives.
func (a DatabaseArtifacts) ArchiveSignatures() []string {
	return []string{a.DatabaseArchiveSignature(), a.FilesArchiveSignature()}
}

// All returns every public alias and stored archive name, including signatures.
func (a DatabaseArtifacts) All() []string {
	names := make([]string, 0, 8)
	names = append(names, a.Aliases()...)
	names = append(names, a.Archives()...)
	names = append(names, a.AliasSignatures()...)
	names = append(names, a.ArchiveSignatures()...)
	return names
}

// ArchiveForAlias resolves a public alias (including its signature) to the
// corresponding stored archive. The bool is false for non-database names.
func (a DatabaseArtifacts) ArchiveForAlias(name string) (string, bool) {
	switch name {
	case a.DatabaseAlias():
		return a.DatabaseArchive(), true
	case a.FilesAlias():
		return a.FilesArchive(), true
	case a.DatabaseAliasSignature():
		return a.DatabaseArchiveSignature(), true
	case a.FilesAliasSignature():
		return a.FilesArchiveSignature(), true
	default:
		return "", false
	}
}

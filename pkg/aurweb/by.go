package aurweb

// By is the field a type=search request matches against, mirroring aurweb's EXPOSED_BYS.
type By string

const (
	ByName          By = "name"
	ByNameDesc      By = "name-desc"
	ByMaintainer    By = "maintainer"
	BySubmitter     By = "submitter"
	ByDepends       By = "depends"
	ByMakeDepends   By = "makedepends"
	ByOptDepends    By = "optdepends"
	ByCheckDepends  By = "checkdepends"
	ByProvides      By = "provides"
	ByConflicts     By = "conflicts"
	ByReplaces      By = "replaces"
	ByGroups        By = "groups"
	ByKeywords      By = "keywords"
	ByCoMaintainers By = "comaintainers"
)

// DefaultBy is the field aurweb searches when none is given.
const DefaultBy = ByNameDesc

var exposedBys = map[By]bool{
	ByName: true, ByNameDesc: true, ByMaintainer: true, BySubmitter: true,
	ByDepends: true, ByMakeDepends: true, ByOptDepends: true, ByCheckDepends: true,
	ByProvides: true, ByConflicts: true, ByReplaces: true, ByGroups: true,
	ByKeywords: true, ByCoMaintainers: true,
}

func (b By) valid() bool { return exposedBys[b] }

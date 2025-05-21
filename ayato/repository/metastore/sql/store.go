package sql

import (
	"fmt"

	"gorm.io/gorm/clause"
)

func (s *Sql) StorePackageFile(packageName, filePath string) error {
	if s.db == nil {
		return fmt.Errorf("database connection is nil")
	}

	pkg := PackageFile{
		PackageName: packageName,
		FilePath:    filePath,
	}
	return s.db.
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "package_name"}},
			DoUpdates: clause.AssignmentColumns([]string{"file_path"}),
		}).
		Create(&pkg).Error
}

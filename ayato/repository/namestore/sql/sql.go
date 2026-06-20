package sql

import (
	"fmt"

	utils "github.com/Hayao0819/Kamisato/internal/utils"
	_ "github.com/lib/pq"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Sql struct {
	db *gorm.DB
}

func NewSql(driver string, dsn string) (*Sql, error) {
	var dialector gorm.Dialector

	switch driver {
	case "postgres":
		dialector = postgres.Open(dsn)
	case "mysql":
		dialector = mysql.Open(dsn)
	case "sqlite":
		dialector = sqlite.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: utils.GormLog(),
	})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&PackageFile{}); err != nil {
		return nil, fmt.Errorf("failed to auto migrate: %w", err)
	}
	return &Sql{
		db: db,
	}, nil
}

type PackageFile struct {
	PackageName string `gorm:"primary_key"`
	FilePath    string
}

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

func (s *Sql) PackageFile(packageName string) (string, error) {
	var pkg PackageFile
	err := s.db.
		Where("package_name = ?", packageName).
		First(&pkg).Error
	if err != nil {
		return "", err
	}

	return pkg.FilePath, nil
}

func (s *Sql) DeletePackageFileEntry(packageName string) error {
	return s.db.
		Where("package_name = ?", packageName).
		Delete(&PackageFile{}).Error
}

package sql

import "github.com/jinzhu/gorm"

type Sql struct {
	db *gorm.DB
}

func NewSql(driver string, dsn string) (*Sql, error) {
	db, err := gorm.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	db.AutoMigrate(&PackageFile{})
	return &Sql{
		db: db,
	}, nil
}

type PackageFile struct {
	PackageName string `gorm:"primary_key"`
	FilePath    string
}

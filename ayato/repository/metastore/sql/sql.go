package sql

import (
	"fmt"

	utils "github.com/Hayao0819/Kamisato/internal/utils"
	_ "github.com/lib/pq"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

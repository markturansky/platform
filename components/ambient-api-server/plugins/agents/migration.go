package agents

import (
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
)

func migration() *gormigrate.Migration {
	type Agent struct {
		db.Model
		Name    string
		RepoUrl *string
		Prompt  *string
	}

	return &gormigrate.Migration{
		ID: "202602132212",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Agent{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&Agent{})
		},
	}
}

func constraintMigration() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202602150002",
		Migrate: func(tx *gorm.DB) error {
			return tx.Exec(`CREATE INDEX IF NOT EXISTS idx_agents_name ON agents(name)`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Exec(`DROP INDEX IF EXISTS idx_agents_name`).Error
		},
	}
}

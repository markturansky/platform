package sessions

import (
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
)

func migration() *gormigrate.Migration {
	type Session struct {
		db.Model
		Name            string
		RepoUrl         *string
		Prompt          *string
		CreatedByUserId *string
		AssignedUserId  *string
		WorkflowId      *string
	}

	return &gormigrate.Migration{
		ID: "202602132218",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Session{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&Session{})
		},
	}
}

func constraintMigration() *gormigrate.Migration {
	migrateStatements := []string{
		`UPDATE sessions SET created_by_user_id = NULL WHERE created_by_user_id IS NOT NULL
			AND created_by_user_id NOT IN (SELECT id FROM users WHERE deleted_at IS NULL)
			AND deleted_at IS NULL`,
		`UPDATE sessions SET assigned_user_id = NULL WHERE assigned_user_id IS NOT NULL
			AND assigned_user_id NOT IN (SELECT id FROM users WHERE deleted_at IS NULL)
			AND deleted_at IS NULL`,
		`UPDATE sessions SET workflow_id = NULL WHERE workflow_id IS NOT NULL
			AND workflow_id NOT IN (SELECT id FROM workflows WHERE deleted_at IS NULL)
			AND deleted_at IS NULL`,
		`ALTER TABLE sessions ADD CONSTRAINT fk_sessions_created_by_user_id
			FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE SET NULL`,
		`ALTER TABLE sessions ADD CONSTRAINT fk_sessions_assigned_user_id
			FOREIGN KEY (assigned_user_id) REFERENCES users(id) ON DELETE SET NULL`,
		`ALTER TABLE sessions ADD CONSTRAINT fk_sessions_workflow_id
			FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE SET NULL`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_created_by ON sessions(created_by_user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_assigned_to ON sessions(assigned_user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_workflow ON sessions(workflow_id)`,
	}
	rollbackStatements := []string{
		`ALTER TABLE sessions DROP CONSTRAINT IF EXISTS fk_sessions_created_by_user_id`,
		`ALTER TABLE sessions DROP CONSTRAINT IF EXISTS fk_sessions_assigned_user_id`,
		`ALTER TABLE sessions DROP CONSTRAINT IF EXISTS fk_sessions_workflow_id`,
		`DROP INDEX IF EXISTS idx_sessions_created_by`,
		`DROP INDEX IF EXISTS idx_sessions_assigned_to`,
		`DROP INDEX IF EXISTS idx_sessions_workflow`,
	}

	return &gormigrate.Migration{
		ID: "202602150006",
		Migrate: func(tx *gorm.DB) error {
			for _, stmt := range migrateStatements {
				if err := tx.Exec(stmt).Error; err != nil {
					return err
				}
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			for _, stmt := range rollbackStatements {
				if err := tx.Exec(stmt).Error; err != nil {
					return err
				}
			}
			return nil
		},
	}
}

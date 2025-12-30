package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddGaplessPlaybackColumns, downAddGaplessPlaybackColumns)
}

func upAddGaplessPlaybackColumns(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`
ALTER TABLE media_file ADD COLUMN encoder_delay INTEGER DEFAULT 0;
ALTER TABLE media_file ADD COLUMN encoder_padding INTEGER DEFAULT 0;
ALTER TABLE media_file ADD COLUMN total_samples INTEGER DEFAULT 0;
`)
	return err
}

func downAddGaplessPlaybackColumns(_ context.Context, tx *sql.Tx) error {
	// SQLite doesn't support DROP COLUMN in older versions, so we leave columns in place
	// The columns will simply contain default values
	return nil
}

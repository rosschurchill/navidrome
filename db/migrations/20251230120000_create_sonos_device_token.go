package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upCreateSonosDeviceToken, downCreateSonosDeviceToken)
}

func upCreateSonosDeviceToken(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`
create table if not exists sonos_device_token
(
    id            varchar(255) not null primary key,
    user_id       varchar(255) not null references user(id) on delete cascade,
    household_id  varchar(255) not null,
    token         varchar(255) not null unique,
    device_name   varchar(255) default '' not null,
    last_seen_at  datetime,
    created_at    datetime not null,
    updated_at    datetime not null
);

create index if not exists sonos_device_token_user_id on sonos_device_token(user_id);
create index if not exists sonos_device_token_household_id on sonos_device_token(household_id);
`)
	return err
}

func downCreateSonosDeviceToken(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`drop table if exists sonos_device_token;`)
	return err
}

-- +goose Up
-- +goose StatementBegin
-- Album artist overrides allow users to manually correct album grouping
-- These overrides persist through scans and take precedence over file metadata

create table if not exists album_artist_override (
    id varchar primary key,
    -- Pattern to match: can be album name or folder path
    match_pattern varchar not null,
    match_type varchar not null default 'album_name', -- 'album_name', 'folder_path', 'media_file_id'
    -- The album artist to use instead of file metadata
    album_artist varchar not null,
    -- Metadata
    created_at datetime default (datetime(current_timestamp, 'localtime')) not null,
    created_by varchar not null default ''
);

create index if not exists album_artist_override_match on album_artist_override(match_type, match_pattern);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table if exists album_artist_override;
-- +goose StatementEnd

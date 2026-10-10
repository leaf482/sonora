-- +goose Up
CREATE TABLE artists (
    id uuid PRIMARY KEY,
    name text NOT NULL CONSTRAINT artists_name_not_blank CHECK (length(btrim(name)) > 0),
    biography text,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE albums (
    id uuid PRIMARY KEY,
    primary_artist_id uuid NOT NULL,
    title text NOT NULL CONSTRAINT albums_title_not_blank CHECK (length(btrim(title)) > 0),
    release_date date,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT albums_primary_artist_fk FOREIGN KEY (primary_artist_id)
        REFERENCES artists (id) ON DELETE RESTRICT ON UPDATE RESTRICT
);

-- PostgreSQL does not automatically index the referencing side of a foreign key.
-- Supports artist album lookup and checks when deleting/updating an artist ID.
CREATE INDEX albums_primary_artist_id_idx ON albums (primary_artist_id);

-- +goose Down
-- Destructive: use only for disposable verification or a separately approved rollback.
-- No CASCADE: future references must be rolled back explicitly first.
DROP TABLE albums;
DROP TABLE artists;

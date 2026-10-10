-- +goose Up
CREATE TABLE tracks (
    id uuid PRIMARY KEY,
    primary_artist_id uuid NOT NULL REFERENCES artists (id) ON DELETE RESTRICT ON UPDATE RESTRICT,
    album_id uuid REFERENCES albums (id) ON DELETE RESTRICT ON UPDATE RESTRICT,
    title text NOT NULL CHECK (length(btrim(title)) > 0),
    display_credits text,
    duration_ms bigint CHECK (duration_ms > 0),
    disc_number integer,
    track_number integer,
    state text NOT NULL DEFAULT 'draft' CHECK (state IN ('draft', 'active', 'unavailable')),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT tracks_position CHECK (
        (album_id IS NULL AND disc_number IS NULL AND track_number IS NULL)
        OR (album_id IS NOT NULL AND disc_number IS NOT NULL AND track_number IS NOT NULL
            AND disc_number > 0 AND track_number > 0)
    ),
    CONSTRAINT tracks_album_position UNIQUE (album_id, disc_number, track_number),
    CONSTRAINT tracks_active_duration CHECK (state <> 'active' OR duration_ms IS NOT NULL)
);
-- The album-position unique index already supports album FK reference checks.
CREATE INDEX tracks_primary_artist_id_idx ON tracks (primary_artist_id);

CREATE TABLE audio_assets (
    id uuid PRIMARY KEY,
    track_id uuid NOT NULL UNIQUE REFERENCES tracks (id) ON DELETE RESTRICT ON UPDATE RESTRICT,
    -- Flat generated filename, not an absolute path, URL or user-supplied path.
    storage_key text NOT NULL UNIQUE CHECK (storage_key ~ '^[a-z0-9][a-z0-9_-]*(\.[a-z0-9]+)?$'),
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    mime_type text NOT NULL CHECK (mime_type ~ '^audio/[a-zA-Z0-9.+-]+$'),
    codec text NOT NULL CHECK (length(btrim(codec)) > 0),
    byte_length bigint NOT NULL CHECK (byte_length > 0),
    duration_ms bigint NOT NULL CHECK (duration_ms > 0),
    file_state text NOT NULL DEFAULT 'staged' CHECK (file_state IN ('staged', 'ready', 'unavailable')),
    license_decision text NOT NULL DEFAULT 'pending' CHECK (license_decision IN ('pending', 'verified', 'rejected', 'withdrawn')),
    acquired_at timestamptz NOT NULL,

    source_provider text,
    source_url text,
    download_reference text,
    creator_credits text,
    rights_holder_credits text,
    license_name text,
    license_version text,
    license_url text,
    evidence_url text,
    evidence_reference text,
    reviewed_by text,
    reviewed_at timestamptz,
    review_rationale text,
    -- NULL is unknown, false is prohibited, true is permitted only within the recorded scope.
    hosting_permitted boolean,
    playback_permitted boolean,
    redistribution_permitted boolean,
    commercial_use_permitted boolean,
    audience_scope text,
    territory_scope text,
    license_term text,
    license_expires_at timestamptz,
    use_conditions text,
    attribution text,
    modifications text,

    unavailable_at timestamptz,
    unavailable_reason text,
    withdrawn_at timestamptz,
    withdrawal_reason text,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT audio_assets_review CHECK (
        license_decision = 'pending' OR (
            NULLIF(btrim(reviewed_by), '') IS NOT NULL AND reviewed_at IS NOT NULL
            AND NULLIF(btrim(review_rationale), '') IS NOT NULL)
    ),
    CONSTRAINT audio_assets_verified_provenance CHECK (
        license_decision <> 'verified' OR (
            NULLIF(btrim(source_provider), '') IS NOT NULL
            AND NULLIF(btrim(source_url), '') IS NOT NULL
            AND NULLIF(btrim(download_reference), '') IS NOT NULL
            AND NULLIF(btrim(creator_credits), '') IS NOT NULL
            AND NULLIF(btrim(rights_holder_credits), '') IS NOT NULL
            AND NULLIF(btrim(license_name), '') IS NOT NULL
            AND NULLIF(btrim(license_version), '') IS NOT NULL
            AND NULLIF(btrim(license_url), '') IS NOT NULL
            AND NULLIF(btrim(evidence_url), '') IS NOT NULL
            AND NULLIF(btrim(evidence_reference), '') IS NOT NULL
            AND hosting_permitted IS NOT NULL AND playback_permitted IS NOT NULL
            AND redistribution_permitted IS NOT NULL AND commercial_use_permitted IS NOT NULL
            AND NULLIF(btrim(audience_scope), '') IS NOT NULL
            AND NULLIF(btrim(territory_scope), '') IS NOT NULL
            AND NULLIF(btrim(license_term), '') IS NOT NULL
            AND NULLIF(btrim(use_conditions), '') IS NOT NULL
            AND NULLIF(btrim(attribution), '') IS NOT NULL
            AND NULLIF(btrim(modifications), '') IS NOT NULL)
    ),
    CONSTRAINT audio_assets_ready CHECK (
        file_state <> 'ready' OR
        (license_decision = 'verified' AND hosting_permitted IS TRUE AND playback_permitted IS TRUE)
    ),
    CONSTRAINT audio_assets_unavailable CHECK (
        (file_state = 'unavailable' AND unavailable_at IS NOT NULL AND NULLIF(btrim(unavailable_reason), '') IS NOT NULL)
        OR (file_state <> 'unavailable' AND unavailable_at IS NULL AND unavailable_reason IS NULL)
    ),
    CONSTRAINT audio_assets_withdrawn CHECK (
        (license_decision = 'withdrawn' AND file_state = 'unavailable'
            AND withdrawn_at IS NOT NULL AND NULLIF(btrim(withdrawal_reason), '') IS NOT NULL)
        OR (license_decision <> 'withdrawn' AND withdrawn_at IS NULL AND withdrawal_reason IS NULL)
    ),
    CONSTRAINT audio_assets_rejected CHECK (license_decision <> 'rejected' OR file_state = 'unavailable')
);
-- track_id/storage_key unique constraints already create the required lookup indexes.
-- No checksum uniqueness: identical bytes may have different catalog/provenance records.

-- +goose Down
-- Destructive rollback; no CASCADE. Preserve earlier catalog tables.
DROP TABLE audio_assets;
DROP TABLE tracks;

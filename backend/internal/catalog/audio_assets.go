package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrAudioAssetNotFound    = errors.New("audio asset not found")
	ErrAudioAssetUnavailable = errors.New("audio asset unavailable for playback")
	ErrAudioAssetWithdrawn   = fmt.Errorf("audio asset withdrawn: %w", ErrAudioAssetUnavailable)
)

// AudioAsset is internal media/provenance data. Never serialize it as a public Track.
// Ready metadata does not establish that the file exists or that a use fits its license.
type AudioAsset struct {
	ID                      string
	TrackID                 string
	StorageKey              string
	SHA256                  string
	MIMEType                string
	Codec                   string
	ByteLength              int64
	DurationMS              int64
	FileState               string
	LicenseDecision         string
	AcquiredAt              time.Time
	SourceProvider          *string
	SourceURL               *string
	DownloadReference       *string
	CreatorCredits          *string
	RightsHolderCredits     *string
	LicenseName             *string
	LicenseVersion          *string
	LicenseURL              *string
	EvidenceURL             *string
	EvidenceReference       *string
	ReviewedBy              *string
	ReviewedAt              *time.Time
	ReviewRationale         *string
	HostingPermitted        *bool
	PlaybackPermitted       *bool
	RedistributionPermitted *bool
	CommercialUsePermitted  *bool
	AudienceScope           *string
	TerritoryScope          *string
	LicenseTerm             *string
	LicenseExpiresAt        *time.Time
	UseConditions           *string
	Attribution             *string
	Modifications           *string
	UnavailableAt           *time.Time
	UnavailableReason       *string
	WithdrawnAt             *time.Time
	WithdrawalReason        *string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// AudioAssetByTrack includes staged, unavailable and withdrawn records for internal inspection.
// No asset row (including an absent Track) returns ErrAudioAssetNotFound, not ErrNotFound.
func (r *Repository) AudioAssetByTrack(ctx context.Context, trackID string) (AudioAsset, error) {
	return r.audioAssetByTrack(ctx, trackID, false)
}

// PlaybackAssetByTrack enforces database eligibility in one statement snapshot.
// Callers must still verify audience/territory/use conditions and file integrity at use time.
func (r *Repository) PlaybackAssetByTrack(ctx context.Context, trackID string) (AudioAsset, error) {
	return r.audioAssetByTrack(ctx, trackID, true)
}

func (r *Repository) audioAssetByTrack(ctx context.Context, trackID string, playbackOnly bool) (AudioAsset, error) {
	var a AudioAsset
	var eligible bool
	err := r.db.QueryRowContext(ctx, `SELECT
 a.id,a.track_id,a.storage_key,a.sha256,a.mime_type,
 a.codec,a.byte_length,a.duration_ms,a.file_state,a.license_decision,
 a.acquired_at,a.source_provider,a.source_url,a.download_reference,a.creator_credits,
 a.rights_holder_credits,a.license_name,a.license_version,a.license_url,a.evidence_url,
 a.evidence_reference,a.reviewed_by,a.reviewed_at,a.review_rationale,a.hosting_permitted,
 a.playback_permitted,a.redistribution_permitted,a.commercial_use_permitted,a.audience_scope,a.territory_scope,
 a.license_term,a.license_expires_at,a.use_conditions,a.attribution,a.modifications,
 a.unavailable_at,a.unavailable_reason,a.withdrawn_at,a.withdrawal_reason,a.created_at,
 a.updated_at,
 COALESCE(t.state='active' AND t.duration_ms=a.duration_ms
 AND a.file_state='ready' AND a.license_decision='verified'
 AND a.hosting_permitted IS TRUE AND a.playback_permitted IS TRUE
 AND (a.license_expires_at IS NULL OR a.license_expires_at > statement_timestamp()),false)
 FROM audio_assets a JOIN tracks t ON t.id=a.track_id WHERE a.track_id=$1`, trackID).Scan(
		&a.ID, &a.TrackID, &a.StorageKey, &a.SHA256, &a.MIMEType,
		&a.Codec, &a.ByteLength, &a.DurationMS, &a.FileState, &a.LicenseDecision,
		&a.AcquiredAt, &a.SourceProvider, &a.SourceURL, &a.DownloadReference, &a.CreatorCredits,
		&a.RightsHolderCredits, &a.LicenseName, &a.LicenseVersion, &a.LicenseURL, &a.EvidenceURL,
		&a.EvidenceReference, &a.ReviewedBy, &a.ReviewedAt, &a.ReviewRationale, &a.HostingPermitted,
		&a.PlaybackPermitted, &a.RedistributionPermitted, &a.CommercialUsePermitted, &a.AudienceScope, &a.TerritoryScope,
		&a.LicenseTerm, &a.LicenseExpiresAt, &a.UseConditions, &a.Attribution, &a.Modifications,
		&a.UnavailableAt, &a.UnavailableReason, &a.WithdrawnAt, &a.WithdrawalReason, &a.CreatedAt,
		&a.UpdatedAt, &eligible)
	if errors.Is(err, sql.ErrNoRows) {
		return AudioAsset{}, ErrAudioAssetNotFound
	}
	if err != nil {
		return AudioAsset{}, readError(ctx, "audio asset lookup", err)
	}
	if playbackOnly {
		if a.LicenseDecision == "withdrawn" {
			return AudioAsset{}, ErrAudioAssetWithdrawn
		}
		if !eligible {
			return AudioAsset{}, ErrAudioAssetUnavailable
		}
	}
	return a, nil
}

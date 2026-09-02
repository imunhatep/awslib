package iam

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A real report header, including columns this parser does not read, so the test also
// covers ignoring the unknown ones.
const reportHeader = "user,arn,user_creation_time,password_enabled,password_last_used," +
	"password_last_changed,password_next_rotation,mfa_active," +
	"access_key_1_active,access_key_1_last_rotated,access_key_1_last_used_date," +
	"access_key_1_last_used_region,access_key_1_last_used_service," +
	"access_key_2_active,access_key_2_last_rotated,access_key_2_last_used_date," +
	"access_key_2_last_used_region,access_key_2_last_used_service," +
	"cert_1_active,cert_1_last_rotated,cert_2_active,cert_2_last_rotated\n"

func TestParseCredentialReport(t *testing.T) {
	csv := reportHeader +
		"svc-deploy,arn:aws:iam::111111111111:user/svc-deploy,2020-01-02T03:04:05+00:00," +
		"false,N/A,N/A,N/A,false," +
		"true,2020-01-02T03:04:05+00:00,2026-08-01T10:00:00+00:00,eu-central-1,s3," +
		"false,N/A,N/A,N/A,N/A," +
		"false,N/A,false,N/A\n"

	entries, err := ParseCredentialReport([]byte(csv))
	require.NoError(t, err)
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, "svc-deploy", entry.User)
	assert.Equal(t, "arn:aws:iam::111111111111:user/svc-deploy", entry.Arn)
	assert.True(t, entry.AccessKey1Active)
	assert.False(t, entry.AccessKey2Active)
	assert.True(t, entry.HasAccessKey())
	assert.False(t, entry.PasswordEnabled)
	assert.False(t, entry.MfaActive)

	require.NotNil(t, entry.AccessKey1LastUsed)
	assert.Equal(t, 2026, entry.AccessKey1LastUsed.Year())
	assert.Equal(t, time.August, entry.AccessKey1LastUsed.Month())
}

// N/A, not_supported and no_information are all real values in this CSV and none is a
// date. Zero-timing them would read as year one — maximally dormant — which for
// not_supported is the opposite of the truth.
func TestParseCredentialReportNonDatesBecomeNil(t *testing.T) {
	csv := reportHeader +
		"<root_account>,arn:aws:iam::111111111111:root,2019-05-05T00:00:00+00:00," +
		"not_supported,no_information,not_supported,not_supported,true," +
		"false,N/A,N/A,N/A,N/A," +
		"false,N/A,N/A,N/A,N/A," +
		"false,N/A,false,N/A\n"

	entries, err := ParseCredentialReport([]byte(csv))
	require.NoError(t, err)
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Nil(t, entry.PasswordLastUsed)
	assert.Nil(t, entry.PasswordLastChanged)
	assert.Nil(t, entry.AccessKey1LastUsed)
	assert.Nil(t, entry.AccessKey2LastUsed)
	assert.Nil(t, entry.LastAccessKeyUse(), "a user with no key use has no last use, not the zero time")
	assert.Nil(t, entry.LastCredentialUse())
	assert.False(t, entry.HasAccessKey())
}

// The root row is not an IAM user and cannot be deleted, so callers have to be able to
// recognise it.
func TestCredentialReportEntryIsRoot(t *testing.T) {
	assert.True(t, CredentialReportEntry{User: CredentialReportRootUser}.IsRoot())
	assert.False(t, CredentialReportEntry{User: "svc-deploy"}.IsRoot())
}

// Columns are read by name, so a report whose columns move or grow still parses. AWS has
// added columns to this report before, and an index-based parser silently starts reading
// the wrong field when that happens.
func TestParseCredentialReportReadsColumnsByName(t *testing.T) {
	reordered := "arn,access_key_1_last_used_date,user,access_key_1_active,a_new_column_aws_added\n" +
		"arn:aws:iam::1:user/u,2026-08-01T10:00:00+00:00,u,true,whatever\n"

	entries, err := ParseCredentialReport([]byte(reordered))
	require.NoError(t, err)
	require.Len(t, entries, 1)

	assert.Equal(t, "u", entries[0].User)
	assert.Equal(t, "arn:aws:iam::1:user/u", entries[0].Arn)
	assert.True(t, entries[0].AccessKey1Active)
	require.NotNil(t, entries[0].AccessKey1LastUsed)
}

// A missing column leaves its field zero rather than failing the whole report: one
// unavailable column must not cost the caller every other user's data.
func TestParseCredentialReportToleratesMissingColumns(t *testing.T) {
	entries, err := ParseCredentialReport([]byte("user,arn\nsvc,arn:aws:iam::1:user/svc\n"))
	require.NoError(t, err)
	require.Len(t, entries, 1)

	assert.Equal(t, "svc", entries[0].User)
	assert.False(t, entries[0].AccessKey1Active)
	assert.Nil(t, entries[0].AccessKey1LastUsed)
}

func TestParseCredentialReportEmpty(t *testing.T) {
	_, err := ParseCredentialReport([]byte(""))
	assert.Error(t, err)
}

// The most recent use of either key wins, and a deactivated key still counts: a key
// switched off last week is evidence the user was in use last week, and ignoring it
// would score the user dormant on the strength of the deactivation.
func TestLastAccessKeyUseTakesTheLater(t *testing.T) {
	older := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	assert.Equal(t, newer, *CredentialReportEntry{
		AccessKey1LastUsed: &older,
		AccessKey2LastUsed: &newer,
	}.LastAccessKeyUse())

	assert.Equal(t, newer, *CredentialReportEntry{
		AccessKey1Active:   false,
		AccessKey1LastUsed: &newer,
		AccessKey2LastUsed: &older,
	}.LastAccessKeyUse(), "an inactive key's last use still counts as evidence of use")

	assert.Equal(t, older, *CredentialReportEntry{AccessKey1LastUsed: &older}.LastAccessKeyUse())
}

// The console password counts as a credential use too.
func TestLastCredentialUseIncludesPassword(t *testing.T) {
	keyUse := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	passwordUse := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	assert.Equal(t, passwordUse, *CredentialReportEntry{
		AccessKey1LastUsed: &keyUse,
		PasswordLastUsed:   &passwordUse,
	}.LastCredentialUse())
}

package migrations

import _ "embed"

//go:embed 000076_public_eligibility.up.sql
var PublicEligibilityViews string

//go:embed 000076_public_eligibility.down.sql
var DropPublicEligibilityViews string

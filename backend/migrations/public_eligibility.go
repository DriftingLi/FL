package migrations

import _ "embed"

//go:embed 000076_public_eligibility.up.sql
var PublicEligibilityViews string

//go:embed 000076_public_eligibility.down.sql
var DropPublicEligibilityViews string

//go:embed 000077_public_state_read_model.up.sql
var PublicStateViews string

//go:embed 000077_public_state_read_model.down.sql
var DropPublicStateViews string

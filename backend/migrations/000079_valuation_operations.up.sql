CREATE TABLE valuation_operations (
    operation_key varchar(128) PRIMARY KEY,
    request_hash varchar(64) NOT NULL,
    draft_id varchar(128) NOT NULL,
    draft_version integer NOT NULL,
    first_accepted_user_id integer,
    evaluation_id bigint,
    status varchar(16) NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed')),
    error_code varchar(64),
    created_at timestamp with time zone NOT NULL DEFAULT NOW(),
    updated_at timestamp with time zone NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_valuation_operations_evaluation_id ON valuation_operations (evaluation_id) WHERE evaluation_id IS NOT NULL;

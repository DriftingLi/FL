ALTER TABLE course ADD COLUMN public_hidden boolean NOT NULL DEFAULT false;
ALTER TABLE question ADD COLUMN public_hidden boolean NOT NULL DEFAULT false;

GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO guacamole_user;
GRANT SELECT,USAGE ON ALL SEQUENCES IN SCHEMA public TO guacamole_user;

create function hex(text) returns text language sql immutable strict as $$
  select encode($1::bytea, 'hex')
$$;

create function hex(bigint) returns text language sql immutable strict as $$
  select to_hex($1)
$$;

create function unhex(text) returns text language sql immutable strict as $$
  select encode(decode($1, 'hex'), 'escape')
$$;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

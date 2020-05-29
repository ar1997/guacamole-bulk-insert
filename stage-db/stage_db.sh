echo "Running cleanup";

dropdb guacamole_db;
createdb guacamole_db;

cat ./schema/*.sql | psql -d guacamole_db -f -;

cat ~/cleanup.sql | psql -d guacamole_db -f -;

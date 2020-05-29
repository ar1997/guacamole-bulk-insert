#!/bin/bash
cat rowcount_function.sql | psql -d guacamole_db -f -
printf "\nCreated function\n"
printf "Getting number of rows now\n"
cat print_row_num.sql | psql -d guacamole_db -f - > one.txt
read -p "Press enter to get rows again"
cat print_row_num.sql | psql -d guacamole_db -f - > two.txt
printf "\n\nFIRST ONE\n\n"
cat one.txt | grep -v "0"
printf "\n\nSECOND ONE\n\n"
cat two.txt | grep -v "0"
#diff one.txt two.txt --suppress-common-lines -y

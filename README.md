# guacamole-bulk-insert
A go program that can insert data into the Guacamole database.

- This program takes data in the form of csv
- It supports adding Connections, Users(Read only), Connection Groups, User Groups(Read only). *(Read only means, not administrators) 
- It can add connection groups, user groups
- It can map users/user groups with connections/connection groups. All using CSV.
- It needs a secondary authentication mechanism. Simply creates empty users, whose names can be same as the credentials required for SSO/LDAP etc.


# Bulk insert data into Postgres - Guacamole

「Guacamole」データベースへのデータの一括挿入

## Files and format

- users.csv

To insert a user, it expects : name of user,group(s) which they belong to, all comma seperated. Currently limiting the number of groups a user can belong to to 20. ( you can change it if you want to)

- connections.csv

1. SSH expects : protocol,name,hostname,username,port,password,connectionGroupName
2. VNC expects: protocol,name,hostname,port,password,connectionGroupName
3. RDP expects : protocol,name,hostname,port,username,password,connectionGroupName

- Adding user groups and connection groups for now is limited to command line. Will be reay in a day

- usermapping.csv

UserEntityName,UserEntityType,ConnectionEntityName,ConnectionEntityType

UserEntityName can be the name of a user/user group
UserEntityType can be either user / usergroup
ConnectionEntityName can be either a connection/connection group
ConnectionEntityType can be either connection / connectiongroup




## To clean up and reinitialize the database and to see which tables are being updated :

1. copy "stage-db" to /var/lib/postgresql

```
sudo cp -r stage-db /var/lib/postgresql/
```

2. Become postgres

```
sudo -s
su postgres
```

## clean up and re initialize

```
./stage-db/stage_db.sh
```

## which tables are being updated

```
./stage-db/row_num.sh
```

- Run, it'll get the table row counts, and then make desired changes to the database, then press enter.

# guacamole-bulk-insert
A go program that can insert data into the Guacamole database. (Postgres in this case)

NOTE :
- This program takes data as csv
- It supports adding Connections, Users, Connection Groups, User Groups.
- It can map users/user groups with connections/connection groups. All using the CSV.
- It needs a secondary authentication mechanism.

`Simply creates empty users, whose names can be same as the credentials required for user-mapping.xml / SSO / LDAP etc.`


# Bulk insert data into Postgres - Guacamole

「Guacamole」データベースへのデータの一括挿入

## users.csv
### Adding a user

1. User entry expects (This does not assign the user any parent group.)
```
user,<name of the user>
```
2. User entry with one more parent user group (The user can have as many parent user groups as desired)
```
user,<name of the user>,<parent user group #1>,<parent user group #2>
```
## Adding a user group

1. User group entry expects (This does not assign this group any parent group.)

```
usergroup,<name of the user group>
```

2. User group entry with one more parent user group (The user group can have as many parent user groups as desired)

```
usergroup,<name of the user group>,<parent user group #1>,<parent user group #2>
```

## connections.csv


### Adding connections

- It is okay to not specify the <ConnectionGroupName> at the end of the recod. The connection will then reside at the root, just like everything else.

1. SSH expects : 
```
connection,ssh,<connection name>,<hostname>,<user>,<port>,<password>,<connectionGroupName>
```
2. VNC expects: 
```
connection,vnc,<connection name>,<hostname>,<port>,<password>,<connectionGroupName>
```
3. RDP expects : 
```
connection,rdp,<connection name>,<hostname>,<user>,<port>,<password>,<connectionGroupName>
```
### Adding connection groups

- When specifying connection groups, do not worry about the order in which you're specifying them, the program first looks for all connection groups and adds all connection groups first. If it cannot find parent group of a connection/connection group, it will recursively check if that immediate parent group is supposed to be created from the same csv (looks for existsance as a child in teh same file) and creates them, if it does not exist at all, it is added to the root. Look at handleParentGroup() function for a better understanding.

- A connection group can only have one parent. It is okay to not specify the parent connection group, it will just get added to the root.

```
usergroup,<name of user group>
```
```
usergroup,<name of user group>,<name of parent group>
```

## usermapping.csv

```
```
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

# guacamole-bulk-insert
A go program that can insert data into the Guacamole database.

- This program takes data in the form of csv
- It supports adding Connections, Users(Read only), Connection Groups, User Groups(Read only). *(Read only means, not administrators) 
- It can add connection groups, user groups
- It can map users/user groups with connections/connection groups. All using CSV.
- It needs a secondary authentication mechanism. Simply creates empty users, whose names can be same as the credentials required for SSO/LDAP etc.

-- Initial commit. More info on the way.

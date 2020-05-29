package main

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	_ "github.com/lib/pq"
	"github.com/manifoldco/promptui"
)

const (
	host     = "localhost"
	port     = 5432
	user     = "guacamole_user"
	password = "password"
	dbname   = "guacamole_db"
)

// Declarations goes here
var db *sql.DB
var err error

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func checkConnectivity() {

	// Checking connectivity.
	var err error
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	fmt.Println("Checking database connectivity.")
	db, err = sql.Open("postgres", psqlInfo)
	checkErr(err)
	// defer db.Close()
	fmt.Println("Connected to the database")
}

func checkUserGroupStatus() {

	// This function checks if there exists at least a default user group.
	// If it does not exist, it creates one called "default" with just
	// read permission".

	var res bool
	err := db.QueryRow("SELECT EXISTS (SELECT user_group_id FROM guacamole_user_group)").Scan(&res)
	checkErr(err)

	if res != true {
		fmt.Println("There are no user groups present, Please create one")
		res = createUserGroup()
	}
	fmt.Println("User groups are present")
}

func checkConnectionGroupStatus() {

	var res bool
	// This function checks if there exists at least a default
	// connection group. If it does not exist, it creates one.
	err := db.QueryRow("SELECT EXISTS (SELECT connection_group_id FROM guacamole_connection_group)").Scan(&res)
	checkErr(err)

	if res != true {
		fmt.Println("There are no connection groups present, please create one")
		createConnectionGroup()
	} else {
		fmt.Println("Connection groups are present")
		listActions()
	}
}

func listActions() bool {

	var res bool
	prompt := promptui.Select{
		Label: "Select from the list",
		Items: []string{"Add users from file", "Create user groups from file", "Add connections from file",
			"Create connection groups from file", "Map users to connections and groups from file", "Exit"},
		// "Delete a user", "Delete a connection", "Delete a user group", "Delete a connection group",
	}
	_, result, err := prompt.Run()
	if err != nil {
		log.Fatalf("Prompt failed %v\n", err)
	}
	switch result {

	case "Create user groups from file":
		res = createUserGroup()

	case "Create connection groups from file":
		res = createConnectionGroup()

	case "Add users from file":
		res = createUser()

	case "Add connections from file":
		res = createConnection()

	case "Map users to connections and groups from file":
		res = mapUserstoGroups()

	// case "Delete a user group":
	// 	res = deleteUserGroup()

	// case "Delete a connection group":
	// 	res = deleteConnectionGroup()

	// case "Delete a user":
	// 	res = deleteUser()

	// case "Delete a connection":
	// 	res = deleteConnection()

	case "Exit":
		os.Exit(0)

	default:
		fmt.Println("Input not defined ?")
		listActions()
	}
	return res
}

func createUserGroup() bool {

	// Checks if the entered groupname exists, if does it asks for another
	// converts eveything to lowercase to avoid conflicts.

	var res bool
	var eid string
	sqlStatement := `SELECT EXISTS (SELECT entity_id FROM guacamole_entity 
		WHERE name = $1 AND type = 'USER_GROUP')`
	reader := bufio.NewReader(os.Stdin)

label:
	fmt.Println("Enter the name of the user group :")
	groupName, _ := reader.ReadString('\n')
	groupName = strings.ToLower(groupName)
	groupName = strings.Replace(groupName, "\n", "", -1)

	err := db.QueryRow(sqlStatement, groupName).Scan(&res)
	checkErr(err)
	if res {
		fmt.Println("Group already exists. Try a new group name")
		goto label
	} else {
		fmt.Println("Creating user group", groupName)
		sqlInsert := `
			INSERT INTO guacamole_entity (name, type)
			VALUES ($1, $2)`
		_, err = db.Exec(sqlInsert, groupName, "USER_GROUP")
		checkErr(err)
		err = db.QueryRow(`SELECT entity_id FROM guacamole_entity 
			WHERE name = $1 AND type = 'USER_GROUP'`,
			groupName).Scan(&eid)
		checkErr(err)
		sqlInsert = `
			INSERT INTO guacamole_user_group (entity_id, disabled)
			VALUES ($1, $2)`
		_, err = db.Exec(sqlInsert, eid, "FALSE")
		checkErr(err)
	}
	fmt.Println("Created", groupName)
	return true
}

func createConnectionGroup() bool {

	// Checks if the entered parent group name exists, if do goes to next line
	// converts eveything to lowercase to avoid conflicts.
	// format expected : parentGroup,childGroup,childGroup...

	var line int = 1
	var res bool
	var connectionGroupName string
	var parentID int
	csvfile, err := os.Open("connectiongroups.csv")
	if err != nil {
		log.Fatalln("Could not open the csv file 'users.csv'!", err)
	}
	r := csv.NewReader(csvfile)
	for {
		line++
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		sqlStatement := `SELECT EXISTS (SELECT connection_group_id 
			FROM guacamole_connection_group 
			WHERE connection_group_name = $1)`
		connectionGroupName = strings.ToLower(record[0])

		// connectionGroupName = strings.Replace(connectionGroupName, "\n", "", -1)

		err = db.QueryRow(sqlStatement, connectionGroupName).Scan(&res)
		checkErr(err)
		if res {
			fmt.Println("Connection group already exists. Skipping")
		} else {
			if len(record) < 1 {
				fmt.Println("Not enough fields near line", line)
			} else if len(record) == 1 {

				// Just root group, check if exists, if not, add to root

				fmt.Println("Creating connection group", connectionGroupName)
				sqlInsert := `
						INSERT INTO guacamole_connection_group 
						(connection_group_name, type, enable_session_affinity)
						VALUES ($1, 'ORGANIZATIONAL', 'f')
						`
				_, err = db.Exec(sqlInsert, connectionGroupName)
				checkErr(err)

				fmt.Println("Created", connectionGroupName)
			} else {

				// Creating and then Getting the connection group ID

				fmt.Println("Creating connection group", connectionGroupName)
				sqlInsert := `
						INSERT INTO guacamole_connection_group 
						(connection_group_name, type, enable_session_affinity)
						VALUES ($1, 'ORGANIZATIONAL', 'f')
						`
				_, err = db.Exec(sqlInsert, connectionGroupName)
				checkErr(err)

				err = db.QueryRow(`SELECT connection_group_id FROM guacamole_connection_group
				WHERE connection_group_name = $1`,
					connectionGroupName).Scan(&parentID)
				checkErr(err)

				// Handling children connection groups part
				for j := 1; j < len(record)-1; j++ {

					sqlStatement := `SELECT EXISTS (SELECT connection_group_id 
						FROM guacamole_connection_group 
						WHERE connection_group_name = $1)`
					connectionGroupName = strings.ToLower(record[j])
					// connectionGroupName = strings.Replace(connectionGroupName, "\n", "", -1)

					err := db.QueryRow(sqlStatement, connectionGroupName).Scan(&res)
					checkErr(err)
					if res {
						fmt.Println("Connection group already exists. Skipping")
					} else {
						fmt.Println("Creating connection group", connectionGroupName)
						sqlInsert := `
							INSERT INTO guacamole_connection_group 
							(parent_id, connection_group_name, type, enable_session_affinity)
							VALUES ($1, $2, 'ORGANIZATIONAL', 'f')
							`
						_, err = db.Exec(sqlInsert, parentID, connectionGroupName)
						checkErr(err)
					}
					fmt.Println("Created", connectionGroupName)
				}
			}

		}
	}
	return true
}

func createUser() bool {

	var line int = 0
	var res bool
	var ueid string
	var geid string
	var gid string
	checkUser := `SELECT EXISTS (SELECT entity_id FROM guacamole_entity 
		WHERE name = $1 AND type = 'USER')`
	checkGroup := `SELECT EXISTS (SELECT entity_id FROM guacamole_entity 
		WHERE name = $1 AND type = 'USER_GROUP')`
	sqlInsertEntity := `
	INSERT INTO guacamole_entity (name, type) VALUES
	($1, 'USER')
	`
	sqlInsertUser := `
	INSERT INTO guacamole_user (entity_id, password_salt, password_hash, password_date, disabled, expired) 
	VALUES ($1, '\x243a726da31d5a2663b5c5941fbdf8ec4903f389901a1e8ec80fcdf4732b0296', 
	'\x3ead2abe1ca76548fba39f7a96c66444bf1cb6d4ee510d132d55329afad8d15f', '2020-01-01 :00:00.001+05:30', 'f', 'f')
	`
	sqlInsertUserGroup := `
	INSERT INTO guacamole_user_group_member (user_group_id, member_entity_id) 
	VALUES ($1, $2)
	`

	// To insert a user, it expects : name of user,group(s) which they belong to,
	// all comma seperated. Currently limiting the number of groups a user can
	// belong to to 20.

	csvfile, err := os.Open("users.csv")
	if err != nil {
		log.Fatalln("Could not open the csv file 'users.csv'!", err)
	}
	r := csv.NewReader(csvfile)
	for {
		line++
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if len(record) < 1 {
			fmt.Println("Not enough fields near line", line)
			goto endOfFunc
		} else if len(record) <= 20 {
			userName := strings.ToLower(record[0])
			fmt.Println("User", record[0], "is going to be added to", len(record)-1, "groups")
			err = db.QueryRow(checkUser, userName).Scan(&res)
			checkErr(err)
			if res {
				fmt.Println("User", userName, "already exists. Trying the next one in the list")
			} else {
				_, err = db.Exec(sqlInsertEntity, userName)
				checkErr(err)
				err = db.QueryRow(`SELECT entity_id FROM guacamole_entity 
				WHERE name = $1 AND type = 'USER'`, userName).Scan(&ueid)
				checkErr(err)
				_, err = db.Exec(sqlInsertUser, ueid)
				checkErr(err)

				for i := 1; i < len(record); i++ {
					userGroup := strings.ToLower(record[i])
					err = db.QueryRow(checkGroup, userGroup).Scan(&res)
					checkErr(err)
					if res {
						err = db.QueryRow(`SELECT entity_id FROM guacamole_entity 
						WHERE name = $1 AND type = 'USER_GROUP'`, userGroup).Scan(&geid)
						checkErr(err)
						err = db.QueryRow(`SELECT user_group_id FROM guacamole_user_group 
						WHERE entity_id = $1`, geid).Scan(&gid)
						checkErr(err)
						_, err = db.Exec(sqlInsertUserGroup, gid, ueid)
						checkErr(err)
						fmt.Println(userName, " added to the group ", userGroup)

					} else {
						fmt.Println("Group ", userGroup, "does not exist. Adding user to root.")
					}
				}
				fmt.Println("User created", userName)
			}
		}
	endOfFunc:
	}
	return true
}

func createConnection() bool {

	checkConnection := `SELECT EXISTS (SELECT connection_id FROM guacamole_connection 
		WHERE connection_name = $1)`
	checkConnectionGroup := `SELECT EXISTS (SELECT connection_group_id 
		FROM guacamole_connection_group WHERE connection_group_name = $1)`
	var id string
	var res bool
	var cgid string

	// Just one csv
	// SSH expects :
	// protocol,name,hostname,username,port,password,connectionGroupName
	// VNC expects:
	// protocol,name,hostname,port,password,connectionGroupName
	// RDP expects :
	// protocol,name,hostname,port,username,password,connectionGroupName

	sqlInsertParameter := `
		INSERT INTO guacamole_connection_parameter (connection_id, parameter_name, parameter_value)
		VALUES ($1, $2, $3)`

	csvfile, err := os.Open("connections.csv")
	if err != nil {
		log.Fatalln("Could not open the csv file 'connections.csv'!", err)
	}
	r := csv.NewReader(csvfile)
	for {
		cgid = ""
		record, err := r.Read()
		if err == io.EOF {
			break
		} else {
			switch record[0] {

			case "ssh":

				// If connection is SSH, checking number of fields

				if len(record) > 7 {
					fmt.Println("More fields than expected present near", record[1])
					goto endOfSwitch
				} else if len(record) < 6 {
					fmt.Println("Less fields present than expected near", record[1])
					goto endOfSwitch
				}

				if len(record) == 7 {
					err = db.QueryRow(checkConnectionGroup, record[6]).Scan(&res)
					checkErr(err)
					if res {
						err = db.QueryRow(`SELECT connection_group_id FROM guacamole_connection_group 
						WHERE connection_group_name=$1`, record[6]).Scan(&cgid)
						checkErr(err)
					} else {
						fmt.Println("There is no connection group named", record[6], ". Adding connection to root group")
					}
				} else {
					fmt.Println("No connection group specified, adding to root")
				}

				err = db.QueryRow(checkConnection, record[1]).Scan(&res)
				checkErr(err)
				if res {
					fmt.Println("A connection with that name", record[1], "already exists. Trying the next one in the list")
				} else {
					if cgid == "" {
						sqlInsertConnection := `
							INSERT INTO guacamole_connection (connection_name, protocol, failover_only)
							VALUES ($1, $2, $3)`
						_, err = db.Exec(sqlInsertConnection, record[1], "ssh", "f")
						checkErr(err)
					} else {
						sqlInsertConnection := `
							INSERT INTO guacamole_connection (connection_name, parent_id, protocol, failover_only)
							VALUES ($1, $2, $3, $4)`
						_, err = db.Exec(sqlInsertConnection, record[1], cgid, "ssh", "f")
						checkErr(err)
					}
					err = db.QueryRow("SELECT connection_id FROM guacamole_connection WHERE connection_name=$1",
						record[1]).Scan(&id)
					checkErr(err)
					_, err = db.Exec(sqlInsertParameter, id, "hostname", record[2])
					checkErr(err)
					_, err = db.Exec(sqlInsertParameter, id, "username", record[3])
					checkErr(err)
					_, err = db.Exec(sqlInsertParameter, id, "port", record[4])
					checkErr(err)
					_, err = db.Exec(sqlInsertParameter, id, "password", record[5])
					checkErr(err)
					fmt.Println("SSH connection", record[1], "added")
				}

			// VNC

			case "vnc":

				if len(record) > 6 {
					fmt.Println("More fields than expected present near", record[1])
					goto endOfSwitch
				} else if len(record) < 5 {
					fmt.Println("Less fields present than expected near", record[1])
					goto endOfSwitch
				}
				if len(record) == 6 {
					err = db.QueryRow(checkConnectionGroup, record[5]).Scan(&res)
					checkErr(err)
					if res {
						err = db.QueryRow(`SELECT connection_group_id FROM guacamole_connection_group 
						WHERE connection_group_name=$1`, record[5]).Scan(&cgid)
						checkErr(err)
					} else {
						fmt.Println("There is no connection group named", record[5], ". Adding connection to root group")
					}
				} else {
					fmt.Println("No connection group specified, adding to root")
				}

				err = db.QueryRow(checkConnection, record[1]).Scan(&res)
				checkErr(err)
				if res {
					fmt.Println("A connection with the name", record[1], "already exists. Trying the next one in the list")
				} else {

					if cgid == "" {
						sqlInsertConnection := `
							INSERT INTO guacamole_connection (connection_name, protocol, failover_only)
							VALUES ($1, $2, $3)`
						_, err = db.Exec(sqlInsertConnection, record[1], "vnc", "f")
						checkErr(err)
					} else {
						sqlInsertConnection := `
							INSERT INTO guacamole_connection (connection_name, parent_id, protocol, failover_only)
							VALUES ($1, $2, $3, $4)`
						_, err = db.Exec(sqlInsertConnection, record[1], cgid, "vnc", "f")
						checkErr(err)
					}
					err = db.QueryRow("SELECT connection_id FROM guacamole_connection WHERE connection_name=$1",
						record[1]).Scan(&id)
					checkErr(err)
					_, err = db.Exec(sqlInsertParameter, id, "hostname", record[2])
					checkErr(err)
					_, err = db.Exec(sqlInsertParameter, id, "port", record[3])
					checkErr(err)
					_, err = db.Exec(sqlInsertParameter, id, "password", record[4])
					checkErr(err)
					fmt.Println("VNC connection", record[1], "added")
				}

			// RDP

			case "rdp":

				if len(record) > 7 {
					fmt.Println("More fields than expected present near", record[1])
					goto endOfSwitch
				} else if len(record) < 6 {
					fmt.Println("Less fields present than expected near", record[1])
					goto endOfSwitch
				}
				if len(record) == 7 {
					err = db.QueryRow(checkConnectionGroup, record[6]).Scan(&res)
					checkErr(err)
					if res {
						err = db.QueryRow(`SELECT connection_group_id FROM guacamole_connection_group 
						WHERE connection_group_name=$1`, record[6]).Scan(&cgid)
						checkErr(err)
					} else {
						fmt.Println("There is no connection group named", record[6], ". Adding connection to root group")
					}
				} else {
					fmt.Println("No connection group specified, adding to root")
				}
				err = db.QueryRow(checkConnection, record[1]).Scan(&res)
				checkErr(err)
				if res {
					fmt.Println("A connection with that name", record[1], "already exists. Trying the next one in the list")
				} else {
					if cgid == "" {
						sqlInsertConnection := `
							INSERT INTO guacamole_connection (connection_name, protocol, failover_only)
							VALUES ($1, $2, $3)`
						_, err = db.Exec(sqlInsertConnection, record[1], "rdp", "f")
						checkErr(err)
					} else {
						sqlInsertConnection := `
							INSERT INTO guacamole_connection (connection_name, parent_id, protocol, failover_only)
							VALUES ($1, $2, $3, $4)`
						_, err = db.Exec(sqlInsertConnection, record[1], cgid, "rdp", "f")
						checkErr(err)
					}
					err = db.QueryRow("SELECT connection_id FROM guacamole_connection WHERE connection_name=$1",
						record[1]).Scan(&id)
					checkErr(err)
					_, err = db.Exec(sqlInsertParameter, id, "hostname", record[2])
					checkErr(err)
					_, err = db.Exec(sqlInsertParameter, id, "port", record[3])
					checkErr(err)
					_, err = db.Exec(sqlInsertParameter, id, "username", record[4])
					checkErr(err)
					_, err = db.Exec(sqlInsertParameter, id, "password", record[5])
					checkErr(err)
					fmt.Println("RDP connection", record[1], "added")
				}

			case "Exit":
				break

			default:
				fmt.Println("Please check the file, there is some error near : ", record[0])
				listActions()
			}
		endOfSwitch:
		}
	}
	return true
}

func mapUserstoGroups() bool {

	// function expects following contents in usermapping.csv
	// UserentityType,UserentityName,ConnectionEntityType,ConnectionEntityName

	checkConnection := `SELECT EXISTS (SELECT connection_id FROM guacamole_connection 
		WHERE connection_name = $1)`
	checkConnectionGroup := `SELECT EXISTS (SELECT connection_group_id 
		FROM guacamole_connection_group WHERE connection_group_name = $1)`
	checkUser := `SELECT EXISTS (SELECT entity_id FROM guacamole_entity 
		WHERE name = $1 AND type = 'USER')`
	checkGroup := `SELECT EXISTS (SELECT entity_id FROM guacamole_entity 
		WHERE name = $1 AND type = 'USER_GROUP')`

	var res bool
	var eid int
	var cid int
	var gid int
	var cgid int

	csvfile, err := os.Open("usermapping.csv")
	if err != nil {
		log.Fatalln("Could not open the csv file 'connections.csv'!", err)
	}
	r := csv.NewReader(csvfile)
	line := 0
	for {

		// Initialize the line number variable
		line++
		record, err := r.Read()
		if err == io.EOF {
			break
		} else {

			if len(record) < 4 {
				fmt.Println("Not enough fields near line :", line)
			} else if len(record)%4 != 0 {
				fmt.Println("Wrong number of fields near line :", line)
			} else {

				for i := 0; i < len(record); i = i + 4 {

					userEntityType := strings.ToLower(record[i+1])
					userEntityName := strings.ToLower(record[i])
					connectionEntityType := strings.ToLower(record[i+3])
					connectionEntityName := strings.ToLower(record[i+2])

					// add user-connection mapping If its just a connection, check if the connection
					// is a part of any group ? if it is then give access to the group also. It  has
					// to be recursive. And so ... Writing a seperate function that takes, the name
					// of a connection. Since the name of a connection will be unique (that's how
					// I have done it for now). A parent, whether a connection or a group, it has to
					// be a group, becuase only groups can have children. So I can write a function
					// that checks for a parent inside the guacamole_connection_group table, recurs-
					// ively.

					switch userEntityType {

					// Handling entity type : USER

					case "user":

						userEntityName := strings.ToLower(userEntityName)
						err = db.QueryRow(checkUser, userEntityName).Scan(&res)
						checkErr(err)
						if res {
							err = db.QueryRow(`SELECT entity_id FROM guacamole_entity 
								WHERE type='USER' AND name=$1`, userEntityName).Scan(&eid)
							switch connectionEntityType {

							case "connection":

								// Mapping a USER to a CONNECTION

								err = db.QueryRow(checkConnection, connectionEntityName).Scan(&res)
								checkErr(err)
								if res {
									err = db.QueryRow(`SELECT connection_id FROM guacamole_connection
										WHERE connection_name=$1`, connectionEntityName).Scan(&cid)
									checkErr(err)
									err = db.QueryRow(`SELECT EXISTS(SELECT * FROM guacamole_connection 
										WHERE parent_id IS NOT NULL AND connection_id=$1);`, cid).Scan(&res)
									checkErr(err)
									if res {
										sqlInsertConnectionPermission := `
											INSERT INTO guacamole_connection_permission (entity_id, connection_id, permission)
											VALUES ($1, $2, 'READ')`
										res = checkExistsConnectionPermission(eid, cid)
										checkErr(err)
										if res {
											fmt.Println("That mapping already exists between", userEntityName, "and", connectionEntityName)
										} else {
											_, err = db.Exec(sqlInsertConnectionPermission, eid, cid)
											checkErr(err)
										}
										fmt.Println("Parent group (s) found for", connectionEntityName)
										err = db.QueryRow(`SELECT parent_id FROM guacamole_connection
											WHERE connection_name=$1`, connectionEntityName).Scan(&gid)
										checkErr(err)
										mapToParentConnectionGroup(eid, gid, 1)
									} else {
										sqlInsertConnectionPermission := `
											INSERT INTO guacamole_connection_permission (entity_id, connection_id, permission)
											VALUES ($1, $2, 'READ')`
										err = db.QueryRow(`SELECT EXISTS(SELECT * FROM guacamole_connection_permission
											WHERE entity_id=$1 AND connection_id=$2);`, eid, cid).Scan(&res)
										checkErr(err)
										if res {
											fmt.Println("That mapping already exists between", userEntityName, "and", connectionEntityName)
										} else {
											_, err = db.Exec(sqlInsertConnectionPermission, eid, cid)
											checkErr(err)
										}
									}
								} else {
									fmt.Println("Cannot map", userEntityName, "to", connectionEntityName, "on line :",
										line, "because there is no connection named '", connectionEntityName, "'. Skipping.")
								}

							case "connectiongroup":

								// Mapping USER to CONNECTION GROUP

								err = db.QueryRow(checkConnectionGroup, connectionEntityName).Scan(&res)
								checkErr(err)
								if res {

									// Map user to groups from the function : Need getting group ID, already have eid

									err = db.QueryRow(`SELECT connection_group_id FROM guacamole_connection_group
										WHERE connection_group_name=$1`, connectionEntityName).Scan(&cgid)
									checkErr(err)

									// Checking if there exists a mapping eid - cgid

									res = checkExistsConnectionGroupPermission(eid, cgid)
									if res {
										fmt.Println("That mapping already exists between", userEntityName, "and", connectionEntityName)
									} else {
										sqlInsertConnectionGroupPermission := `
											INSERT INTO guacamole_connection_group_permission (entity_id, connection_group_id, permission)
											VALUES ($1, $2, 'READ')`
										_, err = db.Exec(sqlInsertConnectionGroupPermission, eid, cgid)
										checkErr(err)

										// Checks if the connection group has a parent, if does mapToParentConnectionGroup is also called
										// else does nothing.

										err = db.QueryRow(`SELECT EXISTS(SELECT * FROM guacamole_connection_group
											WHERE parent_id IS NOT NULL AND connection_group_id=$1);`, cgid).Scan(&res)
										checkErr(err)
										if res {
											mapToParentConnectionGroup(eid, cgid, 1)
											fmt.Println("Done mapping", userEntityName, "to", connectionEntityName, "and its parent connection group(s)")
										} else {
											fmt.Println("No parent groups for", connectionEntityName, "going into mapping connections")
										}
									}

									// Mapping to the immediate connection group - DONE
									// Mapping to parent connection groups - DONE

									// Now this part makes sure that the user is given access to :

									// Mapping to Member connections -
									// Mapping to Member groups and it's successive connections

									// addToChildren should recursively check for children connections of immediate group
									// and add permission to each of them as well as it should call itself for
									// and it should also call
									addToChildren(eid, cgid, 1)

								} else {
									fmt.Println("Cannot map", userEntityName, "to", connectionEntityName, "on line :",
										line, "because there is no connection group named '",
										connectionEntityName, "'. Skipping.")
								}
							}
						} else {
							fmt.Println("Cannot map. User", userEntityName, "Does not exist. On line :", line, "skipping.")
						}

					// Handling entity type : USER GROUPS

					case "usergroup":

						userEntityName := strings.ToLower(userEntityName)
						err = db.QueryRow(checkGroup, userEntityName).Scan(&res)
						checkErr(err)
						if res {
							err = db.QueryRow(`SELECT entity_id FROM guacamole_entity 
								WHERE type='USER_GROUP' AND name=$1`, userEntityName).Scan(&eid)
							switch connectionEntityType {

							case "connection":

								// Mapping a USER to a CONNECTION

								err = db.QueryRow(checkConnection, connectionEntityName).Scan(&res)
								checkErr(err)
								if res {
									err = db.QueryRow(`SELECT connection_id FROM guacamole_connection
										WHERE connection_name=$1`, connectionEntityName).Scan(&cid)
									checkErr(err)
									err = db.QueryRow(`SELECT EXISTS(SELECT * FROM guacamole_connection 
										WHERE parent_id IS NOT NULL AND connection_id=$1);`, cid).Scan(&res)
									checkErr(err)
									if res {
										sqlInsertConnectionPermission := `
											INSERT INTO guacamole_connection_permission (entity_id, connection_id, permission)
											VALUES ($1, $2, 'READ')`
										res = checkExistsConnectionPermission(eid, cid)
										checkErr(err)
										if res {
											fmt.Println("That mapping already exists between", userEntityName, "and", connectionEntityName)
										} else {
											_, err = db.Exec(sqlInsertConnectionPermission, eid, cid)
											checkErr(err)
										}
										fmt.Println("Parent group (s) found for", connectionEntityName)
										err = db.QueryRow(`SELECT parent_id FROM guacamole_connection
											WHERE connection_name=$1`, connectionEntityName).Scan(&gid)
										checkErr(err)
										mapToParentConnectionGroup(eid, gid, 1)
									} else {
										sqlInsertConnectionPermission := `
											INSERT INTO guacamole_connection_permission (entity_id, connection_id, permission)
											VALUES ($1, $2, 'READ')`
										err = db.QueryRow(`SELECT EXISTS(SELECT * FROM guacamole_connection_permission
											WHERE entity_id=$1 AND connection_id=$2);`, eid, cid).Scan(&res)
										checkErr(err)
										if res {
											fmt.Println("That mapping already exists between", userEntityName, "and", connectionEntityName)
										} else {
											_, err = db.Exec(sqlInsertConnectionPermission, eid, cid)
											checkErr(err)
										}
									}
								} else {
									fmt.Println("Cannot map", userEntityName, "to", connectionEntityName, "on line :",
										line, "because there is no connection named '", connectionEntityName, "'. Skipping.")
								}

							case "connectiongroup":

								// Mapping USER to CONNECTION GROUP

								err = db.QueryRow(checkConnectionGroup, connectionEntityName).Scan(&res)
								checkErr(err)
								if res {

									// Map user to groups from the function : Need getting group ID, already have eid

									err = db.QueryRow(`SELECT connection_group_id FROM guacamole_connection_group
										WHERE connection_group_name=$1`, connectionEntityName).Scan(&cgid)
									checkErr(err)

									// Checking if there exists a mapping eid - cgid

									res = checkExistsConnectionGroupPermission(eid, cgid)
									if res {
										fmt.Println("That mapping already exists between", userEntityName, "and", connectionEntityName)
									} else {
										sqlInsertConnectionGroupPermission := `
											INSERT INTO guacamole_connection_group_permission (entity_id, connection_group_id, permission)
											VALUES ($1, $2, 'READ')`
										_, err = db.Exec(sqlInsertConnectionGroupPermission, eid, cgid)
										checkErr(err)

										// Checks if the connection group has a parent, if does mapToParentConnectionGroup is also called
										// else does nothing.

										err = db.QueryRow(`SELECT EXISTS(SELECT * FROM guacamole_connection_group
											WHERE parent_id IS NOT NULL AND connection_group_id=$1);`, cgid).Scan(&res)
										checkErr(err)
										if res {
											mapToParentConnectionGroup(eid, cgid, 1)
											fmt.Println("Done mapping", userEntityName, "to", connectionEntityName, "and its parent connection group(s)")
										} else {
											fmt.Println("No parent groups for", connectionEntityName, "going into mapping connections")
										}
									}

									// Mapping to the immediate connection group - DONE
									// Mapping to parent connection groups - DONE

									// Now this part makes sure that the user is given access to :

									// Mapping to Member connections -
									// Mapping to Member groups and it's successive connections

									// addToChildren should recursively check for children connections of immediate group
									// and add permission to each of them as well as it should call itself for
									// and it should also call
									addToChildren(eid, cgid, 1)

								} else {
									fmt.Println("Cannot map", userEntityName, "to", connectionEntityName, "on line :",
										line, "because there is no connection group named '",
										connectionEntityName, "'. Skipping.")
								}
							}
						} else {
							fmt.Println("Cannot map. User", userEntityName, "Does not exist. On line :", line, "skipping.")
						}

					default:
						fmt.Println("Please check the file, there is some error near line:", line, "continuing")
					}
				}
			}
		}
	}
	return true
}

func mapToParentConnectionGroup(eid int, gid int, i int) bool {
	var res bool
	err := db.QueryRow(`SELECT EXISTS(SELECT * FROM guacamole_connection_group_permission
		WHERE entity_id=$1 AND connection_group_id=$2);`, eid, gid).Scan(&res)
	sqlInsertConnectionGroupPermission := `
	INSERT INTO guacamole_connection_group_permission (entity_id, connection_group_id, permission)
	VALUES ($1, $2, 'READ')`
	checkErr(err)
	if res {
		fmt.Println("That mapping already exists between entity :", eid, "and parent :", gid)
	} else {
		_, err = db.Exec(sqlInsertConnectionGroupPermission, eid, gid)
		checkErr(err)
		fmt.Println("checked for and added mapping for parent #", i)
	}
	checkErr(err)
	err = db.QueryRow(`SELECT EXISTS(SELECT * FROM guacamole_connection_group
		WHERE parent_id IS NOT NULL AND connection_group_id=$1);`, gid).Scan(&res)
	checkErr(err)
	if res {
		err = db.QueryRow(`SELECT parent_id FROM guacamole_connection_group
			WHERE connection_group_id=$1`, gid).Scan(&gid)
		checkErr(err)
		mapToParentConnectionGroup(eid, gid, i+1)
	}
	return true
}

func addToChildren(eid int, cgid int, i int) {
	var childConnections *sql.Rows
	var childConnectionGroups *sql.Rows
	var err error
	var res bool

	childConnections, err = db.Query(`SELECT connection_id FROM guacamole_connection WHERE parent_id=$1`, cgid)
	checkErr(err)
	defer childConnections.Close()
	childConnectionGroups, err = db.Query(`SELECT connection_group_id FROM guacamole_connection_group WHERE parent_id=$1`, cgid)
	checkErr(err)
	defer childConnectionGroups.Close()

	for childConnections.Next() {
		var connectionID int
		err = childConnections.Scan(&connectionID)
		checkErr(err)
		res = checkExistsConnectionPermission(eid, connectionID)
		if res {
			fmt.Println("mapping between entity", eid, "and connection", connectionID, "already exists")
		} else {
			sqlInsertConnectionPermission := `
				INSERT INTO guacamole_connection_permission (entity_id, connection_id, permission)
				VALUES ($1, $2, 'READ')`
			_, err = db.Exec(sqlInsertConnectionPermission, eid, connectionID)
			checkErr(err)
		}
	}

	for childConnectionGroups.Next() {
		var connectionGroupID int
		err = childConnectionGroups.Scan(&connectionGroupID)
		checkErr(err)
		res = checkExistsConnectionPermission(eid, connectionGroupID)
		if res {
			fmt.Println("mapping between entity", eid, "and connection", connectionGroupID, "already exists")
		} else {
			sqlInsertConnectionGroupPermission := `
				INSERT INTO guacamole_connection_group_permission (entity_id, connection_group_id, permission)
				VALUES ($1, $2, 'READ')`
			_, err = db.Exec(sqlInsertConnectionGroupPermission, eid, connectionGroupID)
			checkErr(err)
		}
		addToChildren(eid, connectionGroupID, i+1)
	}
	fmt.Println("Level", i, "complete")
}

func checkExistsConnectionPermission(eid int, cid int) bool {
	var res bool
	err := db.QueryRow(`SELECT EXISTS(SELECT * FROM guacamole_connection_permission
		WHERE entity_id=$1 AND connection_id=$2);`, eid, cid).Scan(&res)
	checkErr(err)
	return res
}

func checkExistsConnectionGroupPermission(eid int, cgid int) bool {
	var res bool
	err := db.QueryRow(`SELECT EXISTS(SELECT * FROM guacamole_connection_group_permission
		WHERE entity_id=$1 AND connection_group_id=$2);`, eid, cgid).Scan(&res)
	checkErr(err)
	return res
}

// func createUserGroups() {

// 	fmt.Println("Create multiple user groups, sub groups, sub sub groups etc")

// }

// func createConnectionGroups() {

// 	fmt.Println("Create multiple connection groups, sub sub groups etc")

// }

// func deleteUserGroup() bool {
// 	// CAN WAIT
// 	var res bool
// 	return res
// }

// func deleteConnectionGroup() bool {
// 	// CAN WAIT
// 	var res bool
// 	return res
// }

// func deleteUser() bool {
// 	// CAN WAIT
// 	var res bool
// 	return res
// }

// func deleteConnection() bool {
// 	// CAN WAIT
// 	var res bool
// 	return res
// }

func main() {

	var res bool
	checkConnectivity()
	checkUserGroupStatus()
	checkConnectionGroupStatus()

loop:
	res = listActions()
	if res != true {
		println("Something went wrong")
	}
	goto loop

}

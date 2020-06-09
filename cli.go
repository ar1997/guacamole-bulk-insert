package main

import (
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
	port     = 55432
	user     = "guacamole_user"
	password = "pass"
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
	db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}
	err = db.Ping()
	if err != nil {
		log.Println("Error")
		panic(err)
	}
	log.Println("Connected")
}

func checkUserGroupStatus() {

	// This function checks if there exists at least a default user group.
	// If it does not exist, it creates one called "default" with just
	// read permission".

	var res bool
	err := db.QueryRow("SELECT EXISTS (SELECT user_group_id FROM guacamole_user_group)").Scan(&res)
	checkErr(err)

	if res != true {
		log.Println("There are no user groups present, please try adding some to 'users.csv'")
		res = createUserGroup()
	}
	log.Println("User groups are present")
}

func checkConnectionGroupStatus() {

	var res bool
	// This function checks if there exists at least a default
	// connection group. If it does not exist, it creates one.
	err := db.QueryRow("SELECT EXISTS (SELECT connection_group_id FROM guacamole_connection_group)").Scan(&res)
	checkErr(err)

	if res != true {
		log.Println("There are no connection groups present, please try adding some to 'connections.csv'")

	} else {
		log.Println("Connection groups are present")
	}
}

func listActions() bool {

	var res bool
	prompt := promptui.Select{
		Label: "What would you like to do ? ",
		Items: []string{"Add users and user groups from file", "Add connections and connection groups from file",
			"Map users to connections and groups from file", "Exit"},
		// "Delete a user", "Delete a connection", "Delete a user group", "Delete a connection group",
	}
	_, result, err := prompt.Run()
	if err != nil {
		log.Fatalf("Prompt failed %v\n", err)
	}
	switch result {

	case "Add users and user groups from file":
		res = createUser()

	case "Add connections and connection groups from file":
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
		fmt.Println("Bye ໒( •́ ‸ •̀ )७ ")
		os.Exit(0)

	default:
		log.Println("Input not defined ?")
		listActions()
	}
	return res
}

func createUserGroup() bool {

	// usergroup,groupname,parent1,parent2,parent3...

	var line int
	var res bool
	var ceid string // child entity id
	// Child user ID is never used, only child entity ID is used.
	var peid string  // parent entity id
	var pugid string // parent user group id

	checkIfGroupExists := `SELECT EXISTS (SELECT entity_id FROM guacamole_entity 
		WHERE name = $1 AND type = 'USER_GROUP')
		`
	sqlInsertEntity := `
		INSERT INTO guacamole_entity (name, type)
		VALUES ($1, $2)
		`
	sqlInsertGroup := `
		INSERT INTO guacamole_user_group (entity_id, disabled)
		VALUES ($1, $2)
		`
	sqlInsertMember := `
		INSERT INTO guacamole_user_group_member (user_group_id, member_entity_id)
		VALUES ($1, $2)
		`
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
		if record[0] == "usergroup" {
			if len(record) < 2 || len(record) > 20 {
				log.Println("Not enough or too many columns")
			} else {
				// If all conditions are met then do :
				childGroupName := strings.ToLower(record[1])
				// check if the group already exists
				err := db.QueryRow(checkIfGroupExists, childGroupName).Scan(&res)
				checkErr(err)
				if res {
					// Something fishy is going on here. :::: (-_-) ::::
					err = db.QueryRow(`SELECT entity_id FROM guacamole_entity
						WHERE name = $1 AND type = 'USER_GROUP'`, childGroupName).Scan(&ceid)
					checkErr(err)
					// Check if the given child-parent relation exists, if so do nothing
					// else, check if the parent exists - if parent exists, then add that
					// parent child relation , else add that parent, and add the relation.
					for i := 2; i < len(record); i++ {
						parentGroupName := strings.ToLower(record[i])
						err := db.QueryRow(checkIfGroupExists, parentGroupName).Scan(&res)
						checkErr(err)
						if res {
							// Since the parent exists, both parent and child exist, and now we can
							// check if the memebership exists already, if not insert it. (Get
							// entity ID of parent then get the group ID). Since members are mapped
							// as parent's group ID and its childrens' entity ID.
							err = db.QueryRow(`SELECT entity_id FROM guacamole_entity
							WHERE name = $1 AND type = 'USER_GROUP'`,
								parentGroupName).Scan(&peid)
							checkErr(err)
							err = db.QueryRow(`SELECT user_group_id FROM guacamole_user_group
							WHERE entity_id = $1`,
								peid).Scan(&pugid)
							checkErr(err)
							err = db.QueryRow(`SELECT EXISTS(SELECT user_group_id FROM guacamole_user_group_member
								WHERE user_group_id=$1 AND member_entity_id=$2)`, pugid, ceid).Scan(&res)
							checkErr(err)
							if res {
								log.Println("Line :", line, "- User group '", childGroupName, "' is already a child of '", parentGroupName, "'")
							} else {
								_, err = db.Exec(sqlInsertMember, pugid, ceid)
								checkErr(err)
								log.Println("Line :", line, "- User group", "'", childGroupName, "'", "is now a child of", "'", parentGroupName, "'")
							}
						} else {
							// if the parent does not exist, then create a parent get the eid of
							// the parent group, use that to get the parent's group ID and use that to
							// populate the table with parent's group ID and chilrens' entity ID.
							_, err = db.Exec(sqlInsertEntity, parentGroupName, "USER_GROUP")
							checkErr(err)
							err = db.QueryRow(`SELECT entity_id FROM guacamole_entity 
								WHERE name = $1 AND type = 'USER_GROUP'`,
								parentGroupName).Scan(&peid)
							checkErr(err)
							_, err = db.Exec(sqlInsertGroup, peid, "FALSE")
							checkErr(err)
							err = db.QueryRow(`SELECT user_group_id FROM guacamole_user_group
							WHERE entity_id = $1`,
								peid).Scan(&pugid)
							checkErr(err)
							_, err = db.Exec(sqlInsertMember, pugid, ceid)
							checkErr(err)
							log.Println("Line :", line, "- Parent user group", "'", parentGroupName, "'", "did not exist. Created it. and", "'",
								childGroupName, "'", "is now a child of it")
						}
					}

				} else {
					// The child user group does  not exist, then create the usergroup first :
					_, err = db.Exec(sqlInsertEntity, childGroupName, "USER_GROUP")
					checkErr(err)
					err = db.QueryRow(`SELECT entity_id FROM guacamole_entity 
						WHERE name = $1 AND type = 'USER_GROUP'`,
						childGroupName).Scan(&ceid)
					checkErr(err)
					_, err = db.Exec(sqlInsertGroup, ceid, "FALSE")
					checkErr(err)
					log.Println("Line :", line, "- Created group '", childGroupName, "'")
					// Checks for every parent of the group. from record[2] to length of the record - 1 right ?
					for i := 2; i < len(record); i++ {
						parentGroupName := strings.ToLower(record[i])
						err := db.QueryRow(checkIfGroupExists, parentGroupName).Scan(&res)
						checkErr(err)
						if res {
							// If the parent exist, then get its group ID, make relation between
							// parent and child.
							err = db.QueryRow(`SELECT entity_id FROM guacamole_entity
							WHERE name = $1 AND type = 'USER_GROUP'`,
								parentGroupName).Scan(&peid)
							checkErr(err)
							err = db.QueryRow(`SELECT user_group_id FROM guacamole_user_group
							WHERE entity_id = $1`,
								peid).Scan(&pugid)
							checkErr(err)
							_, err = db.Exec(sqlInsertMember, pugid, ceid)
							checkErr(err)
							log.Println("Line :", line, "- Parent user group", "'", parentGroupName, "'", "exists.",
								"'", childGroupName, "'", "and is now a child of it")
						} else {
							// if the parent does not exist, then create a parent, create a new
							// group, and then use that group as a parent.
							_, err = db.Exec(sqlInsertEntity, parentGroupName, "USER_GROUP")
							checkErr(err)
							err = db.QueryRow(`SELECT entity_id FROM guacamole_entity 
								WHERE name = $1 AND type = 'USER_GROUP'`,
								parentGroupName).Scan(&peid)
							checkErr(err)
							_, err = db.Exec(sqlInsertGroup, peid, "FALSE")
							checkErr(err)
							err = db.QueryRow(`SELECT user_group_id FROM guacamole_user_group
							WHERE entity_id = $1`,
								peid).Scan(&pugid)
							checkErr(err)
							_, err = db.Exec(sqlInsertMember, pugid, ceid)
							checkErr(err)
							log.Println("Line :", line, "- parent", "'", parentGroupName, "'", "did not exist, created it",
								"and", "' also created", childGroupName, "'", "and is now a child of it")
						}
					}
				}
			}
		}
	}
	return true
}

func createConnectionGroup() bool {

	var line int = 1
	csvfile, err := os.Open("connections.csv")
	if err != nil {
		log.Fatalln("Could not open the csv file 'connections.csv'!", err)
	}
	r := csv.NewReader(csvfile)
	for {
		line++
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		// For every record of the csv, check if the first column is connectiongroup
		if record[0] == "connectiongroup" {
			// If it is just two columns then check if the group exists, if not add
			// that group to the root
			if len(record) != 3 {
				// If the number of columns exceed 3 or is below 2, then should skip
				log.Println("Line :", line, "-", "Connection group : Too many or too less columns near line")
			} else {
				child := strings.ToLower(record[1])
				parent := strings.ToLower(record[2])
				var pid int
				var res bool
				sqlInsert := `
					INSERT INTO guacamole_connection_group
					(parent_id, connection_group_name, type, enable_session_affinity)
					VALUES ($1, $2, 'ORGANIZATIONAL', 'f')
					 `
				getParentID := `SELECT connection_group_id
					FROM guacamole_connection_group
					WHERE connection_group_name = $1`
				// Check if that given connection group is already present
				res = checkExistsConnectionGroup(child)
				if res {
					log.Println("Line :", line, "-", "The connection group", "'", child, "'", "exists. Skipping")
				} else {
					// If the connection group does not exist, then check if
					// the parent of the connection groups is present
					res = checkExistsConnectionGroup(parent)
					if res {
						// If both the group is not present and its parent are present
						// then simply get the group ID of the parent and insert the
						// connection group.
						err = db.QueryRow(getParentID, parent).Scan(&pid)
						checkErr(err)
						_, err = db.Exec(sqlInsert, pid, child)
						checkErr(err)
						log.Println("Line :", line, "-", "Parent connection group", "'", parent, "'",
							"found. Created child connection group", "'", child, "'")
					} else {
						// If the child does not exist and the parent does not, then
						// call the parent handling program which will return the ID
						// of the parent.
						pid, res = handleParentGroup(parent)
						_, err = db.Exec(sqlInsert, pid, child)
						checkErr(err)
						log.Println("Line :", line, "-",
							"Could not find the parent connection group. Parent(s) were created. Created child connection group",
							"'", child, "'")
					}
				}
			}
		} else {
			// Do nothing
		}
	}
	return true
}

func handleParentGroup(parent string) (int, bool) {

	// This function tries to find out if the parent group, which the other function
	// couldn't find in the database is supposed to be created, from the csv. It
	// searches for the parent, if it is present in the list, checks if that group's
	// parent is present, if it is, then it created this parent group with the grand-
	// parent (to the child) as its parent. If the parent does not exists, it does
	// the same thing for the grandparent. If no such entry exist, it creates the
	// the parent group at the root.

	var pid int = 0
	var line int = 1
	var res bool
	var state bool = false
	sqlInsert := `
		INSERT INTO guacamole_connection_group
		(parent_id, connection_group_name, type, enable_session_affinity)
		VALUES ($1, $2, 'ORGANIZATIONAL', 'f')
		 `
	sqlInsertRoot := `
	INSERT INTO guacamole_connection_group
	(connection_group_name, type, enable_session_affinity)
	VALUES ($1, 'ORGANIZATIONAL', 'f')
	 `
	getGroupID := `SELECT connection_group_id
		FROM guacamole_connection_group
		WHERE connection_group_name = $1`
	csvfile, err := os.Open("connections.csv")
	if err != nil {
		log.Fatalln("Could not open the csv file 'connections.csv'!", err)
	}
	r := csv.NewReader(csvfile)
	for {
		line++
		record, err := r.Read()
		if err == io.EOF {
			break
		}

		// Scan the whole connections.csv file for connectiongroup records, and if found then do :

		if record[0] == "connectiongroup" {
			if len(record) == 3 {
				if record[1] == parent {
					res = checkExistsConnectionGroup(record[2])

					if res {
						//  If there is an entry for the parent, then check if the grand parent exists
						res = checkExistsConnectionGroup(record[2])
						if res {

							// If grandparent exists, then get it's id, using that
							db.QueryRow(getGroupID, record[2]).Scan(&pid)
							_, err = db.Exec(sqlInsert, pid, parent)
							checkErr(err)
							db.QueryRow(getGroupID, parent).Scan(&pid)
							checkErr(err)
							log.Println("Line :", line, "-", "Parent connection group of", "'", parent, "'", "exitst with the name",
								record[2], ". Created the group")
							state = true
						} else {
							pid, state = handleParentGroup(record[2])
						}
					}
				}
			}
		}
	}
	if !state {
		_, err = db.Exec(sqlInsertRoot, parent)
		checkErr(err)
		db.QueryRow(getGroupID, parent).Scan(&pid)
		checkErr(err)
		state = true
		log.Println("Line :", line, "-", "No other scope for connection group", "'", parent, "'",
			". Created it at root.")
	}
	return pid, state
}

func createUser() bool {

	// Before anything else, create the user groups :
	createUserGroup()
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

	// To insert a user, it expects : user,<name of user>,<group(s) which they belong to>
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
		if record[0] != "user" {
			goto endOfFunc
		}
		if len(record) < 2 {
			log.Println("Line :", line, "-", "Not enough fields")
			goto endOfFunc
		} else if len(record) <= 20 {
			userName := strings.ToLower(record[1])
			err = db.QueryRow(checkUser, userName).Scan(&res)
			checkErr(err)
			if res {
				log.Println("Line :", line, "-", "User", "'", userName, "'", "already exists. Trying the next one in the list")
			} else {
				_, err = db.Exec(sqlInsertEntity, userName)
				checkErr(err)
				err = db.QueryRow(`SELECT entity_id FROM guacamole_entity 
				WHERE name = $1 AND type = 'USER'`, userName).Scan(&ueid)
				checkErr(err)
				_, err = db.Exec(sqlInsertUser, ueid)
				checkErr(err)

				if len(record) == 2 {
					// Do nothing
					log.Println("Line :", line, "-", "'", userName, "'", "- No parent user group specified. Adding user to root.")
				} else {
					log.Println("Line :", line, "-", "User", userName, "is going to be added to", len(record)-2, "groups")
					for i := 2; i < len(record); i++ {
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
							log.Println("Line :", line, "-", "'", userName, "'", " added to the group ", userGroup)
						} else {
							log.Println("Line :", line, "-", "User group", "'", userGroup, "'", "does not exist. Looking for the next group")
						}
					}
				}
				log.Println("Line :", line, "-", "User created", "'", userName, "'")
			}
		}
	endOfFunc:
	}
	return true
}

func createConnection() bool {

	// Create them connection groups first, then we'll talk about connections.
	createConnectionGroup()
	// If there is no connection groups you cannot pass from here
	checkConnection := `SELECT EXISTS (SELECT connection_id FROM guacamole_connection 
		WHERE connection_name = $1)`
	checkConnectionGroup := `SELECT EXISTS (SELECT connection_group_id 
		FROM guacamole_connection_group WHERE connection_group_name = $1)`
	var id string
	var res bool
	var cgid string
	var line int = 0

	// Just one csv for both connections and connection groups.
	// If you want to add a connection group :: connectiongroup,<name of connection group>,< name of parent group>
	// It's okay if you dont want to add a connection group, you can leave that column blank
	// If you leave that column blank, then it will get added to the root.
	// If there is no parent group with the name you specified, it will get created.
	// The numbers are just to indicate the postions of data. No numbers or spaces in the actual csv

	// SSH expects :

	// 0 connection, 1 protocol, 2 name, 3 hostname, 4 username, 5 port, 6 password, 7 connectionGroupName
	// Min 7 max 8

	// VNC expects:

	// 0 connection, 1 protocol, 2 name, 3 hostname, 4 port, 5 password, 6 connectionGroupName
	// Min 6 max 7

	// RDP expects :

	// 0 connection, 1 protocol, 2 name, 3 hostname, 4 port, 5 username, 6 password, 7 connectionGroupName
	// Min 7 max 8

	sqlInsertParameter := `
		INSERT INTO guacamole_connection_parameter (connection_id, parameter_name, parameter_value)
		VALUES ($1, $2, $3)
		`
	csvfile, err := os.Open("connections.csv")
	if err != nil {
		log.Fatalln("Could not open the csv file 'connections.csv'!", err)
	}
	r := csv.NewReader(csvfile)
	for {
		line++
		cgid = ""
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if record[0] != "connection" {
			goto endOfSwitch
		}
		switch record[1] {
		case "ssh":
			// If connection is SSH, checking number of fields
			name := record[2]
			host := record[3]
			user := record[4]
			port := record[5]
			pass := record[6]
			if len(record) > 8 {
				log.Println("Line :", line, "- Connection:", "More fields than expected present near", "'", name, "'")
				goto endOfSwitch
			} else if len(record) < 7 {
				log.Println("Line :", line, "- Connection:", "Less fields present than expected near", "'", name, "'")
				goto endOfSwitch
			}
			if len(record) == 8 {
				cgnm := record[7]
				err = db.QueryRow(checkConnectionGroup, cgnm).Scan(&res)
				checkErr(err)
				if res {
					err = db.QueryRow(`SELECT connection_group_id FROM guacamole_connection_group 
						WHERE connection_group_name=$1`, cgnm).Scan(&cgid)
					checkErr(err)
				} else {
					log.Println("Line :", line, "-", "There is no connection group named", "'", cgnm, "'", ". Adding connection to root group")
				}
			} else {
				log.Println("Line :", line, "- Connection:", "No connection group specified, adding to root")
			}
			err = db.QueryRow(checkConnection, name).Scan(&res)
			checkErr(err)
			if res {
				log.Println("Line :", line, "-", "A connection with the name", "'", name, "'", "already exists. Trying the next one in the list")
			} else {
				if cgid == "" {
					sqlInsertConnection := `
							INSERT INTO guacamole_connection (connection_name, protocol, failover_only)
							VALUES ($1, $2, $3)`
					_, err = db.Exec(sqlInsertConnection, name, "ssh", "f")
					checkErr(err)
				} else {
					sqlInsertConnection := `
							INSERT INTO guacamole_connection (connection_name, parent_id, protocol, failover_only)
							VALUES ($1, $2, $3, $4)`
					_, err = db.Exec(sqlInsertConnection, name, cgid, "ssh", "f")
					checkErr(err)
				}
				err = db.QueryRow("SELECT connection_id FROM guacamole_connection WHERE connection_name=$1",
					name).Scan(&id)
				checkErr(err)
				_, err = db.Exec(sqlInsertParameter, id, "hostname", host)
				checkErr(err)
				_, err = db.Exec(sqlInsertParameter, id, "username", user)
				checkErr(err)
				_, err = db.Exec(sqlInsertParameter, id, "port", port)
				checkErr(err)
				_, err = db.Exec(sqlInsertParameter, id, "password", pass)
				checkErr(err)
				log.Println("Line :", line, "-", "SSH connection", "'", name, "'", "added")
			}
		// VNC
		case "vnc":
			name := record[2]
			host := record[3]
			port := record[4]
			pass := record[5]
			if len(record) > 7 {
				log.Println("Line :", line, "-", "Connection: More fields than expected present near", "'", name, "'")
				goto endOfSwitch
			} else if len(record) < 6 {
				log.Println("Line :", line, "-", "Connection: Less fields present than expected near", "'", name, "'")
				goto endOfSwitch
			}
			if len(record) == 7 {
				cgnm := record[6]
				err = db.QueryRow(checkConnectionGroup, cgnm).Scan(&res)
				checkErr(err)
				if res {
					err = db.QueryRow(`SELECT connection_group_id FROM guacamole_connection_group 
						WHERE connection_group_name=$1`, cgnm).Scan(&cgid)
					checkErr(err)
				} else {
					log.Println("Line :", line, "-", "There is no connection group named", "'", cgnm, "'", ". Adding connection to root group")
				}
			} else {
				log.Println("Line :", line, "-", "No connection group specified, adding to root")
			}
			err = db.QueryRow(checkConnection, name).Scan(&res)
			checkErr(err)
			if res {
				log.Println("Line :", line, "-", "A connection with the name", "'", name, "'", "already exists. Trying the next one in the list")
			} else {
				if cgid == "" {
					sqlInsertConnection := `
							INSERT INTO guacamole_connection (connection_name, protocol, failover_only)
							VALUES ($1, $2, $3)`
					_, err = db.Exec(sqlInsertConnection, name, "vnc", "f")
					checkErr(err)
				} else {
					sqlInsertConnection := `
							INSERT INTO guacamole_connection (connection_name, parent_id, protocol, failover_only)
							VALUES ($1, $2, $3, $4)`
					_, err = db.Exec(sqlInsertConnection, name, cgid, "vnc", "f")
					checkErr(err)
				}
				err = db.QueryRow("SELECT connection_id FROM guacamole_connection WHERE connection_name=$1",
					name).Scan(&id)
				checkErr(err)
				_, err = db.Exec(sqlInsertParameter, id, "hostname", host)
				checkErr(err)
				_, err = db.Exec(sqlInsertParameter, id, "port", port)
				checkErr(err)
				_, err = db.Exec(sqlInsertParameter, id, "password", pass)
				checkErr(err)
				log.Println("Line :", line, "-", "VNC connection", "'", name, "'", "added")
			}
		// RDP
		case "rdp":
			name := record[2]
			host := record[3]
			user := record[4]
			port := record[5]
			pass := record[6]
			if len(record) > 8 {
				log.Println("Line :", line, "-", "More fields than expected present near", name)
				goto endOfSwitch
			} else if len(record) < 7 {
				log.Println("Line :", line, "-", "Less fields present than expected near", name)
				goto endOfSwitch
			}
			if len(record) == 8 {
				cgnm := record[7]
				err = db.QueryRow(checkConnectionGroup, cgnm).Scan(&res)
				checkErr(err)
				if res {
					err = db.QueryRow(`SELECT connection_group_id FROM guacamole_connection_group 
						WHERE connection_group_name=$1`, cgnm).Scan(&cgid)
					checkErr(err)
				} else {
					log.Println("Line :", line, "-", "There is no connection group named", cgnm, ". Adding connection to root group")
				}
			} else {
				log.Println("Line :", line, "-", "No connection group specified, adding to root")
			}
			err = db.QueryRow(checkConnection, name).Scan(&res)
			checkErr(err)
			if res {
				log.Println("Line :", line, "-", "A connection with that name", "'", name, "'", "already exists. Trying the next one in the list")
			} else {
				if cgid == "" {
					sqlInsertConnection := `
							INSERT INTO guacamole_connection (connection_name, protocol, failover_only)
							VALUES ($1, $2, $3)`
					_, err = db.Exec(sqlInsertConnection, name, "rdp", "f")
					checkErr(err)
				} else {
					sqlInsertConnection := `
							INSERT INTO guacamole_connection (connection_name, parent_id, protocol, failover_only)
							VALUES ($1, $2, $3, $4)`
					_, err = db.Exec(sqlInsertConnection, name, cgid, "rdp", "f")
					checkErr(err)
				}
				err = db.QueryRow("SELECT connection_id FROM guacamole_connection WHERE connection_name=$1",
					name).Scan(&id)
				checkErr(err)
				_, err = db.Exec(sqlInsertParameter, id, "hostname", host)
				checkErr(err)
				_, err = db.Exec(sqlInsertParameter, id, "port", port)
				checkErr(err)
				_, err = db.Exec(sqlInsertParameter, id, "username", user)
				checkErr(err)
				_, err = db.Exec(sqlInsertParameter, id, "password", pass)
				checkErr(err)
				log.Println("Line :", line, "-", "RDP connection", "'", name, "'", "added")
			}
		case "Exit":
			break
		default:
			log.Println("Line :", line, "- Connections:", "Please check the file, there is some error near : ", "'", record[1], "' , Skipping")
		}
	endOfSwitch:
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
	var line int

	csvfile, err := os.Open("usermapping.csv")
	if err != nil {
		log.Fatalln("Could not open the csv file 'connections.csv'!", err)
	}
	r := csv.NewReader(csvfile)
	line = 0
	for {

		// Initialize the line number variable
		line++
		record, err := r.Read()
		if err == io.EOF {
			break
		} else {

			if len(record) < 4 {
				log.Println("Line :", line, "-", "Not enough fields near line :")
			} else if len(record)%4 != 0 {
				log.Println("Line :", line, "-", "Wrong number of fields near line :")
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
											log.Println("Line :", line, "-", "That mapping already exists between user",
												"'", userEntityName, "'", "and connection", "'", connectionEntityName, "'")
										} else {
											_, err = db.Exec(sqlInsertConnectionPermission, eid, cid)
											checkErr(err)
										}
										err = db.QueryRow(`SELECT parent_id FROM guacamole_connection
											WHERE connection_name=$1`, connectionEntityName).Scan(&gid)
										checkErr(err)
										mapToParentConnectionGroup(eid, gid, 1)
										//log.Println("Line :", line, "-", "879 Mapping done for user", "'", userEntityName, "'  to connection",
										//	"'", connectionEntityName, "' And it's parents")
									} else {
										sqlInsertConnectionPermission := `
											INSERT INTO guacamole_connection_permission (entity_id, connection_id, permission)
											VALUES ($1, $2, 'READ')`
										err = db.QueryRow(`SELECT EXISTS(SELECT * FROM guacamole_connection_permission
											WHERE entity_id=$1 AND connection_id=$2);`, eid, cid).Scan(&res)
										checkErr(err)
										if res {
											log.Println("Line :", line, "-", "That mapping already exists between user", "'", userEntityName, "'",
												"and connection", "'", connectionEntityName, "'")
										} else {
											_, err = db.Exec(sqlInsertConnectionPermission, eid, cid)
											checkErr(err)
											log.Println("Line :", line, "-", "894 Mapping done for user", "'", userEntityName, "'  to connection",
												"'", connectionEntityName, "'")
										}
									}
								} else {
									log.Println("Line :", line, "-", "Cannot map user", "'", userEntityName, "'", "to connection", "'",
										connectionEntityName, "'", "on line :", line, "because there is no connection named '",
										"'", connectionEntityName, "'", "'. Skipping.")
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
										log.Println("Line :", line, "-", "That mapping already exists between", userEntityName, "and", connectionEntityName)
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
											log.Println("Line :", line, "-", "938 Mapping done for user", "'", userEntityName, "'  to connection group",
												"'", connectionEntityName, "'")
										} else {
											log.Println("Line :", line, "-", "No parent groups exists for", connectionEntityName,
												"mapping to child connections")
										}
									}

									// addToChildren should recursively check for children connections of immediate group
									// and add permission to each of them as well as it should call itself for
									// and it should also call

									addToChildren(eid, cgid, 1)

								} else {
									log.Println("Line :", line, "-", "Cannot map", userEntityName, "to", connectionEntityName, "on line :",
										line, "because there is no connection group named '",
										connectionEntityName, "'. Skipping.")
								}
							}
						} else {
							log.Println("Line :", line, "-", "Cannot map. User", userEntityName, "Does not exist. Skipping.")
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
											log.Println("Line :", line, "-", "That mapping already exists between user group", "'",
												userEntityName, "and connection", "'", connectionEntityName, "'")
										} else {
											_, err = db.Exec(sqlInsertConnectionPermission, eid, cid)
											checkErr(err)
										}
										err = db.QueryRow(`SELECT parent_id FROM guacamole_connection
											WHERE connection_name=$1`, connectionEntityName).Scan(&gid)
										checkErr(err)
										mapToParentConnectionGroup(eid, gid, 1)
										//log.Println("Line :", line, "-", "1004 Mapping done for user group", "'", userEntityName, "'  to",
										//	"connection '", connectionEntityName, "'")
									} else {
										sqlInsertConnectionPermission := `
											INSERT INTO guacamole_connection_permission (entity_id, connection_id, permission)
											VALUES ($1, $2, 'READ')`
										err = db.QueryRow(`SELECT EXISTS(SELECT * FROM guacamole_connection_permission
											WHERE entity_id=$1 AND connection_id=$2);`, eid, cid).Scan(&res)
										checkErr(err)
										if res {
											log.Println("Line :", line, "-", "'", "That mapping already exists between user group", "'",
												userEntityName, "'", "and connection", "'", connectionEntityName, "'")
										} else {
											_, err = db.Exec(sqlInsertConnectionPermission, eid, cid)
											checkErr(err)
											log.Println("Line :", line, "-", "1019 Mapping done for user group", "'", userEntityName, "'  to",
												"connection '", connectionEntityName, "'")
										}
									}
								} else {
									log.Println("Line :", line, "-", "Cannot map user group", "'", userEntityName, "'", "to connection",
										"'", connectionEntityName, "'", "because there is no connection named '", "'", connectionEntityName,
										"'", "'. Skipping.")
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
										log.Println("Line :", line, "-", "That mapping already exists between user group", "'",
											userEntityName, "'", "and connection group", "'", connectionEntityName, "'")
									} else {
										sqlInsertConnectionGroupPermission := `
											INSERT INTO guacamole_connection_group_permission (entity_id, connection_group_id, permission)
											VALUES ($1, $2, 'READ')`
										_, err = db.Exec(sqlInsertConnectionGroupPermission, eid, cgid)
										checkErr(err)
										log.Println("Line :", line, "-", "1055 Mapping done for user group", "'", userEntityName, "'  to",
											"connection group'", connectionEntityName, "'")
										// Checks if the connection group has a parent, if does mapToParentConnectionGroup is also called
										// else does nothing.

										err = db.QueryRow(`SELECT EXISTS(SELECT * FROM guacamole_connection_group
											WHERE parent_id IS NOT NULL AND connection_group_id=$1);`, cgid).Scan(&res)
										checkErr(err)
										if res {
											mapToParentConnectionGroup(eid, cgid, 1)
											log.Println("Line :", line, "-", "Done mapping user group", "'", userEntityName, "'", "to connection group",
												"'", connectionEntityName, "'", "and its parent connection group(s)")
										} else {
											log.Println("Line :", line, "-", "No parent groups for connection group", "'",
												connectionEntityName, "'", "going into mapping connections")
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
									log.Println("Line :", line, "-", "Cannot map", userEntityName, "to", connectionEntityName,
										"on line :", "because there is no connection group named '",
										connectionEntityName, "'. Skipping.")
								}
							}
						} else {
							log.Println("Line :", line, "-", "Cannot map user group", "'", userEntityName, "'", "because it does not exist. Skipping")
						}

					default:
						log.Println("Line :", line, "-", "Please check the file, there is some error near line. Continuing")
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
	checkErr(err)
	sqlInsertConnectionGroupPermission := `
	INSERT INTO guacamole_connection_group_permission (entity_id, connection_group_id, permission)
	VALUES ($1, $2, 'READ')`
	if res {
		// log.Println("That mapping already exists between entity :", "'", eid, "'", "and parent :", "'", gid, "'")
		// Do nothing here for now, this is creating a fuss in the output. Have to check what is causing this "effect"
		// Oh wth
	} else {
		_, err = db.Exec(sqlInsertConnectionGroupPermission, eid, gid)
		checkErr(err)
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
			// log.Println("Mapping between entity", "'", eid, "'", "and connection", "'", connectionID, "'", "already exists")
			// Same thing here
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
			// log.Println("mapping between entity", "'", eid, "'", "and connection", "'", connectionGroupID, "'", "already exists")
			// Same thing here.
		} else {
			sqlInsertConnectionGroupPermission := `
				INSERT INTO guacamole_connection_group_permission (entity_id, connection_group_id, permission)
				VALUES ($1, $2, 'READ')`
			_, err = db.Exec(sqlInsertConnectionGroupPermission, eid, connectionGroupID)
			checkErr(err)
		}
		addToChildren(eid, connectionGroupID, i+1)
	}
}

func checkExistsConnectionPermission(eid int, cid int) bool {
	var res bool
	err := db.QueryRow(`SELECT EXISTS(SELECT * FROM guacamole_connection_permission
		WHERE entity_id=$1 AND connection_id=$2);`, eid, cid).Scan(&res)
	checkErr(err)
	return res
}

func checkExistsConnectionGroup(connection string) bool {
	var res bool
	err := db.QueryRow(`SELECT EXISTS(SELECT * FROM guacamole_connection_group
		WHERE connection_group_name=$1);`, connection).Scan(&res)
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

func main() {

	var res bool
	checkConnectivity()

loop:
	res = listActions()
	if res != true {
		println("Something went wrong. Try again ?")
	}
	goto loop
	//defer db.Close()
}

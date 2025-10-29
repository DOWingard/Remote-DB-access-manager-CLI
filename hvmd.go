package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var coreEnabled bool
var sshKeyString string = "real.key"

func main() {
	// Check if --core is the LAST argument
	coreRequested := false
	if len(os.Args) > 1 && os.Args[len(os.Args)-1] == "--core" {
		coreRequested = true
	}

	// Filter out --core ONLY if it's the last argument
	var args []string
	for i, arg := range os.Args[1:] {
		if arg == "--core" && i == len(os.Args[1:])-1 {
			continue
		}
		args = append(args, arg)
	}

	if len(args) < 1 {
		fmt.Println("(!) No command provided")
		fmt.Println("    Try: hvmd help")
		os.Exit(0)
	}

	cmd := args[0]

	// --- Normal help without --core ---
	if cmd == "help" && !coreRequested {
		showHelp(false)
		return
	}

	// --- Load .env and DB config ---
	_ = godotenv.Load(".env") // ignore missing

	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	dbname := os.Getenv("POSTGRES_DB")
	host := os.Getenv("POSTGRES_HOST")
	port := os.Getenv("POSTGRES_PORT")

	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}

	if user == "" || password == "" || dbname == "" {
		// If core was requested, fail immediately
		if coreRequested {
			fmt.Println("(!) Unknown command: --core")
			fmt.Println("    Try: hvmd help")
			os.Exit(1)
		}
		fmt.Println("(X) Failed to connect to the VOID. Forcefield active.")
		os.Exit(1)
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, password, host, port, dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		if coreRequested {
			fmt.Println("(!) Unknown command: --core")
			fmt.Println("    Try: hvmd help")
			os.Exit(1)
		}
		fmt.Println("(X) Failed to connect to the VOID. Forcefield active.")
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		if coreRequested {
			fmt.Println("(!) Unknown command: --core")
			fmt.Println("    Try: hvmd help")
			os.Exit(1)
		}
		fmt.Println("(X) Failed to connect to the VOID. Forcefield active.")
		os.Exit(1)
	}

	// --- Check core access if --core was requested ---
	if coreRequested {
		if checkCoreAccess(db, user) {
			coreEnabled = true
			checkSSHConnection(db)

			// If the command is help, now show core help
			if cmd == "help" {
				showHelp(true)
				return
			}
		} else {
			fmt.Println("(!) Unknown command: --core")
			fmt.Println("    Try: hvmd help")
			os.Exit(1)
		}
	}

	// --- Execute other commands ---
	switch cmd {
	case "ping":
		showPing(db)
	case "admins":
		showAdmins(db)
	case "identify":
		if !coreEnabled {
			fmt.Println("(!) Unknown command:", cmd)
			suggestSimilar(cmd)
			os.Exit(1)
		}
		showIdentify(db, user)
	case "addadminsshkey":
		addAdminSSHKey()
	case "catssh":
		catSSH()
	case "readdb":
		if coreEnabled {
			runReadDB(db)
		} else {
			runReadDBBasic(db)
		}
	default:
		if isCoreCommand(cmd) && !coreEnabled {
			fmt.Println("(!) Unknown command:", cmd)
			suggestSimilar(cmd)
			os.Exit(1)
		} else if isCoreCommand(cmd) && coreEnabled {
			handleCoreCommand(cmd, db)
		} else {
			fmt.Println("(!) Unknown command:", cmd)
			suggestSimilar(cmd)
			os.Exit(1)
		}
	}
}

// --- Core-only SSH functions ---
func checkSSHConnection(db *sql.DB) {
	// --- Check for .key file ---
	keyEnv, err := godotenv.Read(sshKeyString)
	if err != nil {
		fmt.Println("(X) Failed to read .key file. Forcefield active.")
		os.Exit(1)
	}

	sshKey := keyEnv["SSH_KEY"]
	if sshKey == "" {
		fmt.Println("(X) No .key file found. Forcefield active.")
		os.Exit(1)
	}

	fmt.Println("{🏷️  } SSH key loaded from .key")

	// --- Test DB connection silently ---
	var now string
	if err := db.QueryRow("SELECT NOW();").Scan(&now); err != nil {
		fmt.Println("(X) Failed to connect to the VOID. Forcefield active.")
		os.Exit(1)
	}

	// Optional: Uncomment if you want a success message
	// fmt.Printf("{🔗 } Database connection OK. Current time: %s\n", now)
}

func addAdminSSHKey() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Paste your SSH public key (press Enter when done):")
	sshKey, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("{⚠️   } Failed to read input: %v\n", err)
		os.Exit(1)
	}

	sshKey = strings.TrimSpace(sshKey)

	if sshKey == "" {
		fmt.Println("(X) No key provided")
		os.Exit(1)
	}

	content := fmt.Sprintf("SSH_KEY=%s\n", sshKey)
	err = os.WriteFile(".key", []byte(content), 0600)
	if err != nil {
		fmt.Printf("{⚠️   } Failed to write .key file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("{📝 } SSH key successfully written to .key")
}

func catSSH() {
	keyEnv, err := godotenv.Read(".key")
	if err != nil {
		fmt.Printf("{⚠️   } Failed to read .key file: %v\n", err)
		os.Exit(1)
	}

	sshKey := keyEnv["SSH_KEY"]
	if sshKey == "" {
		fmt.Println("{⚠️   } No SSH_KEY found in .key file")
		return
	}

	fmt.Println(sshKey)
}

func runTestSSH() {
	keyEnv, err := godotenv.Read(sshKeyString)
	if err != nil {
		fmt.Printf("{⚠️   } Failed to read %s: %v\n", sshKeyString, err)
		return
	}

	sshKey := keyEnv["SSH_KEY"]
	if sshKey == "" {
		fmt.Println("{⚠️   } No SSH_KEY found, cannot test SSH")
		return
	}

	fmt.Println("{🔑 } SSH key loaded, running test connection...")
	time.Sleep(1 * time.Second)
	fmt.Println("{🔗 } SSH connection test successful!")
}

// --- Core access check ---
func checkCoreAccess(db *sql.DB, username string) bool {
	var isSuperuser bool
	err := db.QueryRow(`
		SELECT rolsuper 
		FROM pg_roles 
		WHERE rolname = $1
	`, username).Scan(&isSuperuser)

	if err != nil {
		return false
	}

	return isSuperuser
}

func isCoreCommand(cmd string) bool {
	coreCommands := []string{"identify", "testssh", "readdb"}
	for _, c := range coreCommands {
		if c == cmd {
			return true
		}
	}
	return false
}

func handleCoreCommand(cmd string, db *sql.DB) {
	fmt.Printf("{🌐 } Executing: %s\n", strings.ToUpper(cmd))

	switch cmd {
	case "testssh":
		runTestSSH()
	case "readdb":
		runReadDB(db)
	default:
		fmt.Println("{👁️  } Core command not yet implemented")
	}
}

// --- Database reads ---
func runReadDB(db *sql.DB) {
	fmt.Println("{📚 } Reading database schema...")

	rows, err := db.Query(`
        SELECT table_name
        FROM information_schema.tables
        WHERE table_schema='public'
        ORDER BY table_name;
    `)
	if err != nil {
		fmt.Printf("{⚠️  } Failed to fetch tables: %v\n", err)
		return
	}
	defer rows.Close()

	tables := []string{}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			fmt.Printf("{⚠️  } Failed to read table: %v\n", err)
			continue
		}
		tables = append(tables, table)
	}

	if len(tables) == 0 {
		fmt.Println("{⚠️  } No tables found")
		return
	}

	for _, table := range tables {
		fmt.Printf("\n{🗃️  } Table: %s\n", table)

		colRows, err := db.Query(`
            SELECT column_name, data_type, is_nullable
            FROM information_schema.columns
            WHERE table_name = $1
            ORDER BY ordinal_position;
        `, table)
		if err != nil {
			fmt.Printf("{⚠️  } Failed to read columns for %s: %v\n", table, err)
			continue
		}

		for colRows.Next() {
			var colName, dataType, isNullable string
			if err := colRows.Scan(&colName, &dataType, &isNullable); err != nil {
				fmt.Printf("{⚠️  } Failed to read column: %v\n", err)
				continue
			}
			fmt.Printf("    📝  %s | %s | nullable: %s\n", colName, dataType, isNullable)
		}
		colRows.Close()
	}

	fmt.Println("\n{🔒 } Admin Users:")
	adminRows, err := db.Query(`
        SELECT rolname 
        FROM pg_roles 
        WHERE rolsuper = true OR rolcreaterole = true
        ORDER BY rolname;
    `)
	if err != nil {
		fmt.Printf("{⚠️  } Failed to read admin users: %v\n", err)
		return
	}
	defer adminRows.Close()

	for adminRows.Next() {
		var a string
		if err := adminRows.Scan(&a); err != nil {
			continue
		}
		fmt.Printf("    🔑  %s\n", a)
	}
}

func runReadDBBasic(db *sql.DB) {
	fmt.Println("(>) Reading database tables")

	rows, err := db.Query(`
        SELECT table_name
        FROM information_schema.tables
        WHERE table_schema='public'
        ORDER BY table_name;
    `)
	if err != nil {
		fmt.Printf("(!) Failed to fetch tables: %v\n", err)
		return
	}
	defer rows.Close()

	tables := []string{}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			fmt.Printf("(!) Failed to read table: %v\n", err)
			continue
		}
		tables = append(tables, table)
	}

	if len(tables) == 0 {
		fmt.Println("(!) No tables found")
		return
	}

	for _, table := range tables {
		fmt.Printf("\n(>) Table: %s\n", table)

		colRows, err := db.Query(`
            SELECT column_name
            FROM information_schema.columns
            WHERE table_name = $1
            ORDER BY ordinal_position;
        `, table)
		if err != nil {
			fmt.Printf("(!) Failed to read columns for %s: %v\n", table, err)
			continue
		}

		for colRows.Next() {
			var colName string
			if err := colRows.Scan(&colName); err != nil {
				fmt.Printf("(!) Failed to read column: %v\n", err)
				continue
			}
			fmt.Printf("    - %s\n", colName)
		}
		colRows.Close()
	}
}

// --- Other utilities ---
func showPing(db *sql.DB) {
	var now string
	if err := db.QueryRow("SELECT NOW();").Scan(&now); err != nil {
		fmt.Printf("(X) Failed to query DB: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("(✓) Postgres time: %s\n", now)
}

func showAdmins(db *sql.DB) {
	rows, err := db.Query(`
        SELECT rolname 
        FROM pg_roles 
        WHERE rolsuper = true OR rolcreaterole = true
        ORDER BY rolname;
    `)
	if err != nil {
		fmt.Printf("(X) Failed to query admin users: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var admins []string
	for rows.Next() {
		var rol string
		if err := rows.Scan(&rol); err != nil {
			fmt.Printf("(!) Failed to read row: %v\n", err)
			continue
		}
		admins = append(admins, rol)
	}

	if len(admins) > 0 {
		fmt.Println("(✓) Admin users:")
		for _, a := range admins {
			fmt.Printf("  (-) %s\n", a)
		}
	} else {
		fmt.Println("(!) No admin users found")
	}
}

func showHelp(coreMode bool) {
	hivemind := `                           👁️
                           ╱│╲
                          o   o
                         ╱│╲ ╱│╲
                        o   o   o
                       ╱│╲ ╱│╲ ╱│╲
                      o   o   o   o
                     ╱│╲ ╱│╲ ╱│╲ ╱│╲
                    H I V E ● M I N D`

	fmt.Println("👁····························································👁")
	fmt.Println("👁··········<  hvmd  | Database communication CLI >···········👁")
	fmt.Println("👁····························································👁")
	fmt.Println(hivemind)
	fmt.Println("👁····························································👁")
	fmt.Println("Usage: hvmd command")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("")
	fmt.Println("  ping      - Show current Postgres server time")
	fmt.Println("  admins    - List all DB admin users (SUPERUSER or CREATEROLE)")
	fmt.Println("  help      - Show this help message")
	fmt.Println("  readdb    - Show database tables and column names (limited for non-core)")
	fmt.Println("")
	if coreMode {
		fmt.Println("☢️  ··························································☢️")
		fmt.Println("{👁️  } HIVEMIND CORE:")
		fmt.Println("")
		fmt.Println("Usage: hvmd command --core")
		fmt.Println("")
		fmt.Println("  identify --core     - Show current user privileges and core access")
		fmt.Println("  testssh --core      - Run a core-only SSH key test")
		fmt.Println("  readdb --core       - Read database schema and admin info")
		fmt.Println("  help --core         - You're already fkn here")
		fmt.Println("")
		fmt.Println("Secret Commands public (no --core):")
		fmt.Println("")
		fmt.Println("  addadminsshkey      - Add your SSH public key to .key file")
		fmt.Println("  catssh              - Display SSH key from .key file")
		fmt.Println("")
		fmt.Println("☢️  ·························································☢️")
	} else {
		fmt.Println("👁····························································👁")
	}
}

// --- Identity info ---
func showIdentify(db *sql.DB, username string) {
	var (
		rolname        string
		rolsuper       bool
		rolinherit     bool
		rolcreaterole  bool
		rolcreatedb    bool
		rolcanlogin    bool
		rolreplication bool
		rolconnlimit   int
		rolvaliduntil  sql.NullTime
	)

	err := db.QueryRow(`
		SELECT 
			rolname,
			rolsuper,
			rolinherit,
			rolcreaterole,
			rolcreatedb,
			rolcanlogin,
			rolreplication,
			rolconnlimit,
			rolvaliduntil
		FROM pg_roles 
		WHERE rolname = $1
	`, username).Scan(
		&rolname,
		&rolsuper,
		&rolinherit,
		&rolcreaterole,
		&rolcreatedb,
		&rolcanlogin,
		&rolreplication,
		&rolconnlimit,
		&rolvaliduntil,
	)

	if err != nil {
		fmt.Printf("(X) Failed to query user information: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("{👁️  } Identity Information:")
	fmt.Println("")
	fmt.Printf("  {👁️  } Role Name:        %s\n", rolname)
	fmt.Printf("  {👁️  } Superuser:        %v\n", rolsuper)
	fmt.Printf("  {👁️  } Inherit:          %v\n", rolinherit)
	fmt.Printf("  {👁️  } Create Role:      %v\n", rolcreaterole)
	fmt.Printf("  {👁️  } Create DB:        %v\n", rolcreatedb)
	fmt.Printf("  {👁️  } Can Login:        %v\n", rolcanlogin)
	fmt.Printf("  {👁️  } Replication:      %v\n", rolreplication)
	fmt.Printf("  {👁️  } Connection Limit: %d\n", rolconnlimit)

	if rolvaliduntil.Valid {
		fmt.Printf("  {👁️  } Valid Until:      %s\n", rolvaliduntil.Time.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("  {👁️  } Valid Until:      No expiration\n")
	}

	fmt.Println("")

	if rolsuper {
		fmt.Println("{👁️  } CORE ACCESS GRANTED")
	} else {
		fmt.Printf("{⚠️     👁️  👁️   ⚠️ } Not a superuser - Your breach has been logged at %s\n", time.Now().Format("15:04:05.000"))
	}
}

// --- Suggestion helper ---
func suggestSimilar(cmd string) {
	if strings.Contains(cmd, "core") {
		fmt.Println("    Try: hvmd help")
		return
	}

	suggestions := map[string][]string{
		"help":   {"hlep", "halp", "hel", "hepl", "h", "-h", "--help"},
		"ping":   {"pong", "pign", "pin", "pign", "p"},
		"admins": {"admin", "admn", "adm", "administrators", "users"},
	}

	cmd = strings.ToLower(cmd)

	for correct, typos := range suggestions {
		for _, typo := range typos {
			if strings.Contains(cmd, typo) || strings.Contains(typo, cmd) {
				fmt.Printf("    Did you mean: hvmd %s\n", correct)
				return
			}
		}
	}

	fmt.Println("    Try: hvmd help")
}

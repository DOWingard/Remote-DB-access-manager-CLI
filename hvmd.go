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
			// Skip it - it's the last arg and valid
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

	// Load .env
	if err := godotenv.Load(".env"); err != nil {
		fmt.Println("(!) No .env file found, relying on environment variables")
	}

	// Read environment variables
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	dbname := os.Getenv("POSTGRES_DB")
	host := os.Getenv("POSTGRES_HOST")
	port := os.Getenv("POSTGRES_PORT")
	isNull := os.Getenv("isNULL")

	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}

	if user == "" || password == "" || dbname == "" {
		fmt.Println("(X) Missing POSTGRES_USER, POSTGRES_PASSWORD, or POSTGRES_DB")
		os.Exit(1)
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, password, host, port, dbname)

	// Open connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Printf("(X) Failed to open DB: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Ping DB first for connectivity
	if err := db.Ping(); err != nil {
		fmt.Printf("(X) Failed to connect to database '%s': %v\n", dbname, err)
		os.Exit(1)
	}

	// Check SSH connection if isNULL is False
	if strings.ToLower(isNull) == "false" {
		checkSSHConnection()
	}

	// Check core access if --core was requested
	if coreRequested {
		if checkCoreAccess(db, user) {
			coreEnabled = true
		} else {
			fmt.Println("(!) Unknown command: --core")
			fmt.Println("    Try: hvmd help")
			os.Exit(1)
		}
	}

	switch cmd {
	case "ping":
		showPing(db)
	case "admins":
		showAdmins(db)
	case "help":
		showHelp(coreEnabled)
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
	default:
		// Check if this is a core command being used without --core
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

func checkSSHConnection() {
	// Load .key file
	keyEnv, err := godotenv.Read(".key")
	if err != nil {
		return
	}

	sshKey := keyEnv["SSH_KEY"]
	if sshKey == "" {
		fmt.Println("{⚠️  } No SSH_KEY found in .key file")
		return
	}

	// TODO: Implement actual SSH connection test
	// For now, just confirm key exists
	fmt.Println("{🔓 } SSH key loaded from .key")
}

func addAdminSSHKey() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Paste your SSH public key (press Enter when done):")
	sshKey, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("{⚠️  } Failed to read input: %v\n", err)
		os.Exit(1)
	}

	sshKey = strings.TrimSpace(sshKey)

	if sshKey == "" {
		fmt.Println("(X) No key provided")
		os.Exit(1)
	}

	// Write to .key file
	content := fmt.Sprintf("SSH_KEY=%s\n", sshKey)
	err = os.WriteFile(".key", []byte(content), 0600)
	if err != nil {
		fmt.Printf("{⚠️  } Failed to write .key file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("{🔑 } SSH key successfully written to .key")
}

func catSSH() {
	keyEnv, err := godotenv.Read(".key")
	if err != nil {
		fmt.Printf("{⚠️  } Failed to read .key file: %v\n", err)
		os.Exit(1)
	}

	sshKey := keyEnv["SSH_KEY"]
	if sshKey == "" {
		fmt.Println("{⚠️  } No SSH_KEY found in .key file")
		return
	}

	fmt.Println(sshKey)
}

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
	coreCommands := []string{"identify"}
	for _, c := range coreCommands {
		if c == cmd {
			return true
		}
	}
	return false
}

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
		fmt.Printf("{⚠️    👁️  👁️   ⚠️} Not a superuser - Your breach has been logged at %s\n", time.Now().Format("15:04:05.000"))
	}
}

func suggestSimilar(cmd string) {
	// Don't suggest anything for --core related commands
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

func handleCoreCommand(cmd string, db *sql.DB) {
	fmt.Printf("{👁️  } Executing: %s\n", strings.ToUpper(cmd))

	switch cmd {
	// TODOTODOTODOTODOTODO
	default:
		fmt.Println("{👁️  } Core command not yet implemented")
	}
}

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
	fmt.Println("")
	if coreMode {
		fmt.Println("☢️  ··························································☢️")
		fmt.Println("{👁️  } HIVEMIND CORE:")
		fmt.Println("")
		fmt.Println("Usage: hvmd command --core")
		fmt.Println("")
		fmt.Println("  identify --core     - Show current user privileges and core access")
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

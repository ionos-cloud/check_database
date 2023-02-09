package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"text/tabwriter"
	"text/template"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/pelletier/go-toml"
)

var (
	configPath = flag.String("config", "check_database.conf", "path to the main config file")
	warnLevel  = flag.Float64("warn", 0, "set the warning level")
	critLevel  = flag.Float64("error", 0, "set the error level")
)

type (
	Config struct {
		IncludeDir string               `toml:"include_dir"`
		Queries    map[string]Query     `toml:"query"`
		Databases  map[string]*Database `toml:"database"`
	}

	Query struct {
		Query      string      `toml:"query"`
		Parameters []Parameter `toml:"params"`
		Doc        string      `toml:"desc"`
		Message    string      `toml:"message"`
	}

	Parameter struct {
		Name  string `toml:"name"`
		Descr string `toml:"desc"`
		Value string `toml:"-"`
	}

	// Server contains the connection details to make a successful connection.
	Database struct {
		Type     string `toml:"type"`
		Username string `toml:"username"`
		Password string `toml:"password"`
		Hostname string `toml:"hostname"`
		Port     int    `toml:"port"`
		Database string `toml:"database"`
		SSL      string `toml:"ssl"`
	}
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s [options] command:\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  commands are:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "    list databases\n")
		fmt.Fprintf(flag.CommandLine.Output(), "    \tlist configured databases\n")
		fmt.Fprintf(flag.CommandLine.Output(), "    list queries\n")
		fmt.Fprintf(flag.CommandLine.Output(), "    \tlist available sql query\n")
		fmt.Fprintf(flag.CommandLine.Output(), "    run query on database [parameters]\n")
		fmt.Fprintf(flag.CommandLine.Output(), "    \trun a query against the database\n")
		fmt.Fprintf(flag.CommandLine.Output(), "\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  options are:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	config := newConfig(*configPath)

	switch flag.Arg(0) {
	case "":
		fallthrough
	case "help":
		flag.Usage()
		os.Exit(0)
	case "run":
		if len(flag.Args()) < 4 {
			flag.Usage()
			os.Exit(0)
		}
		runCommand(config, flag.Args()[1], flag.Args()[3], flag.Args()[4:])
	case "list":
		w := tabwriter.NewWriter(os.Stdout, 40, 2, 1, ' ', 0)
		switch flag.Arg(1) {
		case "databases":
			dbs := make([]string, len(config.Databases))
			i := 0
			for k, _ := range config.Databases {
				dbs[i] = k
				i++
			}
			sort.Strings(dbs)
			fmt.Fprintf(w, "type\tname\thostname\tdatabase\tuser\n")
			for _, name := range dbs {
				db := config.Databases[name]
				fmt.Fprintf(w, "%s\t%s\t%s:%d\t%s\t%s\n", db.Type, name, db.Hostname, db.Port, db.Database, db.Username)
			}
		case "queries":
			qs := make([]string, len(config.Queries))
			i := 0
			for k, _ := range config.Queries {
				qs[i] = k
				i++
			}
			sort.Strings(qs)
			fmt.Fprintf(w, "name\tparameters\tdescription\n")
			for _, name := range qs {
				q := config.Queries[name]
				params := make([]string, len(q.Parameters))
				for i, p := range q.Parameters {
					params[i] = p.Name
				}
				sort.Strings(params)
				fmt.Fprintf(w, "%s\t%s\t%s\n", name, strings.Join(params, ", "), q.Doc)
			}
		default:
			fmt.Fprintf(flag.CommandLine.Output(), "possible objects to list: databases, queries\n")
			os.Exit(0)
		}
		w.Flush()
	}
}

func newConfig(configPath string) *Config {
	config := Config{}
	if t, err := toml.LoadFile(configPath); err != nil {
		fmt.Printf("could not load main config file: %s\n", err)
		os.Exit(1)
	} else {
		if err := t.Unmarshal(&config); err != nil {
			fmt.Printf("could not parse config: %s\n", err)
			os.Exit(1)
		}
	}
	if len(config.Databases) == 0 {
		config.Databases = map[string]*Database{}
	}
	if len(config.Queries) == 0 {
		config.Queries = map[string]Query{}
	}
	if config.IncludeDir != "" {
		includeDir := config.IncludeDir
		if !path.IsAbs(includeDir) {
			includeDir = path.Join(path.Dir(configPath), includeDir)
		}
		entries, err := ioutil.ReadDir(includeDir)
		if err != nil {
			fmt.Printf("could not open directory: %s\n", err)
			os.Exit(1)
		}
		for _, entry := range entries {
			tmp, err := toml.LoadFile(path.Join(includeDir, entry.Name()))
			if err != nil {
				fmt.Printf("could not open file '%s': %s\n", entry.Name(), err)
				os.Exit(1)
			}
			tmpConf := Config{}
			if err := tmp.Unmarshal(&tmpConf); err != nil {
				fmt.Printf("could not parse file '%s': %s\n", entry.Name(), err)
				os.Exit(1)
			}
			for k, v := range tmpConf.Queries {
				if _, found := config.Queries[k]; found {
					fmt.Printf("query '%s' already exists in config, duplicate in file '%s'\n", k, entry.Name())
					os.Exit(1)
				}
				config.Queries[k] = v
			}
			for k, v := range tmpConf.Databases {
				if _, found := config.Databases[k]; found {
					fmt.Printf("server key '%s' already exists in config, duplicate in file '%s'\n", k, entry.Name())
					os.Exit(1)
				}
				config.Databases[k] = v
			}
		}
	}
	for _, db := range config.Databases {
		if db.Type == "" {
			db.Type = "postgres"
		}
		if db.Port == 0 {
			switch db.Type {
			case "mysql":
				db.Port = 3306
			case "postgres":
				db.Port = 5432
			default:
				fmt.Printf("unknown database type for '%s'", db.Type)
				os.Exit(1)
			}
			if db.Type == "mysql" {
				db.Port = 3306
			} else {
				db.Port = 5432
			}
		}
	}
	return &config
}

// Run the query on the specified host.
func runCommand(config *Config, queryName, dbName string, args []string) {
	query, found := config.Queries[queryName]
	if !found {
		fmt.Fprintf(os.Stderr, "could not find query '%s'\n", queryName)
		os.Exit(1)
	}
	db, found := config.Databases[dbName]
	if !found {
		fmt.Fprintf(os.Stderr, "could not find database '%s'\n", dbName)
		os.Exit(1)
	}
	tmpl := template.Must(template.New("default").Parse(query.Message))

	params := make([]interface{}, len(query.Parameters))
	flags := flag.NewFlagSet(fmt.Sprintf("check_database run %s on %s", queryName, dbName), flag.ExitOnError)
	for i, p := range query.Parameters {
		params[i] = new(string)
		if p.Descr != "" {
			flags.StringVar(params[i].(*string), p.Name, "", p.Descr)
		} else {
			flags.StringVar(params[i].(*string), p.Name, "", "parameter for "+p.Name)
		}
	}
	flags.Parse(args)

	for i, p := range params {
		if *p.(*string) == "" {
			fmt.Fprintf(os.Stderr, "parameter '%s' is empty", query.Parameters[i].Name)
			os.Exit(1)
		} else {
			query.Parameters[i].Value = *p.(*string)
		}
	}

	conn, err := sql.Open(db.Type, db.String())
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not open connection: %s\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	row := conn.QueryRow(query.Query, params...)
	result := 0.0
	if err := row.Scan(&result); err != nil && err != sql.ErrNoRows {
		fmt.Fprintf(os.Stderr, "could not receive result: %s\n", err)
		os.Exit(1)
	} else if err == sql.ErrNoRows {
		fmt.Fprintf(os.Stdout, "no value returned by query\n")
		os.Exit(1)
	}

	level := 0
	levelName := "okay"
	if *critLevel <= *warnLevel {
		if result <= *critLevel {
			level = 2
		} else if result <= *warnLevel {
			level = 1
		}
	} else {
		if result >= *critLevel {
			level = 2
		} else if result >= *warnLevel {
			level = 1
		}
	}
	switch level {
	case 2:
		levelName = "critical"
	case 1:
		levelName = "warning"
	}
	vars := map[string]interface{}{
		"DBName":    dbName,
		"DB":        db,
		"QueryName": queryName,
		"Query":     query,
		"Result":    result,
		"Level":     level,
		"LevelName": levelName,
		"Limit": map[string]float64{
			"Critical": *critLevel,
			"Warn":     *warnLevel,
		},
	}
	if err := tmpl.Execute(os.Stdout, vars); err != nil {
		fmt.Fprintf(os.Stderr, "could not render template of query '%s': %s\n", queryName, err)
		os.Exit(1)
	}
	fmt.Println()
	os.Exit(level)
}

func (db *Database) String() string {
	res := ""
	switch db.Type {
	case "postgres":
		if db.Username != "" {
			res += " user=" + db.Username
		}
		if db.Password != "" {
			res += " password=" + db.Password
		}
		if db.Hostname != "" {
			res += " host=" + db.Hostname
		}
		if db.Port != 0 {
			res += fmt.Sprintf(" port=%d", db.Port)
		}
		if db.Database != "" {
			res += " dbname=" + db.Database
		}
		if db.SSL != "" {
			res += " sslmode=" + db.SSL
		}
	case "mysql":
		if db.Username != "" {
			res = db.Username
			if db.Password != "" {
				res += ":" + db.Password
			}
			res += "@"
		}
		if db.Hostname != "" {
			res += "tcp(" + db.Hostname
			if db.Port != 0 {
				res += fmt.Sprintf(":%d", db.Port)
			}
			res += ")"
		}
		if db.Database != "" {
			res += "/" + db.Database
		}
	default:
		res = "unsupported database type " + db.Type
	}
	return res
}

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"time"

	corev2 "github.com/sensu/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
	"github.com/xo/dburl"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"github.com/sardinasystems/sensu-go-prometheus-metric-check/utils"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	DBURL             string
	Driver            string
	Host              string
	Port              int
	User              string
	Password          string
	Database          string
	Query             string
	QueryArgs         []string
	WarningStr        string
	CriticalStr       string
	WarningThreshold  utils.NagiosThreshold
	CriticalThreshold utils.NagiosThreshold
	Unquote           bool
	Debug             bool
}

var (
	allowedDrivers = []string{"mysql", "postgresql"}

	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "sensu-go-sql-select-count-check",
			Short:    "Query SQL DB and check for threashold",
			Keyspace: "sensu.io/plugins/sensu-go-sql-select-count-check/config",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "dburl",
			Env:       "SQL_URL",
			Argument:  "dburl",
			Shorthand: "",
			Default:   "",
			Usage:     "DB URL",
			Value:     &plugin.DBURL,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "driver",
			Env:       "SQL_DRIVER",
			Argument:  "driver",
			Shorthand: "",
			Default:   "mysql",
			Usage:     "DB Driver",
			Value:     &plugin.Driver,
			Allow:     allowedDrivers,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "host",
			Env:       "SQL_HOST",
			Argument:  "host",
			Shorthand: "H",
			Default:   "",
			Usage:     "DB Host",
			Value:     &plugin.Host,
		},

		&sensu.PluginConfigOption[int]{
			Path:      "port",
			Env:       "SQL_PORT",
			Argument:  "port",
			Shorthand: "P",
			Default:   0,
			Usage:     "DB Port",
			Value:     &plugin.Port,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "user",
			Env:       "SQL_USER",
			Argument:  "user",
			Shorthand: "u",
			Default:   "",
			Usage:     "DB User",
			Value:     &plugin.User,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "password",
			Env:       "SQL_PASSWORD",
			Argument:  "password",
			Shorthand: "p",
			Default:   "",
			Usage:     "DB Password",
			Value:     &plugin.Password,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "database",
			Env:       "SQL_DATABASE",
			Argument:  "database",
			Shorthand: "d",
			Default:   "",
			Usage:     "Database name",
			Value:     &plugin.Database,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "query",
			Env:       "SQL_QUERY",
			Argument:  "query",
			Shorthand: "q",
			Default:   "",
			Usage:     "Query",
			Value:     &plugin.Query,
		},
		&sensu.SlicePluginConfigOption[string]{
			Path:      "query_args",
			Env:       "SQL_QUERY_ARGS",
			Argument:  "query-args",
			Shorthand: "a",
			Default:   []string{},
			Usage:     "Optional query arguments passed to prepare statement",
			Value:     &plugin.QueryArgs,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "warning",
			Env:       "SQL_WARNING",
			Argument:  "warning",
			Shorthand: "w",
			Default:   "",
			Usage:     "Warning level",
			Value:     &plugin.WarningStr,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "critical",
			Env:       "SQL_CRITICAL",
			Argument:  "critical",
			Shorthand: "c",
			Default:   "",
			Usage:     "Critical level",
			Value:     &plugin.CriticalStr,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "unquote",
			Env:       "SQL_UNQUOTE",
			Argument:  "unquote",
			Shorthand: "j",
			Default:   false,
			Usage:     "Unquote string before parsing as float (useful for JSON fields)",
			Value:     &plugin.Unquote,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "debug",
			Env:       "DEBUG",
			Argument:  "debug",
			Shorthand: "",
			Default:   false,
			Usage:     "Enable debug log",
			Value:     &plugin.Debug,
		},
	}
)

func (s *Config) NewDB() (*sql.DB, error) {
	var err error
	var u *dburl.URL
	var dsn string

	if s.DBURL == "" {
		u = &dburl.URL{}
		u.Driver = s.Driver
		u.Host = s.Host
		if s.Port > 0 {
			u.Host += fmt.Sprintf(":%d", s.Port)
		}
		if s.User != "" {
			u.User = url.UserPassword(s.User, s.Password)
		}
		u.Path = s.Database

		switch s.Driver {
		case "mysql":
			dsn, _, err = dburl.GenMysql(u)
		case "postgresql":
			dsn, _, err = dburl.GenPostgres(u)
		default:
			return nil, fmt.Errorf("unsupported driver: %s", s.Driver)
		}
	} else {
		u, err = dburl.Parse(s.DBURL)
		if u != nil {
			s.Driver = u.Driver
			dsn = u.DSN
		}
	}
	if err != nil {
		return nil, err
	}

	slog.With("driver", s.Driver, "dsn", dsn).Debug("opening db...")
	return sql.Open(s.Driver, dsn)
}

func (s *Config) DoQuery(ctx context.Context, db *sql.DB) (*sql.Rows, error) {
	stmt, err := db.PrepareContext(ctx, s.Query)
	if err != nil {
		return nil, err
	}

	args := make([]any, len(s.QueryArgs))
	for i, a := range s.QueryArgs {
		args[i] = a
	}

	return stmt.QueryContext(ctx, args...)
}

func (s *Config) ExtractValueAndClose(rows *sql.Rows) (result float64, err error) {
	defer func() {
		err = errors.Join(err, rows.Close())
	}()

	columns, err := rows.Columns()
	if err != nil {
		return 0, err
	}

	clg := slog.With("columns", columns)
	if len(columns) == 0 {
		return 0, fmt.Errorf("No columns returned")
	} else if len(columns) > 1 {
		clg.Warn("Expected to have only one column. First column will be used")
	} else {
		clg.Debug("Got column")
	}

	for idx := 0; rows.Next(); idx++ {
		valuesAny := make([]any, len(columns))
		for i := range valuesAny {
			valuesAny[i] = new(string)
		}

		err = rows.Scan(valuesAny...)
		if err != nil {
			return 0, err
		}

		// NOTE: convert for slog
		valuesStr := make([]string, len(valuesAny))
		for i, v := range valuesAny {
			valuesStr[i] = *(v.(*string))
		}

		cvlg := clg.With("values", valuesStr, "row", idx+1)
		if idx == 0 {
			cvlg.Debug("First row")

			if s.Unquote {
				newStr, err := strconv.Unquote(valuesStr[0])
				if err != nil {
					return 0, err
				}
				valuesStr[0] = newStr
			}

			result, err = strconv.ParseFloat(valuesStr[0], 64)
			if err != nil {
				return 0, err
			}
		} else {
			cvlg.Warn("Query returned more than one row. Skipped")
		}
	}

	err = rows.Err()
	return
}

func main() {
	useStdin := false
	fi, err := os.Stdin.Stat()
	if err != nil {
		fmt.Printf("Error check stdin: %v\n", err)
		panic(err)
	}
	// Check the Mode bitmask for Named Pipe to indicate stdin is connected
	if fi.Mode()&os.ModeNamedPipe != 0 {
		useStdin = true
	}

	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, useStdin)
	check.Execute()
}

func checkArgs(event *corev2.Event) (int, error) {
	var err error

	plugin.WarningThreshold, err = utils.ParseThreshold(plugin.WarningStr)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("--warning error: %v", err)
	}

	plugin.CriticalThreshold, err = utils.ParseThreshold(plugin.CriticalStr)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("--critical error: %v", err)
	}

	return sensu.CheckStateOK, nil
}

func executeCheck(event *corev2.Event) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	programLevel := new(slog.LevelVar)
	if plugin.Debug {
		programLevel.Set(slog.LevelDebug)
	} else {
		programLevel.Set(slog.LevelInfo)
	}

	lgh := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: programLevel})
	slog.SetDefault(slog.New(lgh))

	db, err := plugin.NewDB()
	if err != nil {
		return sensu.CheckStateUnknown, fmt.Errorf("open db error: %w", err)
	}
	defer db.Close()

	rows, err := plugin.DoQuery(ctx, db)
	if err != nil {
		return sensu.CheckStateUnknown, fmt.Errorf("query error: %w", err)
	}

	value, err := plugin.ExtractValueAndClose(rows)
	if err != nil {
		return sensu.CheckStateUnknown, fmt.Errorf("read error: %w", err)
	}

	crit := plugin.CriticalThreshold.Check(value)
	warn := plugin.WarningThreshold.Check(value)
	state := sensu.CheckStateOK

	if crit {
		fmt.Printf("CRITICAL: result is %f which is out of %s", value, plugin.CriticalStr)
		state = sensu.CheckStateCritical
	} else if warn {
		fmt.Printf("WARNING: result is %f which is out of %s", value, plugin.WarningStr)
		state = sensu.CheckStateWarning
	} else {
		fmt.Printf("OK: result is %f", value)
	}

	fmt.Println()
	return state, nil
}

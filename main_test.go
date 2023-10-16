package main

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initLogger(t *testing.T) {
	t.Helper()

	buf := new(bytes.Buffer)
	lg := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{AddSource: true, Level: slog.LevelDebug}))
	slog.SetDefault(lg)

	t.Cleanup(func() {
		t.Log(buf.String())
	})
}

func TestNewDB(t *testing.T) {
	initLogger(t)

	testCases := []struct {
		name        string
		plugin      *Config
		expectedErr error
	}{
		{"mysql-url-ok", &Config{DBURL: "mysql://tester:testerpw@localhost:3306/test"}, nil},
		{"mysql-args-ok", &Config{Driver: "mysql", User: "tester", Password: "testerpw", Host: "localhost", Port: 3306, Database: "test"}, nil},
		{"mysql-url-no-pw", &Config{DBURL: "mysql://localhost:3306/test"}, nil},
		{"mysql-args-no-pw", &Config{Driver: "mysql", Host: "localhost", Port: 3306, Database: "test"}, nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			db, err := tc.plugin.NewDB()
			if tc.expectedErr == nil {
				assert.NoError(err)
				assert.NotNil(db)
			} else {
				assert.Nil(db)
				assert.ErrorIs(err, tc.expectedErr)
			}

			if db != nil {
				assert.NoError(db.Close())
			}
		})
	}
}

func TestDoQueryAndExtract(t *testing.T) {
	initLogger(t)
	assert := assert.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	config := &Config{DBURL: "mysql://tester:testerpw@localhost:3306/test"}
	db, err := config.NewDB()
	require.NoError(t, err)
	defer db.Close()

	exec := func(stmt string) {
		_, err := db.ExecContext(ctx, stmt)
		require.NoError(t, err)
	}

	exec(`DROP TABLE IF EXISTS test;`)
	exec(`CREATE TABLE test
(
  id integer AUTO_INCREMENT NOT NULL,
  foo varchar(255) NOT NULL,

  PRIMARY KEY (id)
);`)
	exec(`INSERT INTO test (foo) VALUES ("test1");`)

	// test single row

	config.Query = `SELECT COUNT(*) FROM test;`
	config.QueryArgs = []string{}

	rows, err := config.DoQuery(ctx, db)
	if assert.NoError(err) {
		result, err := config.ExtractValueAndClose(rows)
		assert.NoError(err)
		assert.Equal(1.0, result)
	}

	// test multiple rows

	exec(`INSERT INTO test (foo) VALUES ("test2");`)
	exec(`INSERT INTO test (foo) VALUES ("test3");`)

	rows, err = config.DoQuery(ctx, db)
	if assert.NoError(err) {
		result, err := config.ExtractValueAndClose(rows)
		assert.NoError(err)
		assert.Equal(1.0, result)
	}

	// test query args

	config.Query = `SELECT COUNT(*) FROM test WHERE foo = ?;`
	config.QueryArgs = []string{"test0"}

	rows, err = config.DoQuery(ctx, db)
	if assert.NoError(err) {
		result, err := config.ExtractValueAndClose(rows)
		assert.NoError(err)
		assert.Equal(1.0, result)
	}

	// test field

	config.Query = `SELECT id FROM test WHERE foo = ?;`
	config.QueryArgs = []string{"test2"}

	rows, err = config.DoQuery(ctx, db)
	if assert.NoError(err) {
		result, err := config.ExtractValueAndClose(rows)
		assert.NoError(err)
		assert.Equal(2.0, result)
	}

	// test multiple columns and rows

	config.Query = `SELECT * FROM test;`
	config.QueryArgs = []string{}

	rows, err = config.DoQuery(ctx, db)
	if assert.NoError(err) {
		result, err := config.ExtractValueAndClose(rows)
		assert.NoError(err)
		assert.Equal(1.0, result)
	}
}

func TestMain(t *testing.T) {
	initLogger(t)
}

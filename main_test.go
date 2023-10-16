package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewDB(t *testing.T) {
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
	assert := assert.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	config := &Config{DBURL: "mysql://tester:testerpw@localhost:3306/test"}
	db, err := config.NewDB()
	assert.NoError(err)
	defer db.Close()

	_, err = db.ExecContext(ctx, `
DROP TABLE test;

CREATE TABLE test
(
  id integer AUTO_INCREMENT NOT NULL,
  foo varchar(255) NOT NULL,

  PRIMARY KEY (id)
);

INSERT INTO test (foo) VALUES ("test1");
`)
	assert.NoError(err)

	// test single row

	config.Query = `SELECT COUNT(*) FROM test;`
	config.QueryArgs = []string{}

	rows, err := config.DoQuery(db)
	assert.NoError(err)

	result, err := config.ExtractValueAndClose(rows)
	assert.NoError(err)
	assert.Equal(1.0, result)

	// test multiple rows

	_, err = db.ExecContext(ctx, `
INSERT INTO test (foo) VALUES ("test2");
INSERT INTO test (foo) VALUES ("test3");
`)
	assert.NoError(err)

	rows, err = config.DoQuery(db)
	assert.NoError(err)

	result, err = config.ExtractValueAndClose(rows)
	assert.NoError(err)
	assert.Equal(1.0, result)

	// test query args

	config.Query = `SELECT COUNT(*) FROM test WHERE foo = ?;`
	config.QueryArgs = []string{"test0"}

	rows, err = config.DoQuery(db)
	assert.NoError(err)

	result, err = config.ExtractValueAndClose(rows)
	assert.NoError(err)
	assert.Equal(0.0, result)
}

func TestMain(t *testing.T) {
}

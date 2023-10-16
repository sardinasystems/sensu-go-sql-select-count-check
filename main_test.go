package main

import (
	"testing"

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

func TestMain(t *testing.T) {
}

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
		{"mysql-url", &Config{DBURL: "mysql://tester:testerpw@localhost:3306"}, nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			db, err := tc.plugin.NewDB()
			if tc.expectedErr != nil {
				assert.NoError(err)
				assert.NotNil(db)
				assert.NoError(db.Close())
			}

		})
	}

}

func TestMain(t *testing.T) {
}

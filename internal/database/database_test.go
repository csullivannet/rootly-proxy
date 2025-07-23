package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockDB struct {
	mock.Mock
}

func (m *mockDB) QueryRow(query string, args ...interface{}) Row {
	args = append([]interface{}{query}, args...)
	return m.Called(args...).Get(0).(Row)
}

type mockRow struct {
	mock.Mock
}

func (m *mockRow) Scan(dest ...interface{}) error {
	args := m.Called(dest...)
	return args.Error(0)
}

func TestFindByHostname(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		want     *StatusPage
		wantErr  bool
		setupDB  func() *mockDB
	}{
		{
			name:     "known hostname",
			hostname: "status.acme.com",
			want:     &StatusPage{ID: 1, Hostname: "status.acme.com", PageDataURL: "http://backend/acme"},
			wantErr:  false,
			setupDB: func() *mockDB {
				db := new(mockDB)
				row := new(mockRow)
				row.On("Scan", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
					*args.Get(0).(*int) = 1
					*args.Get(1).(*string) = "status.acme.com"
					*args.Get(2).(*string) = "http://backend/acme"
				}).Return(nil)
				db.On("QueryRow", mock.Anything, "status.acme.com").Return(row)
				return db
			},
		},
		{
			name:     "unknown hostname",
			hostname: "unknown.com",
			want:     nil,
			wantErr:  false,
			setupDB: func() *mockDB {
				db := new(mockDB)
				row := new(mockRow)
				row.On("Scan", mock.Anything, mock.Anything, mock.Anything).Return(ErrNoRows)
				db.On("QueryRow", mock.Anything, "unknown.com").Return(row)
				return db
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := tt.setupDB()
			repo := &PostgresRepository{db: db}

			got, err := repo.FindByHostname(tt.hostname)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

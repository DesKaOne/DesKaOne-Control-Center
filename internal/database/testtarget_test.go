package database

import "testing"

func TestValidateTestDatabaseURL(t *testing.T) {
	tests := []struct {
		name, url string
		ok        bool
	}{{"lokal", "postgres://u:p@127.0.0.1:5439/deskaone_control_test?sslmode=disable", true}, {"localhost", "postgresql://u:p@localhost/test_db", true}, {"produksi", "postgres://u:p@127.0.0.1/deskaone_control", false}, {"remote", "postgres://u:p@db.example.com/deskaone_test", false}, {"malformed", "bukan-url", false}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTestDatabaseURL(tc.url)
			if tc.ok && err != nil {
				t.Fatal(err)
			}
			if !tc.ok && err == nil {
				t.Fatal("target berbahaya diterima")
			}
		})
	}
}

package ovh

import "testing"

func Test_getCorrectSubdomain(t *testing.T) {
	tests := []struct {
		name  string
		d, bd string
		want  string
	}{
		{
			name: "root",
			d:    "foobar.com",
			bd:   "foobar.com",
			want: "_acme-challenge",
		},
		{
			name: "one level",
			d:    "www.foobar.com",
			bd:   "foobar.com",
			want: "_acme-challenge.www",
		},
		{
			name: "two levels",
			d:    "www.staging.foobar.com",
			bd:   "foobar.com",
			want: "_acme-challenge.www.staging",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getCorrectSubdomain(tt.d, tt.bd); got != tt.want {
				t.Errorf("getCorrectSubdomain() = %v, want %v", got, tt.want)
			}
		})
	}
}

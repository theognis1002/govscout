package sync

import (
	"testing"
	"time"
)

func TestParseFlexibleDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "MM/DD/YYYY format",
			input: "01/27/2026",
			want:  time.Date(2026, 1, 27, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "YYYY-MM-DD format",
			input: "2026-01-27",
			want:  time.Date(2026, 1, 27, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "MM/DD/YYYY leap day",
			input: "02/29/2024",
			want:  time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "YYYY-MM-DD leap day",
			input: "2024-02-29",
			want:  time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "invalid format",
			input:   "27-01-2026",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFlexibleDate(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

package main

import (
	"fmt"
	"testing"
)

func TestParseObjectSpec(t *testing.T) {
	zipSpec := fmt.Sprintf("secrets/bundle.zip%stls/server.crt", zipSpecSeparator)
	trimmedZipSpec := fmt.Sprintf("  secrets/bundle.zip %s tls/server.crt  ", zipSpecSeparator)
	missingObjectSpec := zipSpecSeparator + "tls/server.crt"
	missingEntrySpec := "secrets/bundle.zip" + zipSpecSeparator

	tests := []struct {
		name      string
		spec      string
		wantObj   string
		wantEntry string
		wantZip   bool
		wantErr   bool
	}{
		{
			name:      "plain object",
			spec:      "certs/root.crt",
			wantObj:   "certs/root.crt",
			wantEntry: "",
			wantZip:   false,
			wantErr:   false,
		},
		{
			name:      "zip object and entry",
			spec:      zipSpec,
			wantObj:   "secrets/bundle.zip",
			wantEntry: "tls/server.crt",
			wantZip:   true,
			wantErr:   false,
		},
		{
			name:      "zip object trims spaces",
			spec:      trimmedZipSpec,
			wantObj:   "secrets/bundle.zip",
			wantEntry: "tls/server.crt",
			wantZip:   true,
			wantErr:   false,
		},
		{
			name:    "missing object in zip spec",
			spec:    missingObjectSpec,
			wantErr: true,
		},
		{
			name:    "missing entry in zip spec",
			spec:    missingEntrySpec,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotObj, gotEntry, gotZip, err := parseObjectSpec(tt.spec)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for spec %q", tt.spec)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for spec %q: %v", tt.spec, err)
			}
			if gotObj != tt.wantObj {
				t.Fatalf("unexpected object for spec %q: got %q want %q", tt.spec, gotObj, tt.wantObj)
			}
			if gotEntry != tt.wantEntry {
				t.Fatalf("unexpected entry for spec %q: got %q want %q", tt.spec, gotEntry, tt.wantEntry)
			}
			if gotZip != tt.wantZip {
				t.Fatalf("unexpected zip flag for spec %q: got %v want %v", tt.spec, gotZip, tt.wantZip)
			}
		})
	}
}
